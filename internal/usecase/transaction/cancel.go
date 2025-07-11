package transaction

import (
	"time"

	"github.com/saradorri/gameintegrator/internal/domain"
	"gorm.io/gorm"
)

// handleCancel409Conflict handles 409 conflicts for cancel transactions
func (uc *TransactionUseCase) handleCancel409Conflict(tx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, userID int64) (*domain.Transaction, error) {
	currentBalance, err := uc.getBalance(userID)
	if err != nil {
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

	return tx, nil
}

// handleCancelFailure handles failed cancel transactions with original transaction revert
func (uc *TransactionUseCase) handleCancelFailure(tx *domain.Transaction, originalTx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, err error, statusCode int) error {
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
	tx, txTransactionRepo, txUserRepo, err := uc.setupTransactionWithRecovery()
	if err != nil {
		return nil, err
	}

	_, err = uc.getUserAndValidateWithoutCurrency(txUserRepo, userID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	originalTx, err := txTransactionRepo.GetByProviderTxIDForUpdate(providerTxID)
	if err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to get transaction", 500, err)
	}
	if originalTx == nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeTransactionNotFound, "Transaction not found", 404, nil)
	}

	if err := uc.validateTransactionOwnership(originalTx, userID); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := uc.validateTransactionStatus(originalTx, domain.TransactionStatusPending, "cancelled"); err != nil {
		tx.Rollback()
		return nil, err
	}

	cancelTx := uc.createTransactionRecord(userID, domain.TransactionTypeCancel, originalTx.Amount, originalTx.Currency, "cancel_"+providerTxID, 0, 0, &originalTx.ID)

	if err := txTransactionRepo.Create(cancelTx); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to create cancel transaction", 500, err)
	}

	originalTx.Status = domain.TransactionStatusCancelled
	originalTx.UpdatedAt = time.Now()
	if err := txTransactionRepo.Update(originalTx); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update transaction", 500, err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

	walletReq := uc.createWalletRequest(userID, originalTx.Currency, originalTx.Amount, originalTx.ID, cancelTx.ProviderTxID)

	walletResp, err := uc.walletSvc.Deposit(walletReq)

	tx, txTransactionRepo, txUserRepo, txErr := uc.setupTransactionWithRecovery()
	if txErr != nil {
		return nil, txErr
	}

	if err != nil {
		// Check for 409 conflicts (idempotency) - mark as completed
		if uc.is409Error(err) {
			cancelTx, err := uc.handleCancel409Conflict(cancelTx, txTransactionRepo, tx, userID)
			if err != nil {
				return nil, err
			}
			return cancelTx, nil
		}

		// Check for 4xx errors (client errors) first
		if uc.is4xxError(err) {
			return nil, uc.handleCancelFailure(cancelTx, originalTx, txTransactionRepo, tx, err, 400)
		}

		// For 5xx errors (server errors)
		return nil, uc.handleCancelFailure(cancelTx, originalTx, txTransactionRepo, tx, err, 500)
	}

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

	return cancelTx, nil
}
