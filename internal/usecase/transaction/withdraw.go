package transaction

import (
	"log"
	"strconv"
	"time"

	"github.com/saradorri/gameintegrator/internal/domain"
)

// withdraw creates a withdrawal transaction
func (uc *TransactionUseCase) withdraw(userID int64, amount float64, providerTxID string, currency string) (*domain.Transaction, error) {
	if err := uc.validateWithdrawInput(amount, providerTxID); err != nil {
		return nil, err
	}

	tx, txTransactionRepo, txUserRepo, err := uc.setupTransactionDB()
	if err != nil {
		return nil, err
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Check provider transaction ID  (no lock needed - transaction is not modified)
	if err := uc.checkProviderTxIDExists(txTransactionRepo, providerTxID); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Validate user exists and currency matches (no lock needed - user is not modified)
	_, err = uc.getUserAndValidate(txUserRepo, userID, currency)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	transaction := &domain.Transaction{
		UserID:       userID,
		Type:         domain.TransactionTypeWithdraw,
		Status:       domain.TransactionStatusSyncing,
		Amount:       amount,
		Currency:     currency,
		ProviderTxID: providerTxID,
		OldBalance:   0, // Will be updated after wallet service call
		NewBalance:   0, // Will be updated after wallet service call
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := txTransactionRepo.Create(transaction); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to create transaction", 500, err)
	}

	// Commit the transaction quickly to release locks
	if err := tx.Commit().Error; err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

	// Send transaction to wallet service
	walletReq := domain.WalletTransactionRequest{
		UserID:   userID,
		Currency: currency,
		Transactions: []domain.WalletRequestTransaction{
			{
				Amount:    amount,
				BetID:     transaction.ID,
				Reference: providerTxID,
			},
		},
	}

	walletResp, err := uc.walletSvc.Withdraw(walletReq)
	if err != nil {
		// If wallet service fails, update transaction status to failed
		transaction.Status = domain.TransactionStatusFailed
		transaction.UpdatedAt = time.Now()
		if updateErr := uc.transactionRepo.Update(transaction); updateErr != nil {
			log.Printf("Failed to update transaction status to failed: %v", updateErr)
		}

		log.Printf("Withdraw wallet service failed for transaction %d: %v", transaction.ID, err)

		// check if error is 409
		if uc.is409Error(err) {
			// Create a revert transaction for 409 conflicts that occur after 5xx error in wallet service
			_, err := uc.Revert(userID, providerTxID, amount, domain.TransactionTypeDeposit)
			if err != nil {
				return nil, err
			}

			return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to commit transaction; If transaction is not reverted after 1 hour, please contact support", 400, err)
		}

		return nil, err
	}

	// If wallet service succeeds, update transaction status to pending
	transaction.Status = domain.TransactionStatusPending

	// Parse balance from wallet response
	newBalance, err := strconv.ParseFloat(walletResp.Balance, 64)
	if err != nil {
		log.Printf("Invalid balance format from wallet for transaction %d: %v", transaction.ID, err)
		transaction.Status = domain.TransactionStatusFailed
		transaction.UpdatedAt = time.Now()
		if updateErr := uc.transactionRepo.Update(transaction); updateErr != nil {
			log.Printf("Failed to update transaction status to failed: %v", updateErr)
		}

		return nil, err
	}

	transaction.OldBalance = newBalance + amount
	transaction.NewBalance = newBalance
	transaction.UpdatedAt = time.Now()
	if err := uc.transactionRepo.Update(transaction); err != nil {
		log.Printf("Failed to update transaction status for %d: %v", transaction.ID, err)
		// TODO: Implement retry mechanism for failed updates
		return nil, err
	}

	return transaction, nil
}
