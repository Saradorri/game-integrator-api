package transaction

import (
	"time"

	"github.com/saradorri/gameintegrator/internal/domain"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// handleTransactionFailure handles failed transactions with proper error handling
func (uc *TransactionUseCase) handleTransactionFailure(tx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, err error, statusCode int) error {
	uc.logger.Error("Handling transaction failure", zap.Int64("transactionID", tx.ID), zap.String("statusCode", string(rune(statusCode))), zap.Error(err))
	if updateErr := uc.updateTransactionStatus(tx, domain.TransactionStatusFailed, txTransactionRepo, dbTx); updateErr != nil {
		return updateErr
	}

	if commitErr := uc.commitTransaction(dbTx); commitErr != nil {
		return commitErr
	}

	return domain.NewAppError(domain.ErrCodeWalletServiceError, err.Error(), statusCode, err)
}

// handle409Conflict handles 409 conflicts with balance verification
func (uc *TransactionUseCase) handle409Conflict(tx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, userID int64, providerTxID string, amount float64, balance float64) (*domain.Transaction, error) {
	uc.logger.Info("Handling 409 conflict for withdraw", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Float64("amount", amount))
	currentBalance, err := uc.getBalance(userID)
	if err != nil {
		uc.logger.Error("Failed to get balance during 409 conflict handling", zap.Int64("userID", userID), zap.Error(err))
		tx.Status = domain.TransactionStatusFailed
		tx.OldBalance = balance
		tx.NewBalance = balance - amount
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

	expectedBalance := balance - amount
	if currentBalance != expectedBalance {
		uc.logger.Warn("Balance mismatch during 409 conflict", zap.Float64("expectedBalance", expectedBalance), zap.Float64("currentBalance", currentBalance))
		// Balance changed - this might be a race condition
		// Create revert to be safe

		tx.Status = domain.TransactionStatusFailed
		tx.OldBalance = balance
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
	uc.logger.Info("409 conflict resolved successfully, updating transaction to pending", zap.Int64("transactionID", tx.ID))
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
	uc.logger.Info("Starting withdraw transaction", zap.Int64("userID", userID), zap.Float64("amount", amount), zap.String("providerTxID", providerTxID), zap.String("currency", currency))
	balance, err := uc.getBalance(userID)
	if err != nil {
		uc.logger.Error("Failed to get user balance for withdraw", zap.Int64("userID", userID), zap.Error(err))
		return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to get user balance", 500, err)
	}

	if err := uc.validateWithdrawInput(amount, providerTxID); err != nil {
		uc.logger.Warn("Withdraw input validation failed", zap.Int64("userID", userID), zap.Float64("amount", amount), zap.String("providerTxID", providerTxID), zap.Error(err))
		return nil, err
	}

	tx, txTransactionRepo, txUserRepo, err := uc.setupTransactionWithRecovery()
	if err != nil {
		return nil, err
	}

	if err = uc.checkProviderTxIDExists(txTransactionRepo, providerTxID); err != nil {
		uc.logger.Warn("Provider transaction ID already exists", zap.String("providerTxID", providerTxID), zap.Error(err))
		tx.Rollback()
		return nil, err
	}

	_, err = uc.getUserAndValidate(txUserRepo, userID, currency)
	if err != nil {
		uc.logger.Error("User validation failed for withdraw", zap.Int64("userID", userID), zap.String("currency", currency), zap.Error(err))
		tx.Rollback()
		return nil, err
	}

	transaction := uc.createTransactionRecord(userID, domain.TransactionTypeWithdraw, amount, currency, providerTxID, balance, balance-amount, nil)

	if err = txTransactionRepo.Create(transaction); err != nil {
		uc.logger.Error("Failed to create withdraw transaction in database", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Error(err))
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to create transaction", 500, err)
	}

	if err = tx.Commit().Error; err != nil {
		uc.logger.Error("Failed to commit withdraw transaction", zap.Int64("transactionID", transaction.ID), zap.Error(err))
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

	walletReq := uc.createWalletRequest(userID, currency, amount, transaction.ID, providerTxID)

	uc.logger.Info("Calling wallet service for withdraw", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID))
	walletResp, err := uc.walletSvc.Withdraw(walletReq)

	tx, txTransactionRepo, txUserRepo, txErr := uc.setupTransactionWithRecovery()
	if txErr != nil {
		return nil, txErr
	}

	if err != nil {
		uc.logger.Error("Wallet service withdraw failed", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Error(err))
		// Check for 409 conflicts (idempotency)
		if uc.is409Error(err) {
			uc.logger.Info("409 conflict detected for withdraw, handling idempotency", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID))
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
			uc.logger.Warn("4xx error from wallet service for withdraw", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Error(err))
			return nil, uc.handleTransactionFailure(transaction, txTransactionRepo, tx, err, 400)
		}

		// For 5xx errors (server errors)
		uc.logger.Error("5xx error from wallet service for withdraw", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Error(err))
		return nil, uc.handleTransactionFailure(transaction, txTransactionRepo, tx, err, 500)
	}

	uc.logger.Info("Wallet service withdraw successful", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID))
	newBalance, err := uc.parseWalletBalance(walletResp.Balance)
	if err != nil {
		return nil, uc.handleBalanceParseError(transaction, txTransactionRepo, tx, err)
	}

	transaction.OldBalance = newBalance + amount
	transaction.NewBalance = newBalance

	if updateErr := uc.updateTransactionStatus(transaction, domain.TransactionStatusPending, txTransactionRepo, tx); updateErr != nil {
		return nil, updateErr
	}

	if err = uc.commitTransaction(tx); err != nil {
		return nil, err
	}

	uc.logger.Info("Withdraw transaction completed successfully", zap.Int64("transactionID", transaction.ID), zap.Int64("userID", userID), zap.String("providerTxID", providerTxID))
	return transaction, nil
}
