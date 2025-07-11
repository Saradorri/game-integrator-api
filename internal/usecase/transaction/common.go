package transaction

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/saradorri/gameintegrator/internal/domain"
	"gorm.io/gorm"
)

// *****  Database Transaction Management

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

// setupTransactionWithRecovery sets up a database transaction with panic recovery
func (uc *TransactionUseCase) setupTransactionWithRecovery() (*gorm.DB, domain.TransactionRepository, domain.UserRepository, error) {
	tx, txTransactionRepo, txUserRepo, err := uc.setupTransactionDB()
	if err != nil {
		return nil, nil, nil, err
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()
	}()

	return tx, txTransactionRepo, txUserRepo, nil
}

// *****  User Validation

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

// ***** Transaction Validation

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

// *****  Error Handling

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

// ***** Input Validation

// validateAmount validates amount is positive and has correct precision
func (uc *TransactionUseCase) validateAmount(amount float64) error {
	if amount <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidAmount, "Amount must be greater than 0", 400, nil)
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

// validateProviderTxID validates provider transaction ID is not empty
func (uc *TransactionUseCase) validateProviderTxID(providerTxID string) error {
	if providerTxID == "" {
		return domain.NewAppError(domain.ErrCodeRequiredField, "Provider transaction ID is required", 400, nil)
	}
	return nil
}

// validateWithdrawInput validates withdrawal input parameters
func (uc *TransactionUseCase) validateWithdrawInput(amount float64, providerTxID string) error {
	if err := uc.validateAmount(amount); err != nil {
		return err
	}
	return uc.validateProviderTxID(providerTxID)
}

// validateDepositInput validates deposit input parameters
func (uc *TransactionUseCase) validateDepositInput(amount float64, providerTxID string) error {
	if err := uc.validateAmount(amount); err != nil {
		return err
	}
	return uc.validateProviderTxID(providerTxID)
}

// ***** Wallet Service Utilities

// getBalance gets the current balance from wallet service
func (uc *TransactionUseCase) getBalance(userID int64) (float64, error) {
	walletResp, err := uc.walletSvc.GetBalance(userID)
	if err != nil {
		return 0, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to get balance after 409", 500, err)
	}

	currentBalance, err := strconv.ParseFloat(walletResp.Balance, 64)
	if err != nil {
		return 0, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid balance format from wallet", 400, err)
	}

	return currentBalance, nil
}

// ***** Transaction Management Utilities

// updateTransactionStatus updates transaction status with proper error handling
func (uc *TransactionUseCase) updateTransactionStatus(tx *domain.Transaction, status domain.TransactionStatus, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB) error {
	tx.Status = status
	tx.UpdatedAt = time.Now()

	if updateErr := txTransactionRepo.Update(tx); updateErr != nil {
		dbTx.Rollback()
		return domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update transaction status", 500, updateErr)
	}
	return nil
}

// parseWalletBalance parses balance from wallet service response
func (uc *TransactionUseCase) parseWalletBalance(balanceStr string) (float64, error) {
	balance, err := strconv.ParseFloat(balanceStr, 64)
	if err != nil {
		return 0, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid balance format from wallet", 400, err)
	}
	return balance, nil
}

// createWalletRequest creates a wallet transaction request
func (uc *TransactionUseCase) createWalletRequest(userID int64, currency string, amount float64, betID int64, reference string) domain.WalletTransactionRequest {
	return domain.WalletTransactionRequest{
		UserID:   userID,
		Currency: currency,
		Transactions: []domain.WalletRequestTransaction{
			{
				Amount:    amount,
				BetID:     betID,
				Reference: reference,
			},
		},
	}
}

// createTransactionRecord creates a new transaction record
func (uc *TransactionUseCase) createTransactionRecord(userID int64, txType domain.TransactionType, amount float64, currency string, providerTxID string, oldBalance, newBalance float64, providerWithdrawnTxID *int64) *domain.Transaction {
	return &domain.Transaction{
		UserID:                userID,
		Type:                  txType,
		Status:                domain.TransactionStatusSyncing,
		Amount:                amount,
		Currency:              currency,
		ProviderTxID:          providerTxID,
		ProviderWithdrawnTxID: providerWithdrawnTxID,
		OldBalance:            oldBalance,
		NewBalance:            newBalance,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
}

// handleBalanceParseError handles balance parsing errors with transaction status update
func (uc *TransactionUseCase) handleBalanceParseError(tx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, err error) error {
	if updateErr := uc.updateTransactionStatus(tx, domain.TransactionStatusFailed, txTransactionRepo, dbTx); updateErr != nil {
		return updateErr
	}

	if commitErr := dbTx.Commit().Error; commitErr != nil {
		return domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, commitErr)
	}

	return domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid balance format from wallet", 400, err)
}

// commitTransaction commits database transaction with error handling
func (uc *TransactionUseCase) commitTransaction(dbTx *gorm.DB) error {
	if err := dbTx.Commit().Error; err != nil {
		return domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}
	return nil
}

// rollbackTransaction rolls back database transaction with error handling
func (uc *TransactionUseCase) rollbackTransaction(dbTx *gorm.DB, err error) error {
	dbTx.Rollback()
	return err
}
