package transaction

import (
	"strconv"
	"time"

	"github.com/saradorri/gameintegrator/internal/domain"
	"gorm.io/gorm"
)

// Revert creates a revert transaction for handling 409 conflicts
func (uc *TransactionUseCase) Revert(userID int64, providerTxID string, amount float64, txType domain.TransactionType) (*domain.Transaction, error) {
	tx, txTransactionRepo, txUserRepo, err := uc.setupTransactionWithRecovery()
	if err != nil {
		return nil, err
	}

	_, err = uc.getUserAndValidateWithoutCurrency(txUserRepo, userID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	originalTx, err := txTransactionRepo.GetByProviderTxID(providerTxID)
	if err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to get transaction", 500, err)
	}

	if originalTx == nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeTransactionNotFound, "Transaction not found", 404, nil)
	}

	if err = uc.validateTransactionOwnership(originalTx, userID); err != nil {
		tx.Rollback()
		return nil, err
	}

	if originalTx.Status != domain.TransactionStatusFailed {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeTransactionInvalidStatus, "Transaction cannot be reverted", 400, nil)
	}

	revertTx := &domain.Transaction{
		UserID:                userID,
		Type:                  domain.TransactionTypeRevert,
		Status:                domain.TransactionStatusSyncing,
		Amount:                originalTx.Amount,
		Currency:              originalTx.Currency,
		ProviderTxID:          "revert_" + providerTxID,
		ProviderWithdrawnTxID: &originalTx.ID,
		OldBalance:            0,
		NewBalance:            0,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	if err = txTransactionRepo.Create(revertTx); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to create revert transaction", 500, err)
	}

	if err = tx.Commit().Error; err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

	walletReq := domain.WalletTransactionRequest{
		UserID:   userID,
		Currency: originalTx.Currency,
		Transactions: []domain.WalletRequestTransaction{
			{
				Amount:    originalTx.Amount,
				BetID:     originalTx.ID,
				Reference: revertTx.ProviderTxID,
			},
		},
	}

	var walletResp domain.WalletTransactionResponse
	switch txType {
	case domain.TransactionTypeWithdraw:
		walletResp, err = uc.walletSvc.Withdraw(walletReq)
	case domain.TransactionTypeDeposit:
		walletResp, err = uc.walletSvc.Deposit(walletReq)
	default:
		return nil, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid transaction type for revert", 400, nil)
	}

	tx, txTransactionRepo, txUserRepo, txErr := uc.setupTransactionWithRecovery()
	if txErr != nil {
		return nil, txErr
	}

	if err != nil {
		return uc.handleRevertError(err, revertTx, txTransactionRepo, tx, userID, amount)
	}

	newBalance, err := strconv.ParseFloat(walletResp.Balance, 64)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid balance format from wallet", 400, nil)
	}

	revertTx.Status = domain.TransactionStatusCompleted
	revertTx.OldBalance = newBalance - originalTx.Amount
	revertTx.NewBalance = newBalance
	revertTx.UpdatedAt = time.Now()

	if err := txTransactionRepo.Update(revertTx); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update revert transaction status", 500, err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

	return revertTx, nil
}

// handleRevertError handles different types of errors during revert operation
func (uc *TransactionUseCase) handleRevertError(err error, revertTx *domain.Transaction, txTransactionRepo domain.TransactionRepository, tx *gorm.DB, userID int64, amount float64) (*domain.Transaction, error) {
	// Handle 409 Conflict - transaction already processed
	if uc.is409Error(err) {
		currentBalance, err := uc.getBalance(userID)
		if err != nil {
			return nil, err
		}

		revertTx.Status = domain.TransactionStatusCompleted
		revertTx.NewBalance = currentBalance
		revertTx.OldBalance = currentBalance - amount
		revertTx.UpdatedAt = time.Now()

		if updateErr := txTransactionRepo.Update(revertTx); updateErr != nil {
			return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update revert transaction status", 500, updateErr)
		}

		if commitErr := tx.Commit().Error; commitErr != nil {
			return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, commitErr)
		}

		return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to commit transaction; If transaction is not reverted after 1 hour, please contact support", 400, err)
	}

	// Handle 4xx client errors
	if uc.is4xxError(err) {
		revertTx.Status = domain.TransactionStatusFailed
		revertTx.UpdatedAt = time.Now()

		if updateErr := txTransactionRepo.Update(revertTx); updateErr != nil {
			return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update revert transaction status", 500, updateErr)
		}

		if commitErr := tx.Commit().Error; commitErr != nil {
			return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, commitErr)
		}

		return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, err.Error(), 400, err)
	}

	// Handle 5xx server errors
	return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to process revert in wallet", 500, err)
}
