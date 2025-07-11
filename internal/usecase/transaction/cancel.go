package transaction

import (
	"strconv"
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

	tx.Status = domain.TransactionStatusCompleted
	tx.NewBalance = currentBalance
	tx.OldBalance = currentBalance - tx.Amount
	tx.UpdatedAt = time.Now()

	if updateErr := txTransactionRepo.Update(tx); updateErr != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update cancel transaction status", 500, updateErr)
	}

	if commitErr := dbTx.Commit().Error; commitErr != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

	return tx, nil
}

// handleCancelFailure handles failed cancel transactions with original transaction revert
func (uc *TransactionUseCase) handleCancelFailure(tx *domain.Transaction, originalTx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, err error, statusCode int) error {
	tx.Status = domain.TransactionStatusFailed
	tx.UpdatedAt = time.Now()

	if updateErr := txTransactionRepo.Update(tx); updateErr != nil {
		dbTx.Rollback()
		return domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to update transaction status to failed", 500, updateErr)
	}

	// Revert original transaction back to pending
	originalTx.Status = domain.TransactionStatusPending
	originalTx.UpdatedAt = time.Now()
	if updateErr := txTransactionRepo.Update(originalTx); updateErr != nil {
		dbTx.Rollback()
		return domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to update original transaction status", 500, updateErr)
	}

	if commitErr := dbTx.Commit().Error; commitErr != nil {
		return domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, commitErr)
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

	cancelTx := &domain.Transaction{
		UserID:                userID,
		Type:                  domain.TransactionTypeCancel,
		Status:                domain.TransactionStatusSyncing,
		Amount:                originalTx.Amount,
		Currency:              originalTx.Currency,
		ProviderTxID:          "cancel_" + providerTxID,
		ProviderWithdrawnTxID: &originalTx.ID,
		OldBalance:            0,
		NewBalance:            0,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

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

	walletReq := domain.WalletTransactionRequest{
		UserID:   userID,
		Currency: originalTx.Currency,
		Transactions: []domain.WalletRequestTransaction{
			{
				Amount:    originalTx.Amount,
				BetID:     originalTx.ID,
				Reference: cancelTx.ProviderTxID,
			},
		},
	}

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

	newBalance, err := strconv.ParseFloat(walletResp.Balance, 64)
	if err != nil {
		return nil, uc.handleBalanceParseError(cancelTx, txTransactionRepo, tx, err)
	}

	cancelTx.Status = domain.TransactionStatusCompleted
	cancelTx.OldBalance = newBalance - originalTx.Amount
	cancelTx.NewBalance = newBalance
	cancelTx.UpdatedAt = time.Now()

	if updateErr := txTransactionRepo.Update(cancelTx); updateErr != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update cancel transaction status", 500, updateErr)
	}

	if commitErr := tx.Commit().Error; commitErr != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, commitErr)
	}

	return cancelTx, nil
}
