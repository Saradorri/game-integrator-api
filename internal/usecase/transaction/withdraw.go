package transaction

import (
	"strconv"
	"time"

	"github.com/saradorri/gameintegrator/internal/domain"
	"gorm.io/gorm"
)

// handleTransactionFailure handles failed transactions with proper error handling
func (uc *TransactionUseCase) handleTransactionFailure(tx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, err error, statusCode int) error {
	tx.Status = domain.TransactionStatusFailed
	tx.UpdatedAt = time.Now()

	if updateErr := txTransactionRepo.Update(tx); updateErr != nil {
		dbTx.Rollback()
		return domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to update transaction status to failed", 500, updateErr)
	}

	if commitErr := dbTx.Commit().Error; commitErr != nil {
		return domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, commitErr)
	}

	return domain.NewAppError(domain.ErrCodeWalletServiceError, err.Error(), statusCode, err)
}

// handleBalanceParseError handles balance parsing errors
func (uc *TransactionUseCase) handleBalanceParseError(tx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, err error) error {
	tx.Status = domain.TransactionStatusFailed
	tx.UpdatedAt = time.Now()

	if updateErr := txTransactionRepo.Update(tx); updateErr != nil {
		dbTx.Rollback()
		return domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to update transaction status to failed", 500, updateErr)
	}

	if err = dbTx.Commit().Error; err != nil {
		dbTx.Rollback()
		return domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

	return domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid balance format from wallet", 400, err)
}

// handle409Conflict handles 409 conflicts with balance verification
func (uc *TransactionUseCase) handle409Conflict(tx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, userID int64, providerTxID string, amount float64, balance float64) (*domain.Transaction, error) {
	currentBalance, err := uc.getBalance(userID)
	if err != nil {
		tx.Status = domain.TransactionStatusFailed
		tx.OldBalance = balance + amount
		tx.NewBalance = currentBalance
		tx.UpdatedAt = time.Now()

		if updateErr := txTransactionRepo.Update(tx); updateErr != nil {
			dbTx.Rollback()
			return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to update transaction status", 500, updateErr)
		}

		if err = dbTx.Commit().Error; err != nil {
			dbTx.Rollback()
			return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
		}

		_, revertErr := uc.Revert(userID, providerTxID, amount, domain.TransactionTypeDeposit)
		if revertErr != nil {
			return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, "Transaction processed but balance verification failed. Please contact support.", 500, revertErr)
		}

		return nil, domain.NewAppError(domain.ErrCodeConcurrentModification, "Balance was modified by another transaction during processing", 409, err)
	}

	expectedBalance := balance + amount
	if currentBalance != expectedBalance {
		// Balance changed - this might be a race condition
		// Create revert to be safe

		tx.Status = domain.TransactionStatusFailed
		tx.OldBalance = balance + amount
		tx.NewBalance = currentBalance
		tx.UpdatedAt = time.Now()

		if updateErr := txTransactionRepo.Update(tx); updateErr != nil {
			dbTx.Rollback()
			return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to update transaction status", 500, updateErr)
		}

		if err = dbTx.Commit().Error; err != nil {
			dbTx.Rollback()
			return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
		}

		_, revertErr := uc.Revert(userID, providerTxID, amount, domain.TransactionTypeDeposit)
		if revertErr != nil {
			return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, "Transaction processed but balance verification failed. Please contact support.", 500, revertErr)
		}

		return nil, domain.NewAppError(domain.ErrCodeConcurrentModification, "Balance was modified by another transaction during processing", 409, err)
	}

	// 409 means success - update transaction to pending
	tx.Status = domain.TransactionStatusPending
	tx.OldBalance = currentBalance + amount
	tx.NewBalance = currentBalance
	tx.UpdatedAt = time.Now()

	if updateErr := txTransactionRepo.Update(tx); updateErr != nil {
		dbTx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to update transaction status", 500, updateErr)
	}

	if commitErr := dbTx.Commit().Error; commitErr != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, commitErr)
	}

	return tx, nil
}

// withdraw creates a withdrawal transaction
func (uc *TransactionUseCase) withdraw(userID int64, amount float64, providerTxID string, currency string) (*domain.Transaction, error) {
	balance, err := uc.getBalance(userID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to get user balance", 500, err)
	}

	if err := uc.validateWithdrawInput(amount, providerTxID); err != nil {
		return nil, err
	}

	tx, txTransactionRepo, txUserRepo, err := uc.setupTransactionWithRecovery()
	if err != nil {
		return nil, err
	}

	if err = uc.checkProviderTxIDExists(txTransactionRepo, providerTxID); err != nil {
		tx.Rollback()
		return nil, err
	}

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
		OldBalance:   balance,
		NewBalance:   balance - amount,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err = txTransactionRepo.Create(transaction); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to create transaction", 500, err)
	}

	if err = tx.Commit().Error; err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

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

	tx, txTransactionRepo, txUserRepo, txErr := uc.setupTransactionWithRecovery()
	if txErr != nil {
		return nil, txErr
	}

	if err != nil {
		// Check for 409 conflicts (idempotency)
		if uc.is409Error(err) {
			// 409 conflicts that occur after `5xx with 666` error and `CORS` error in wallet service
			// 409 means the transaction was ALREADY PROCESSED SUCCESSFULLY!

			transaction, err := uc.handle409Conflict(transaction, txTransactionRepo, tx, userID, providerTxID, amount, balance)
			if err != nil {
				return nil, err
			}

			return transaction, nil
		}

		// Check for 4xx errors (including insufficient balance)
		if uc.is4xxError(err) {
			return nil, uc.handleTransactionFailure(transaction, txTransactionRepo, tx, err, 400)
		}

		// For 5xx errors (server errors)
		return nil, uc.handleTransactionFailure(transaction, txTransactionRepo, tx, err, 500)
	}

	newBalance, err := strconv.ParseFloat(walletResp.Balance, 64)
	if err != nil {
		return nil, uc.handleBalanceParseError(transaction, txTransactionRepo, tx, err)
	}

	transaction.Status = domain.TransactionStatusPending
	transaction.OldBalance = newBalance + amount
	transaction.NewBalance = newBalance
	transaction.UpdatedAt = time.Now()

	if updateErr := txTransactionRepo.Update(transaction); updateErr != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to update transaction status", 500, updateErr)
	}

	if err = tx.Commit().Error; err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

	return transaction, nil
}
