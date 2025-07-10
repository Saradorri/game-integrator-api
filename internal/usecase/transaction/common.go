package transaction

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/saradorri/gameintegrator/internal/domain"
	"gorm.io/gorm"
)

// setupTransactionDB sets up a database transaction with repositories
func (uc *TransactionUseCase) setupTransactionDB() (*gorm.DB, domain.TransactionRepository, domain.UserRepository, error) {
	tx := uc.db.Begin()
	if tx.Error != nil {
		return nil, nil, nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to start transaction", 500, tx.Error)
	}

	txTransactionRepo := uc.transactionRepo.WithTransaction(tx)
	txUserRepo := uc.userRepo.WithTransaction(tx)

	return tx, txTransactionRepo, txUserRepo, nil
}

// getUserAndValidate validates user exists and currency matches
func (uc *TransactionUseCase) getUserAndValidate(repo domain.UserRepository, userID int64, currency string) (*domain.User, error) {
	user, err := repo.GetByID(userID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to get user from DB", 500, err)
	}
	if user == nil {
		return nil, domain.NewAppError(domain.ErrCodeUserNotFound, "User not found", 404, nil)
	}

	return uc.validateUser(user, userID, currency)
}

// getUserAndValidateWithoutCurrency validates user exists (no currency check)
func (uc *TransactionUseCase) getUserAndValidateWithoutCurrency(repo domain.UserRepository, userID int64) (*domain.User, error) {
	user, err := repo.GetByID(userID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to get user from DB", 500, err)
	}
	if user == nil {
		return nil, domain.NewAppError(domain.ErrCodeUserNotFound, "User not found", 404, nil)
	}

	return user, nil
}

// validateUser validates user data
func (uc *TransactionUseCase) validateUser(user *domain.User, userID int64, currency string) (*domain.User, error) {
	if user.ID != userID {
		return nil, domain.NewAppError(domain.ErrCodeUserNotFound, "User not found", 404, nil)
	}

	if user.Currency != currency {
		return nil, domain.NewAppError(domain.ErrCodeInvalidCurrency, "User currency does not match", 400, nil)
	}

	return user, nil
}

// checkProviderTxIDExists checks if provider transaction ID already exists
func (uc *TransactionUseCase) checkProviderTxIDExists(repo domain.TransactionRepository, providerTxID string) error {
	existingTx, err := repo.GetByProviderTxID(providerTxID)
	if err != nil {
		return domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to check provider transaction ID", 500, err)
	}
	if existingTx != nil {
		return domain.NewAppError(domain.ErrCodeTransactionAlreadyExists, "Provider transaction ID already exists", 409, nil)
	}
	return nil
}

// checkProviderTxIDExistsForUpdate checks if provider transaction ID exists with lock
func (uc *TransactionUseCase) checkProviderTxIDExistsForUpdate(repo domain.TransactionRepository, providerTxID string) error {
	existingTx, err := repo.GetByProviderTxIDForUpdate(providerTxID)
	if err != nil {
		return domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to check provider transaction ID", 500, err)
	}
	if existingTx != nil {
		return domain.NewAppError(domain.ErrCodeTransactionAlreadyExists, "Provider transaction ID already exists", 409, nil)
	}
	return nil
}

// checkProviderWithdrawalTxIDExists checks if provider withdrawal transaction ID exists
func (uc *TransactionUseCase) checkProviderWithdrawalTxIDExists(repo domain.TransactionRepository, providerWithdrawalTxID int64) error {
	existingTx, err := repo.GetByProviderWithdrawnTxID(providerWithdrawalTxID)
	if err != nil {
		return domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to check provider withdrawal transaction ID", 500, err)
	}
	if existingTx != nil {
		return domain.NewAppError(domain.ErrCodeWithdrawalTransactionDoseNotExists, "Provider withdrawal transaction ID already exists", 409, nil)
	}
	return nil
}

// validateTransactionOwnership validates that the transaction belongs to the user
func (uc *TransactionUseCase) validateTransactionOwnership(tx *domain.Transaction, userID int64) error {
	if tx.UserID != userID {
		return domain.NewForbiddenError("Transaction does not belong to user")
	}
	return nil
}

// validateTransactionStatus validates that the transaction has the expected status
func (uc *TransactionUseCase) validateTransactionStatus(tx *domain.Transaction, expectedStatus domain.TransactionStatus, operation string) error {
	if tx.Status != expectedStatus {
		return domain.NewAppError(domain.ErrCodeTransactionInvalidStatus, fmt.Sprintf("Transaction cannot be %s", operation), 400, nil)
	}
	return nil
}

// is4xxError checks if the error is a 4xx client error
func (uc *TransactionUseCase) is4xxError(err error) bool {
	var walletErr *domain.WalletServiceError
	if errors.As(err, &walletErr) {
		return walletErr.Is4xxError()
	}
	return false
}

// is409Error checks if the error is a 409 Conflict status code
func (uc *TransactionUseCase) is409Error(err error) bool {
	var walletErr *domain.WalletServiceError
	if errors.As(err, &walletErr) {
		return walletErr.StatusCode == 409
	}
	return false
}

// validateWithdrawInput validates withdrawal input parameters
func (uc *TransactionUseCase) validateWithdrawInput(amount float64, providerTxID string) error {
	if amount <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidAmount, "Amount must be greater than 0", 400, nil)
	}

	if providerTxID == "" {
		return domain.NewAppError(domain.ErrCodeRequiredField, "Provider transaction ID is required", 400, nil)
	}

	// Validate amount precision (max 2 decimal places)
	amountStr := strconv.FormatFloat(amount, 'f', -1, 64)
	if len(amountStr) > 0 {
		parts := strconv.FormatFloat(amount, 'f', -1, 64)
		if len(parts) > 0 {
			decimalIndex := -1
			for i, char := range parts {
				if char == '.' {
					decimalIndex = i
					break
				}
			}
			if decimalIndex != -1 && len(parts)-decimalIndex-1 > 2 {
				return domain.NewAppError(domain.ErrCodeInvalidPrecision, "Amount cannot have more than 2 decimal places", 400, nil)
			}
		}
	}

	return nil
}

// validateDepositInput validates deposit input parameters
func (uc *TransactionUseCase) validateDepositInput(amount float64, providerTxID string) error {
	if amount <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidAmount, "Amount must be greater than 0", 400, nil)
	}

	if providerTxID == "" {
		return domain.NewAppError(domain.ErrCodeRequiredField, "Provider transaction ID is required", 400, nil)
	}

	// Validate amount precision (max 2 decimal places)
	amountStr := strconv.FormatFloat(amount, 'f', -1, 64)
	if len(amountStr) > 0 {
		parts := strconv.FormatFloat(amount, 'f', -1, 64)
		if len(parts) > 0 {
			decimalIndex := -1
			for i, char := range parts {
				if char == '.' {
					decimalIndex = i
					break
				}
			}
			if decimalIndex != -1 && len(parts)-decimalIndex-1 > 2 {
				return domain.NewAppError(domain.ErrCodeInvalidPrecision, "Amount cannot have more than 2 decimal places", 400, nil)
			}
		}
	}

	return nil
}

// Revert creates a revert transaction for handling 409 conflicts
func (uc *TransactionUseCase) Revert(userID int64, providerTxID string, amount float64, txType domain.TransactionType) (*domain.Transaction, error) {
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

	// For revert, we can handle transactions that are in Failed status
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

	if err := txTransactionRepo.Create(revertTx); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to create revert transaction", 500, err)
	}

	// Mark original transaction as reverted
	originalTx.Status = domain.TransactionStatusCancelled
	originalTx.UpdatedAt = time.Now()
	if err := txTransactionRepo.Update(originalTx); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update original transaction", 500, err)
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
	if err != nil {
		// If wallet service fails, update transaction status to failed
		revertTx.Status = domain.TransactionStatusFailed
		revertTx.UpdatedAt = time.Now()
		if updateErr := uc.transactionRepo.Update(revertTx); updateErr != nil {
			log.Printf("Failed to update revert transaction status to failed: %v", updateErr)
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
		return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to process revert in wallet", 500, err)
	}

	// Parse balance from wallet response
	newBalance, err := strconv.ParseFloat(walletResp.Balance, 64)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid balance format from wallet", 400, nil)
	}

	// If wallet service succeeds, update transaction status to completed
	revertTx.Status = domain.TransactionStatusCompleted
	revertTx.OldBalance = newBalance - originalTx.Amount
	revertTx.NewBalance = newBalance
	revertTx.UpdatedAt = time.Now()
	if err := uc.transactionRepo.Update(revertTx); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update revert transaction status", 500, err)
	}

	return revertTx, nil
}
