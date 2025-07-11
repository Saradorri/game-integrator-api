package transaction

import (
	"context"

	"github.com/saradorri/gameintegrator/internal/domain"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// handleDeposit409Conflict handles 409 conflicts for deposit transactions
func (uc *TransactionUseCase) handleDeposit409Conflict(tx *domain.Transaction, withdrawnTx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, userID int64) (*domain.Transaction, error) {
	uc.logger.Info("Handling 409 conflict for deposit", zap.Int64("userID", userID), zap.Int64("transactionID", tx.ID), zap.Int64("withdrawnTxID", withdrawnTx.ID))
	currentBalance, walletErr := uc.getBalance(userID)
	if walletErr != nil {
		uc.logger.Error("Failed to get balance during deposit 409 conflict", zap.Int64("userID", userID), zap.Error(walletErr))
		return nil, walletErr
	}

	tx.NewBalance = currentBalance
	tx.OldBalance = currentBalance - tx.Amount

	if updateErr := uc.updateTransactionStatus(tx, domain.TransactionStatusCompleted, txTransactionRepo, dbTx); updateErr != nil {
		return nil, updateErr
	}

	if updateErr := uc.updateTransactionStatus(withdrawnTx, domain.TransactionStatusCompleted, txTransactionRepo, dbTx); updateErr != nil {
		return nil, updateErr
	}

	if commitErr := uc.commitTransaction(dbTx); commitErr != nil {
		return nil, commitErr
	}

	uc.logger.Info("Deposit 409 conflict resolved successfully", zap.Int64("transactionID", tx.ID), zap.Int64("userID", userID))
	return tx, nil
}

// handleDepositFailure handles failed deposit transactions with withdrawn transaction revert
func (uc *TransactionUseCase) handleDepositFailure(tx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, err error, statusCode int) error {
	uc.logger.Error("Handling deposit failure", zap.Int64("transactionID", tx.ID), zap.String("statusCode", string(rune(statusCode))), zap.Error(err))
	if updateErr := uc.updateTransactionStatus(tx, domain.TransactionStatusFailed, txTransactionRepo, dbTx); updateErr != nil {
		return updateErr
	}

	if commitErr := uc.commitTransaction(dbTx); commitErr != nil {
		return commitErr
	}

	return domain.NewAppError(domain.ErrCodeWalletServiceError, err.Error(), statusCode, err)
}

// deposit creates a deposit transaction
func (uc *TransactionUseCase) deposit(userID int64, amount float64, providerTxID string, providerWithdrawnTxID int64, currency string) (*domain.Transaction, error) {
	uc.logger.Info("Starting deposit transaction", zap.Int64("userID", userID), zap.Float64("amount", amount), zap.String("providerTxID", providerTxID), zap.Int64("providerWithdrawnTxID", providerWithdrawnTxID), zap.String("currency", currency))

	// Acquire user lock to prevent concurrent transactions
	ctx := context.Background()
	if err := uc.lockUser(ctx, userID); err != nil {
		return nil, err
	}
	defer uc.unlockUser(ctx, userID)

	if err := uc.validateDepositInput(amount, providerTxID); err != nil {
		uc.logger.Warn("Deposit input validation failed", zap.Int64("userID", userID), zap.Float64("amount", amount), zap.String("providerTxID", providerTxID), zap.Error(err))
		return nil, err
	}

	tx, txTransactionRepo, txUserRepo, err := uc.setupTransactionWithRecovery()
	if err != nil {
		return nil, err
	}

	_, err = uc.getUserAndValidate(txUserRepo, userID, currency)
	if err != nil {
		uc.logger.Error("User validation failed for deposit", zap.Int64("userID", userID), zap.String("currency", currency), zap.Error(err))
		tx.Rollback()
		return nil, err
	}

	if err := uc.checkProviderTxIDExistsForUpdate(txTransactionRepo, providerTxID); err != nil {
		uc.logger.Warn("Provider transaction ID already exists for deposit", zap.String("providerTxID", providerTxID), zap.Error(err))
		tx.Rollback()
		return nil, err
	}

	withdrawnTx, err := txTransactionRepo.GetByIDForUpdate(providerWithdrawnTxID)
	if err != nil {
		uc.logger.Error("Failed to get withdrawn transaction from database", zap.Int64("providerWithdrawnTxID", providerWithdrawnTxID), zap.Error(err))
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to get withdrawn transaction from DB", 500, err)
	}
	if withdrawnTx == nil {
		uc.logger.Warn("Withdrawn transaction not found", zap.Int64("providerWithdrawnTxID", providerWithdrawnTxID))
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeTransactionNotFound, "Withdrawn transaction not found", 404, nil)
	}

	if err := uc.validateTransactionOwnership(withdrawnTx, userID); err != nil {
		uc.logger.Error("Transaction ownership validation failed for deposit", zap.Int64("userID", userID), zap.Int64("withdrawnTxID", withdrawnTx.ID), zap.Error(err))
		tx.Rollback()
		return nil, err
	}

	if withdrawnTx.Status == domain.TransactionStatusCompleted {
		uc.logger.Warn("Withdrawn transaction already deposited", zap.Int64("withdrawnTxID", withdrawnTx.ID))
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeTransactionAlreadyDeposited, "Withdrawn transaction already deposited", 404, nil)
	}

	if err := uc.validateTransactionStatus(withdrawnTx, domain.TransactionStatusPending, "deposited"); err != nil {
		uc.logger.Error("Invalid transaction status for deposit", zap.Int64("withdrawnTxID", withdrawnTx.ID), zap.String("status", string(withdrawnTx.Status)), zap.Error(err))
		tx.Rollback()
		return nil, err
	}

	transaction := uc.createTransactionRecord(userID, domain.TransactionTypeDeposit, amount, currency, providerTxID, 0, 0, &providerWithdrawnTxID)

	if err := txTransactionRepo.Create(transaction); err != nil {
		uc.logger.Error("Failed to create deposit transaction in database", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Error(err))
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to create DB transaction", 500, err)
	}

	if err = tx.Commit().Error; err != nil {
		uc.logger.Error("Failed to commit deposit transaction", zap.Int64("transactionID", transaction.ID), zap.Error(err))
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

	walletReq := uc.createWalletRequest(userID, currency, amount, withdrawnTx.ID, providerTxID)

	uc.logger.Info("Calling wallet service for deposit", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID))
	walletResp, err := uc.walletSvc.Deposit(walletReq)

	tx, txTransactionRepo, txUserRepo, txErr := uc.setupTransactionWithRecovery()
	if txErr != nil {
		return nil, txErr
	}

	if err != nil {
		uc.logger.Error("Wallet service deposit failed", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Error(err))
		// Check for 409 conflicts (idempotency) - mark as completed
		if uc.is409Error(err) {
			uc.logger.Info("409 conflict detected for deposit, handling idempotency", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID))
			transaction, err := uc.handleDeposit409Conflict(transaction, withdrawnTx, txTransactionRepo, tx, userID)
			if err != nil {
				return nil, err
			}
			return transaction, nil
		}

		// Check for 4xx errors
		if uc.is4xxError(err) {
			uc.logger.Warn("4xx error from wallet service for deposit", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Error(err))
			return nil, uc.handleDepositFailure(transaction, txTransactionRepo, tx, err, 400)
		}

		// For 5xx errors (server errors)
		uc.logger.Error("5xx error from wallet service for deposit", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID), zap.Error(err))
		return nil, uc.handleDepositFailure(transaction, txTransactionRepo, tx, err, 500)
	}

	uc.logger.Info("Wallet service deposit successful", zap.Int64("userID", userID), zap.String("providerTxID", providerTxID))
	newBalance, err := uc.parseWalletBalance(walletResp.Balance)
	if err != nil {
		return nil, uc.handleBalanceParseError(transaction, txTransactionRepo, tx, err)
	}

	transaction.OldBalance = newBalance - amount
	transaction.NewBalance = newBalance

	if updateErr := uc.updateTransactionStatus(transaction, domain.TransactionStatusCompleted, txTransactionRepo, tx); updateErr != nil {
		return nil, updateErr
	}

	if updateErr := uc.updateTransactionStatus(withdrawnTx, domain.TransactionStatusCompleted, txTransactionRepo, tx); updateErr != nil {
		return nil, updateErr
	}

	if commitErr := uc.commitTransaction(tx); commitErr != nil {
		return nil, commitErr
	}

	uc.logger.Info("Deposit transaction completed successfully", zap.Int64("transactionID", transaction.ID), zap.Int64("userID", userID), zap.String("providerTxID", providerTxID))
	return transaction, nil
}
