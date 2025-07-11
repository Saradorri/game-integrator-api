package transaction

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/saradorri/gameintegrator/internal/domain"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// *****  Database Transaction Management

// setupTransactionDB sets up a database transaction with repositories
func (uc *TransactionUseCase) setupTransactionDB() (*gorm.DB, domain.TransactionRepository, domain.UserRepository, error) {
	uc.logger.Info("Setting up database transaction")
	tx := uc.db.Begin()
	if tx.Error != nil {
		uc.logger.Error("Failed to start database transaction", zap.Error(tx.Error))
		return nil, nil, nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to start transaction", 500, tx.Error)
	}

	txTransactionRepo := uc.transactionRepo.WithTransaction(tx)
	txUserRepo := uc.userRepo.WithTransaction(tx)

	uc.logger.Info("Database transaction setup completed successfully")
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
	uc.logger.Info("Getting and validating user", zap.Int64("userID", userID), zap.String("currency", currency))
	user, err := repo.GetByID(userID)
	if err != nil {
		uc.logger.Error("Failed to get user from database", zap.Int64("userID", userID), zap.Error(err))
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to get user from DB", 500, err)
	}
	if user == nil {
		uc.logger.Warn("User not found", zap.Int64("userID", userID))
		return nil, domain.NewAppError(domain.ErrCodeUserNotFound, "User not found", 404, nil)
	}

	uc.logger.Info("User retrieved successfully", zap.Int64("userID", userID), zap.String("username", user.Username))
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
	uc.logger.Debug("Validating user data", zap.Int64("userID", userID), zap.String("expectedCurrency", currency), zap.String("userCurrency", user.Currency))

	if user.ID != userID {
		uc.logger.Warn("User ID mismatch", zap.Int64("expectedUserID", userID), zap.Int64("actualUserID", user.ID))
		return nil, domain.NewAppError(domain.ErrCodeUserNotFound, "User not found", 404, nil)
	}

	if user.Currency != currency {
		uc.logger.Warn("Currency mismatch", zap.String("expectedCurrency", currency), zap.String("userCurrency", user.Currency))
		return nil, domain.NewAppError(domain.ErrCodeInvalidCurrency, "User currency does not match", 400, nil)
	}

	uc.logger.Debug("User validation successful", zap.Int64("userID", userID))
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
	uc.logger.Info("Getting balance from wallet service", zap.Int64("userID", userID))
	walletResp, err := uc.walletSvc.GetBalance(userID)
	if err != nil {
		uc.logger.Error("Failed to get balance from wallet service", zap.Int64("userID", userID), zap.Error(err))
		return 0, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to get balance after 409", 500, err)
	}

	currentBalance, err := strconv.ParseFloat(walletResp.Balance, 64)
	if err != nil {
		uc.logger.Error("Failed to parse balance from wallet response", zap.String("balance", walletResp.Balance), zap.Error(err))
		return 0, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid balance format from wallet", 400, err)
	}

	uc.logger.Info("Balance retrieved successfully", zap.Int64("userID", userID), zap.Float64("balance", currentBalance))
	return currentBalance, nil
}

// ***** Transaction Management Utilities

// updateTransactionStatus updates transaction status with proper error handling
func (uc *TransactionUseCase) updateTransactionStatus(tx *domain.Transaction, status domain.TransactionStatus, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB) error {
	uc.logger.Info("Updating transaction status", zap.Int64("transactionID", tx.ID), zap.String("newStatus", string(status)), zap.String("oldStatus", string(tx.Status)))
	tx.Status = status
	tx.UpdatedAt = time.Now()

	if updateErr := txTransactionRepo.Update(tx); updateErr != nil {
		uc.logger.Error("Failed to update transaction status", zap.Int64("transactionID", tx.ID), zap.Error(updateErr))
		dbTx.Rollback()
		return domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update transaction status", 500, updateErr)
	}
	uc.logger.Info("Transaction status updated successfully", zap.Int64("transactionID", tx.ID), zap.String("status", string(status)))
	return nil
}

// parseWalletBalance parses balance from wallet service response
func (uc *TransactionUseCase) parseWalletBalance(balanceStr string) (float64, error) {
	uc.logger.Debug("Parsing wallet balance", zap.String("balanceString", balanceStr))
	balance, err := strconv.ParseFloat(balanceStr, 64)
	if err != nil {
		uc.logger.Error("Failed to parse wallet balance", zap.String("balanceString", balanceStr), zap.Error(err))
		return 0, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid balance format from wallet", 400, err)
	}
	uc.logger.Debug("Wallet balance parsed successfully", zap.Float64("balance", balance))
	return balance, nil
}

// createWalletRequest creates a wallet transaction request
func (uc *TransactionUseCase) createWalletRequest(userID int64, currency string, amount float64, betID int64, reference string) domain.WalletTransactionRequest {
	uc.logger.Debug("Creating wallet request", zap.Int64("userID", userID), zap.String("currency", currency), zap.Float64("amount", amount), zap.Int64("betID", betID), zap.String("reference", reference))
	walletReq := domain.WalletTransactionRequest{
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
	uc.logger.Debug("Wallet request created successfully")
	return walletReq
}

// createTransactionRecord creates a new transaction record
func (uc *TransactionUseCase) createTransactionRecord(userID int64, txType domain.TransactionType, amount float64, currency string, providerTxID string, oldBalance, newBalance float64, providerWithdrawnTxID *int64) *domain.Transaction {
	uc.logger.Info("Creating transaction record", zap.Int64("userID", userID), zap.String("type", string(txType)), zap.Float64("amount", amount), zap.String("currency", currency), zap.String("providerTxID", providerTxID))
	tx := &domain.Transaction{
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
	uc.logger.Info("Transaction record created successfully", zap.String("type", string(txType)), zap.String("providerTxID", providerTxID))
	return tx
}

// handleBalanceParseError handles balance parsing errors with transaction status update
func (uc *TransactionUseCase) handleBalanceParseError(tx *domain.Transaction, txTransactionRepo domain.TransactionRepository, dbTx *gorm.DB, err error) error {
	uc.logger.Error("Handling balance parse error", zap.Int64("transactionID", tx.ID), zap.Error(err))
	if updateErr := uc.updateTransactionStatus(tx, domain.TransactionStatusFailed, txTransactionRepo, dbTx); updateErr != nil {
		return updateErr
	}

	if commitErr := dbTx.Commit().Error; commitErr != nil {
		uc.logger.Error("Failed to commit transaction after balance parse error", zap.Int64("transactionID", tx.ID), zap.Error(commitErr))
		return domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, commitErr)
	}

	uc.logger.Error("Balance parse error handled, transaction marked as failed", zap.Int64("transactionID", tx.ID))
	return domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid balance format from wallet", 400, err)
}

// commitTransaction commits database transaction with error handling
func (uc *TransactionUseCase) commitTransaction(dbTx *gorm.DB) error {
	uc.logger.Debug("Committing database transaction")
	if err := dbTx.Commit().Error; err != nil {
		uc.logger.Error("Failed to commit database transaction", zap.Error(err))
		return domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}
	uc.logger.Debug("Database transaction committed successfully")
	return nil
}

// rollbackTransaction rolls back database transaction with error handling
func (uc *TransactionUseCase) rollbackTransaction(dbTx *gorm.DB, err error) error {
	uc.logger.Warn("Rolling back database transaction", zap.Error(err))
	dbTx.Rollback()
	return err
}
