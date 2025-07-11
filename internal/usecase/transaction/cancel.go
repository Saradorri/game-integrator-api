package transaction

import (
	"context"
	"time"

	"github.com/saradorri/gameintegrator/internal/domain"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// handleCancel409Conflict handles 409 conflicts for cancel transactions
func (uc *TransactionUseCase) handleCancel409Conflict(tx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, userID int64) (*domain.Transaction, error) {
	uc.logger.Info("Handling 409 conflict for cancel", zap.Int64("userID", userID), zap.Int64("transactionID", tx.ID))
	currentBalance, err := uc.getBalance(userID)
	if err != nil {
		uc.logger.Error("Failed to get balance during cancel 409 conflict", zap.Int64("userID", userID), zap.Error(err))
		return nil, err
	}

	tx.NewBalance = currentBalance
	tx.OldBalance = currentBalance - tx.Amount

	if updateErr := uc.updateTransactionStatus(tx, domain.TransactionStatusCompleted, txTransactionRepo, dbTx); updateErr != nil {
		return nil, updateErr
	}

	if commitErr := uc.commitTransaction(dbTx); commitErr != nil {
		return nil, commitErr
	}

	uc.logger.Info("Cancel 409 conflict resolved successfully", zap.Int64("transactionID", tx.ID), zap.Int64("userID", userID))
	return tx, nil
}

// handleCancelFailure handles failed cancel transactions with original transaction revert
func (uc *TransactionUseCase) handleCancelFailure(tx *domain.Transaction, originalTx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, err error, statusCode int) error {
	uc.logger.Error("Handling cancel failure", zap.Int64("cancelTxID", tx.ID), zap.Int64("originalTxID", originalTx.ID), zap.String("statusCode", string(rune(statusCode))), zap.Error(err))
	if updateErr := uc.updateTransactionStatus(tx, domain.TransactionStatusFailed, txTransactionRepo, dbTx); updateErr != nil {
		return updateErr
	}

	// Revert original transaction back to pending
	if updateErr := uc.updateTransactionStatus(originalTx, domain.TransactionStatusPending, txTransactionRepo, dbTx); updateErr != nil {
		return updateErr
	}

	if commitErr := uc.commitTransaction(dbTx); commitErr != nil {
		return commitErr
	}

	return domain.NewAppError(domain.ErrCodeWalletServiceError, err.Error(), statusCode, err)
}

// cancel cancels a transaction
func (uc *TransactionUseCase) cancel(userID int64, providerTxID string) (*domain.Transaction, error) {
	uc.logger.Info("Starting cancel transaction", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID))

	// Acquire user lock to prevent concurrent transactions
	ctx := context.Background()
	if err := uc.lockUser(ctx, userID); err != nil {
		return nil, err
	}
	defer uc.unlockUser(ctx, userID)

	tx, txTransactionRepo, txUserRepo, err := uc.setupTransactionWithRecovery()
	if err != nil {
		return nil, err
	}

	_, err = uc.getUserAndValidateWithoutCurrency(txUserRepo, userID)
	if err != nil {
		uc.logger.Error("User validation failed for cancel", zap.Int64("userID", userID), zap.Error(err))
		tx.Rollback()
		return nil, err
	}

	originalTx, err := txTransactionRepo.GetByProviderTxIDForUpdate(providerTxID)
	if err != nil {
		uc.logger.Error("Failed to get original transaction from database", zap.String("providerTxID", providerTxID), zap.Error(err))
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to get transaction", 500, err)
	}
	if originalTx == nil {
		uc.logger.Warn("Original transaction not found for cancel", zap.String("providerTxID", providerTxID))
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeTransactionNotFound, "Transaction not found", 404, nil)
	}

	if err := uc.validateTransactionOwnership(originalTx, userID); err != nil {
		uc.logger.Error("Transaction ownership validation failed for cancel", zap.Int64("userID", userID), zap.Int64("originalTxID", originalTx.ID), zap.Error(err))
		tx.Rollback()
		return nil, err
	}

	if err := uc.validateTransactionStatus(originalTx, domain.TransactionStatusPending, "cancelled"); err != nil {
		uc.logger.Error("Invalid transaction status for cancel", zap.Int64("originalTxID", originalTx.ID), zap.String("status", string(originalTx.Status)), zap.Error(err))
		tx.Rollback()
		return nil, err
	}

	cancelTx := uc.createTransactionRecord(userID, domain.TransactionTypeCancel, originalTx.Amount, originalTx.Currency, "cancel_"+providerTxID, 0, 0, &originalTx.ID)

	if err := txTransactionRepo.Create(cancelTx); err != nil {
		uc.logger.Error("Failed to create cancel transaction in database", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Error(err))
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to create cancel transaction", 500, err)
	}

	originalTx.Status = domain.TransactionStatusCancelled
	originalTx.UpdatedAt = time.Now()
	if err := txTransactionRepo.Update(originalTx); err != nil {
		uc.logger.Error("Failed to update original transaction status to cancelled", zap.Int64("originalTxID", originalTx.ID), zap.Error(err))
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update transaction", 500, err)
	}

	if err := tx.Commit().Error; err != nil {
		uc.logger.Error("Failed to commit cancel transaction", zap.Int64("cancelTxID", cancelTx.ID), zap.Error(err))
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

	walletReq := uc.createWalletRequest(userID, originalTx.Currency, originalTx.Amount, originalTx.ID, cancelTx.ProviderTxID)

	uc.logger.Info("Calling wallet service for cancel", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID))
	walletResp, err := uc.walletSvc.Deposit(walletReq)

	tx, txTransactionRepo, txUserRepo, txErr := uc.setupTransactionWithRecovery()
	if txErr != nil {
		return nil, txErr
	}

	if err != nil {
		uc.logger.Error("Wallet service cancel failed", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Error(err))
		// Check for 409 conflicts (idempotency) - mark as completed
		if uc.is409Error(err) {
			uc.logger.Info("409 conflict detected for cancel, handling idempotency", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID))
			cancelTx, err := uc.handleCancel409Conflict(cancelTx, txTransactionRepo, tx, userID)
			if err != nil {
				return nil, err
			}
			return cancelTx, nil
		}

		// Check for 4xx errors (client errors) first
		if uc.is4xxError(err) {
			uc.logger.Warn("4xx error from wallet service for cancel", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Error(err))
			return nil, uc.handleCancelFailure(cancelTx, originalTx, txTransactionRepo, tx, err, 400)
		}

		// For 5xx errors (server errors)
		uc.logger.Error("5xx error from wallet service for cancel", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Error(err))
		return nil, uc.handleCancelFailure(cancelTx, originalTx, txTransactionRepo, tx, err, 500)
	}

	uc.logger.Info("Wallet service cancel successful", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID))
	newBalance, err := uc.parseWalletBalance(walletResp.Balance)
	if err != nil {
		return nil, uc.handleBalanceParseError(cancelTx, txTransactionRepo, tx, err)
	}

	cancelTx.OldBalance = newBalance - originalTx.Amount
	cancelTx.NewBalance = newBalance

	if updateErr := uc.updateTransactionStatus(cancelTx, domain.TransactionStatusCompleted, txTransactionRepo, tx); updateErr != nil {
		return nil, updateErr
	}

	if commitErr := uc.commitTransaction(tx); commitErr != nil {
		return nil, commitErr
	}

	uc.logger.Info("Cancel transaction completed successfully", zap.Int64("cancelTxID", cancelTx.ID), zap.Int64("userID", userID), zap.String("providerTxID", providerTxID))
	return cancelTx, nil
}
