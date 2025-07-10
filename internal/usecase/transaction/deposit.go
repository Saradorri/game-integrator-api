package transaction

import (
	"log"
	"strconv"
	"time"

	"github.com/saradorri/gameintegrator/internal/domain"
)

// deposit creates a deposit transaction
func (uc *TransactionUseCase) deposit(userID int64, amount float64, providerTxID string, providerWithdrawnTxID int64, currency string) (*domain.Transaction, error) {
	if err := uc.validateDepositInput(amount, providerTxID); err != nil {
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

	// Validate user exists and currency matches (no lock needed - user is not modified)
	_, err = uc.getUserAndValidate(txUserRepo, userID, currency)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Check provider transaction ID exists with lock to prevent race conditions
	if err := uc.checkProviderTxIDExistsForUpdate(txTransactionRepo, providerTxID); err != nil {
		tx.Rollback()
		return nil, err
	}

	withdrawnTx, err := txTransactionRepo.GetByID(providerWithdrawnTxID)
	if err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to get withdrawn transaction from DB", 500, err)
	}
	if withdrawnTx == nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeTransactionNotFound, "Withdrawn transaction not found", 404, nil)
	}

	if err := uc.validateTransactionOwnership(withdrawnTx, userID); err != nil {
		tx.Rollback()
		return nil, err
	}

	if withdrawnTx.Status == domain.TransactionStatusCompleted {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeTransactionAlreadyDeposited, "Withdrawn transaction already deposited", 404, nil)
	}

	if err := uc.validateTransactionStatus(withdrawnTx, domain.TransactionStatusPending, "deposited"); err != nil {
		tx.Rollback()
		return nil, err
	}

	transaction := &domain.Transaction{
		UserID:                userID,
		Type:                  domain.TransactionTypeDeposit,
		Status:                domain.TransactionStatusSyncing,
		Amount:                amount,
		Currency:              currency,
		ProviderTxID:          providerTxID,
		ProviderWithdrawnTxID: &providerWithdrawnTxID,
		OldBalance:            0,
		NewBalance:            0,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	if err := txTransactionRepo.Create(transaction); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to create DB transaction", 500, err)
	}

	withdrawnTx.Status = domain.TransactionStatusCompleted
	withdrawnTx.UpdatedAt = time.Now()
	if err := txTransactionRepo.Update(withdrawnTx); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update withdrawn transaction", 500, err)
	}

	// Commit the transaction quickly to release locks
	if err := tx.Commit().Error; err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

	// Process wallet service
	walletReq := domain.WalletTransactionRequest{
		UserID:   userID,
		Currency: currency,
		Transactions: []domain.WalletRequestTransaction{
			{
				Amount:    amount,
				BetID:     withdrawnTx.ID,
				Reference: providerTxID,
			},
		},
	}

	walletResp, err := uc.walletSvc.Deposit(walletReq)
	if err != nil {
		transaction.Status = domain.TransactionStatusFailed
		transaction.UpdatedAt = time.Now()
		if updateErr := uc.transactionRepo.Update(transaction); updateErr != nil {
			log.Printf("Failed to update transaction status to failed: %v", updateErr)
		}
		log.Printf("Deposit wallet service failed for transaction %d: %v", transaction.ID, err)

		// Check if error is 409
		if uc.is409Error(err) {
			// Create a revert transaction for 409 conflicts
			return uc.Revert(userID, providerTxID, amount, domain.TransactionTypeWithdraw)
		}

		return nil, err
	}

	transaction.Status = domain.TransactionStatusCompleted

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

	transaction.OldBalance = newBalance - amount
	transaction.NewBalance = newBalance
	transaction.UpdatedAt = time.Now()
	if err := uc.transactionRepo.Update(transaction); err != nil {
		log.Printf("Failed to update transaction status for %d: %v", transaction.ID, err)
		// TODO: Implement retry mechanism for failed updates
		return nil, err
	}

	return transaction, nil
}
