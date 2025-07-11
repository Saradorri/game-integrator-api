package transaction

import (
	"github.com/saradorri/gameintegrator/internal/domain"
	"gorm.io/gorm"
)

// handleDeposit409Conflict handles 409 conflicts for deposit transactions
func (uc *TransactionUseCase) handleDeposit409Conflict(tx *domain.Transaction, withdrawnTx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, userID int64) (*domain.Transaction, error) {
	currentBalance, walletErr := uc.getBalance(userID)
	if walletErr != nil {
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

	return tx, nil
}

// handleDepositFailure handles failed deposit transactions with withdrawn transaction revert
func (uc *TransactionUseCase) handleDepositFailure(tx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, err error, statusCode int) error {
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
	if err := uc.validateDepositInput(amount, providerTxID); err != nil {
		return nil, err
	}

	tx, txTransactionRepo, txUserRepo, err := uc.setupTransactionWithRecovery()
	if err != nil {
		return nil, err
	}

	_, err = uc.getUserAndValidate(txUserRepo, userID, currency)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

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

	transaction := uc.createTransactionRecord(userID, domain.TransactionTypeDeposit, amount, currency, providerTxID, 0, 0, &providerWithdrawnTxID)

	if err := txTransactionRepo.Create(transaction); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to create DB transaction", 500, err)
	}

	if err = tx.Commit().Error; err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

	walletReq := uc.createWalletRequest(userID, currency, amount, withdrawnTx.ID, providerTxID)

	walletResp, err := uc.walletSvc.Deposit(walletReq)

	tx, txTransactionRepo, txUserRepo, txErr := uc.setupTransactionWithRecovery()
	if txErr != nil {
		return nil, txErr
	}

	if err != nil {
		// Check for 409 conflicts (idempotency) - mark as completed
		if uc.is409Error(err) {
			transaction, err := uc.handleDeposit409Conflict(transaction, withdrawnTx, txTransactionRepo, tx, userID)
			if err != nil {
				return nil, err
			}
			return transaction, nil
		}

		// Check for 4xx errors
		if uc.is4xxError(err) {
			return nil, uc.handleDepositFailure(transaction, txTransactionRepo, tx, err, 400)
		}

		// For 5xx errors (server errors)
		return nil, uc.handleDepositFailure(transaction, txTransactionRepo, tx, err, 500)
	}

	newBalance, err := uc.parseWalletBalance(walletResp.Balance)
	if err != nil {
		return nil, uc.handleBalanceParseError(transaction, txTransactionRepo, tx, err)
	}

	transaction.OldBalance = newBalance - amount
	transaction.NewBalance = newBalance

	if updateErr := uc.updateTransactionStatus(transaction, domain.TransactionStatusCompleted, txTransactionRepo, tx); updateErr != nil {
		return nil, updateErr
	}

	if commitErr := uc.commitTransaction(tx); commitErr != nil {
		return nil, commitErr
	}

	return transaction, nil
}
