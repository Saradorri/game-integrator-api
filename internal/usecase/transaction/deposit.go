package transaction

import (
	"strconv"
	"time"

	"github.com/saradorri/gameintegrator/internal/domain"
	"gorm.io/gorm"
)

// handleDeposit409Conflict handles 409 conflicts for deposit transactions
func (uc *TransactionUseCase) handleDeposit409Conflict(tx *domain.Transaction, withdrawnTx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, userID int64) (*domain.Transaction, error) {
	currentBalance, walletErr := uc.getBalance(userID)
	if walletErr != nil {
		return nil, walletErr
	}

	tx.Status = domain.TransactionStatusCompleted
	tx.NewBalance = currentBalance
	tx.OldBalance = currentBalance - tx.Amount
	tx.UpdatedAt = time.Now()

	if updateErr := txTransactionRepo.Update(tx); updateErr != nil {
		dbTx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to update transaction status", 500, updateErr)
	}

	withdrawnTx.Status = domain.TransactionStatusCompleted
	withdrawnTx.UpdatedAt = time.Now()

	if updateErr := txTransactionRepo.Update(withdrawnTx); updateErr != nil {
		dbTx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to update transaction status", 500, updateErr)
	}

	if commitErr := dbTx.Commit().Error; commitErr != nil {
		dbTx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, commitErr)
	}

	return tx, nil
}

// handleDepositFailure handles failed deposit transactions with withdrawn transaction revert
func (uc *TransactionUseCase) handleDepositFailure(tx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, err error, statusCode int) error {
	// Deposit transaction failed
	tx.Status = domain.TransactionStatusFailed
	tx.UpdatedAt = time.Now()

	if updateErr := txTransactionRepo.Update(tx); updateErr != nil {
		dbTx.Rollback()
		return domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to update transaction status", 500, updateErr)
	}

	if commitErr := dbTx.Commit().Error; commitErr != nil {
		return domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, commitErr)
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

	if err = tx.Commit().Error; err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

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

	newBalance, err := strconv.ParseFloat(walletResp.Balance, 64)
	if err != nil {
		return nil, uc.handleBalanceParseError(transaction, txTransactionRepo, tx, err)
	}

	transaction.Status = domain.TransactionStatusCompleted
	transaction.OldBalance = newBalance - amount
	transaction.NewBalance = newBalance
	transaction.UpdatedAt = time.Now()

	if updateErr := txTransactionRepo.Update(transaction); updateErr != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to update transaction status", 500, updateErr)
	}

	if commitErr := tx.Commit().Error; commitErr != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, commitErr)
	}

	return transaction, nil
}
