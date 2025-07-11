package transaction

import (
	"github.com/saradorri/gameintegrator/internal/domain"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// handleRevert409Conflict handles 409 conflicts for revert transactions
func (uc *TransactionUseCase) handleRevert409Conflict(tx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, userID int64) (*domain.Transaction, error) {
	uc.logger.Info("Handling 409 conflict for revert", zap.Int64("userID", userID), zap.Int64("transactionID", tx.ID))
	currentBalance, err := uc.getBalance(userID)
	if err != nil {
		uc.logger.Error("Failed to get balance during revert 409 conflict", zap.Int64("userID", userID), zap.Error(err))
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

	uc.logger.Info("Revert 409 conflict resolved successfully", zap.Int64("transactionID", tx.ID), zap.Int64("userID", userID))
	return tx, nil
}

// handleRevertFailure handles failed revert transactions
func (uc *TransactionUseCase) handleRevertFailure(tx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, err error, statusCode int) error {
	uc.logger.Error("Handling revert failure", zap.Int64("transactionID", tx.ID), zap.String("statusCode", string(rune(statusCode))), zap.Error(err))
	if updateErr := uc.updateTransactionStatus(tx, domain.TransactionStatusFailed, txTransactionRepo, dbTx); updateErr != nil {
		return updateErr
	}

	if commitErr := uc.commitTransaction(dbTx); commitErr != nil {
		return commitErr
	}

	return domain.NewAppError(domain.ErrCodeWalletServiceError, err.Error(), statusCode, err)
}

// Revert creates a revert transaction
func (uc *TransactionUseCase) Revert(userID int64, providerTxID string, amount float64, txType domain.TransactionType) (*domain.Transaction, error) {
	uc.logger.Info("Starting revert transaction", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Float64("amount", amount), zap.String("type", string(txType)))
	tx, txTransactionRepo, txUserRepo, err := uc.setupTransactionWithRecovery()
	if err != nil {
		return nil, err
	}

	_, err = uc.getUserAndValidateWithoutCurrency(txUserRepo, userID)
	if err != nil {
		uc.logger.Error("User validation failed for revert", zap.Int64("userID", userID), zap.Error(err))
		tx.Rollback()
		return nil, err
	}

	// Check if revert transaction already exists
	existingRevert, err := txTransactionRepo.GetByProviderTxID("revert_" + providerTxID)
	if err != nil {
		uc.logger.Error("Failed to check existing revert transaction", zap.String("providerTxID", providerTxID), zap.Error(err))
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to check existing revert", 500, err)
	}
	if existingRevert != nil {
		uc.logger.Warn("Revert transaction already exists", zap.String("providerTxID", providerTxID), zap.Int64("existingRevertID", existingRevert.ID))
		tx.Rollback()
		return existingRevert, nil
	}

	originTx, err := txTransactionRepo.GetByProviderTxIDForUpdate(providerTxID)
	if err != nil {
		uc.logger.Error("Failed to check existing revert transaction", zap.String("providerTxID", providerTxID), zap.Error(err))
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to check existing revert", 500, err)
	}
	if originTx.Status != domain.TransactionStatusFailed {
		uc.logger.Warn("The transaction cannot be revert", zap.String("providerTxID", providerTxID), zap.Int64("WithdrawnID", originTx.ID))
		tx.Rollback()
		return originTx, nil
	}

	revertTx := uc.createTransactionRecord(userID, domain.TransactionTypeRevert, amount, originTx.Currency, "revert_"+providerTxID, 0, 0, &originTx.ID)

	if err := txTransactionRepo.Create(revertTx); err != nil {
		uc.logger.Error("Failed to create revert transaction in database", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Error(err))
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to create revert transaction", 500, err)
	}

	if err := tx.Commit().Error; err != nil {
		uc.logger.Error("Failed to commit revert transaction", zap.Int64("revertTxID", revertTx.ID), zap.Error(err))
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

	walletReq := uc.createWalletRequest(userID, originTx.Currency, amount, revertTx.ID, revertTx.ProviderTxID)

	uc.logger.Info("Calling wallet service for revert", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID))
	walletResp, err := uc.walletSvc.Deposit(walletReq)

	tx, txTransactionRepo, txUserRepo, txErr := uc.setupTransactionWithRecovery()
	if txErr != nil {
		return nil, txErr
	}

	if err != nil {
		uc.logger.Error("Wallet service revert failed", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Error(err))
		// Check for 409 conflicts (idempotency) - mark as completed
		if uc.is409Error(err) {
			uc.logger.Info("409 conflict detected for revert, handling idempotency", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID))
			revertTx, err := uc.handleRevert409Conflict(revertTx, txTransactionRepo, tx, userID)
			if err != nil {
				return nil, err
			}
			return revertTx, nil
		}

		// Check for 4xx errors (client errors) first
		if uc.is4xxError(err) {
			uc.logger.Warn("4xx error from wallet service for revert", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Error(err))
			return nil, uc.handleRevertFailure(revertTx, txTransactionRepo, tx, err, 400)
		}

		// For 5xx errors (server errors)
		uc.logger.Error("5xx error from wallet service for revert", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Error(err))
		return nil, uc.handleRevertFailure(revertTx, txTransactionRepo, tx, err, 500)
	}

	uc.logger.Info("Wallet service revert successful", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID))
	newBalance, err := uc.parseWalletBalance(walletResp.Balance)
	if err != nil {
		return nil, uc.handleBalanceParseError(revertTx, txTransactionRepo, tx, err)
	}

	revertTx.OldBalance = newBalance - amount
	revertTx.NewBalance = newBalance

	if updateErr := uc.updateTransactionStatus(revertTx, domain.TransactionStatusCompleted, txTransactionRepo, tx); updateErr != nil {
		return nil, updateErr
	}

	if commitErr := uc.commitTransaction(tx); commitErr != nil {
		return nil, commitErr
	}

	uc.logger.Info("Revert transaction completed successfully", zap.Int64("revertTxID", revertTx.ID), zap.Int64("userID", userID), zap.String("providerTxID", providerTxID))
	return revertTx, nil
}
