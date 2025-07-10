package transaction

import (
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/saradorri/gameintegrator/internal/domain"
)

// cancel cancels a transaction
func (uc *TransactionUseCase) cancel(userID int64, providerTxID string) (*domain.Transaction, error) {
	tx, txTransactionRepo, txUserRepo, err := uc.setupTransactionDB()
	if err != nil {
		return nil, err
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Validate user exists
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

	// Commit the transaction first
	if err := tx.Commit().Error; err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

	// Send transaction to wallet service
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
	if err != nil {
		// If wallet service fails, update transaction status to failed
		cancelTx.Status = domain.TransactionStatusFailed
		cancelTx.UpdatedAt = time.Now()
		if updateErr := uc.transactionRepo.Update(cancelTx); updateErr != nil {
			log.Printf("Failed to update cancel transaction status to failed: %v", updateErr)
		}

		// Check if it's a 4xx error (client error) and return wallet service error
		if uc.is4xxError(err) {
			var walletErr *domain.WalletServiceError
			if errors.As(err, &walletErr) {
				return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, err.Error(), walletErr.StatusCode, err)
			}
			return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, err.Error(), 400, err)
		}
		// For 5xx errors, return generic message
		return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to process cancel in wallet", 500, err)
	}

	// Parse balance from wallet response
	newBalance, err := strconv.ParseFloat(walletResp.Balance, 64)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid balance format from wallet", 400, nil)
	}

	// If wallet service succeeds, update transaction status to completed
	cancelTx.Status = domain.TransactionStatusCompleted
	cancelTx.OldBalance = newBalance - originalTx.Amount
	cancelTx.NewBalance = newBalance
	cancelTx.UpdatedAt = time.Now()
	if err := uc.transactionRepo.Update(cancelTx); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update cancel transaction status", 500, err)
	}

	return cancelTx, nil
}
