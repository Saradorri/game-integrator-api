package usecase

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/saradorri/gameintegrator/internal/domain"
	"gorm.io/gorm"
)

// TransactionUseCase implements domain.TransactionUseCase
type TransactionUseCase struct {
	transactionRepo domain.TransactionRepository
	userRepo        domain.UserRepository
	walletSvc       domain.WalletService
	db              *gorm.DB
}

// NewTransactionUseCase creates a new transaction use case
func NewTransactionUseCase(
	transactionRepo domain.TransactionRepository,
	userRepo domain.UserRepository,
	walletSvc domain.WalletService,
	db *gorm.DB,
) domain.TransactionUseCase {
	return &TransactionUseCase{
		transactionRepo: transactionRepo,
		userRepo:        userRepo,
		walletSvc:       walletSvc,
		db:              db,
	}
}

// validateWithdrawInput validates withdrawal input parameters
func (uc *TransactionUseCase) validateWithdrawInput(amount float64, providerTxID string) error {
	if amount <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidAmount, "Amount must be greater than zero", 400, nil)
	}
	if providerTxID == "" {
		return domain.NewAppError(domain.ErrCodeRequiredField, "Provider transaction ID required", 400, nil)
	}
	return nil
}

// validateDepositInput validates deposit input parameters
func (uc *TransactionUseCase) validateDepositInput(amount float64, providerTxID string) error {
	if amount <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidAmount, "Amount must be greater than zero", 400, nil)
	}
	if providerTxID == "" {
		return domain.NewAppError(domain.ErrCodeRequiredField, "Provider transaction ID required", 400, nil)
	}
	return nil
}

// validateUser validates user exists and currency matches
func (uc *TransactionUseCase) validateUser(user *domain.User, userID int64, currency string) error {
	if user == nil {
		return domain.NewAppError(domain.ErrCodeUserNotFound, "User not found", 404, nil)
	}
	if user.ID != userID {
		return domain.NewForbiddenError("Unauthorized operation")
	}
	if user.Currency != currency {
		return domain.NewAppError(domain.ErrCodeInvalidCurrency, "Currency mismatch", 400, nil)
	}
	return nil
}

// checkProviderTxIDExists checks if provider transaction ID already exists
func (uc *TransactionUseCase) checkProviderTxIDExists(repo domain.TransactionRepository, providerTxID string) error {
	existingTx, err := repo.GetByProviderTxID(providerTxID)
	if err != nil {
		return domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to check existing transaction", 500, err)
	}
	if existingTx != nil {
		return domain.NewAppError(domain.ErrCodeTransactionAlreadyExists, "Transaction already exists", 409, nil)
	}
	return nil
}

// checkProviderTxIDExistsForUpdate checks if provider transaction ID already exists with lock
func (uc *TransactionUseCase) checkProviderTxIDExistsForUpdate(repo domain.TransactionRepository, providerTxID string) error {
	existingTx, err := repo.GetByProviderTxIDForUpdate(providerTxID)
	if err != nil {
		return domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to check existing transaction", 500, err)
	}
	if existingTx != nil {
		return domain.NewAppError(domain.ErrCodeTransactionAlreadyExists, "Transaction already exists", 409, nil)
	}
	return nil
}

func (uc *TransactionUseCase) checkProviderWithdrawalTxIDExists(repo domain.TransactionRepository, providerWithdrawalTxID int64) error {
	existingTx, err := repo.GetByID(providerWithdrawalTxID)
	if err != nil {
		return domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to check existing transaction", 500, err)
	}
	if existingTx == nil {
		return domain.NewAppError(domain.ErrCodeWithdrawalTransactionDoseNotExists, "Withdrawal transaction does not exists", 400, nil)
	}
	return nil
}

// setupTransactionDB sets up database transaction and repositories
func (uc *TransactionUseCase) setupTransactionDB() (*gorm.DB, domain.TransactionRepository, domain.UserRepository, error) {
	tx := uc.db.Begin()
	if tx.Error != nil {
		return nil, nil, nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to start transaction", 500, tx.Error)
	}

	txTransactionRepo := uc.transactionRepo.WithTransaction(tx)
	txUserRepo := uc.userRepo.WithTransaction(tx)

	return tx, txTransactionRepo, txUserRepo, nil
}

// getUserAndValidate retrieves user and validates ownership and currency
func (uc *TransactionUseCase) getUserAndValidate(repo domain.UserRepository, userID int64, currency string) (*domain.User, error) {
	user, err := repo.GetByID(userID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to get user", 500, err)
	}

	if err := uc.validateUser(user, userID, currency); err != nil {
		return nil, err
	}

	return user, nil
}

// getUserAndValidateWithoutCurrency retrieves user and validates ownership (for cancel operations)
func (uc *TransactionUseCase) getUserAndValidateWithoutCurrency(repo domain.UserRepository, userID int64) (*domain.User, error) {
	user, err := repo.GetByID(userID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to get user", 500, err)
	}

	if user == nil {
		return nil, domain.NewAppError(domain.ErrCodeUserNotFound, "User not found", 404, nil)
	}
	if user.ID != userID {
		return nil, domain.NewForbiddenError("Unauthorized operation")
	}

	return user, nil
}

// validateTransactionOwnership validates that a transaction belongs to the user
func (uc *TransactionUseCase) validateTransactionOwnership(tx *domain.Transaction, userID int64) error {
	if tx.UserID != userID {
		return domain.NewForbiddenError("Unauthorized operation")
	}
	return nil
}

// validateTransactionStatus validates that a transaction has the expected status
func (uc *TransactionUseCase) validateTransactionStatus(tx *domain.Transaction, expectedStatus domain.TransactionStatus, operation string) error {
	if tx.Status != expectedStatus {
		return domain.NewAppError(domain.ErrCodeTransactionInvalidStatus, fmt.Sprintf("Transaction cannot be %s", operation), 400, nil)
	}
	return nil
}

// is4xxError checks if the error is a 4xx client error from wallet service
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

// Withdraw creates a withdrawal transaction
func (uc *TransactionUseCase) Withdraw(userID int64, amount float64, providerTxID string, currency string) (*domain.Transaction, error) {
	if err := uc.validateWithdrawInput(amount, providerTxID); err != nil {
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

	// Check provider transaction ID  (no lock needed - transaction is not modified)
	if err := uc.checkProviderTxIDExists(txTransactionRepo, providerTxID); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Validate user exists and currency matches (no lock needed - user is not modified)
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
		OldBalance:   0, // Will be updated after wallet service call
		NewBalance:   0, // Will be updated after wallet service call
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := txTransactionRepo.Create(transaction); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to create transaction", 500, err)
	}

	// Commit the transaction quickly to release locks
	if err := tx.Commit().Error; err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

	// Send transaction to wallet service
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
	if err != nil {
		// If wallet service fails, update transaction status to failed
		transaction.Status = domain.TransactionStatusFailed
		transaction.UpdatedAt = time.Now()
		if updateErr := uc.transactionRepo.Update(transaction); updateErr != nil {
			log.Printf("Failed to update transaction status to failed: %v", updateErr)
		}

		log.Printf("Withdraw wallet service failed for transaction %d: %v", transaction.ID, err)

		// check if error is 409
		if uc.is409Error(err) {
			// Create a revert transaction for 409 conflicts that occur after 5xx error in wallet service
			_, err := uc.Revert(userID, providerTxID, amount, domain.TransactionTypeDeposit)
			if err != nil {
				return nil, err
			}

			return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to commit transaction; If transaction is not reverted after 1 hour, please contact support", 400, err)
		}

		return nil, err
	}

	// If wallet service succeeds, update transaction status to pending
	transaction.Status = domain.TransactionStatusPending

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

	transaction.OldBalance = newBalance + amount
	transaction.NewBalance = newBalance
	transaction.UpdatedAt = time.Now()
	if err := uc.transactionRepo.Update(transaction); err != nil {
		log.Printf("Failed to update transaction status for %d: %v", transaction.ID, err)
		// TODO: Implement retry mechanism for failed updates
		return nil, err
	}

	return transaction, nil
}

// Deposit creates a deposit transaction
func (uc *TransactionUseCase) Deposit(userID int64, amount float64, providerTxID string, providerWithdrawnTxID int64, currency string) (*domain.Transaction, error) {
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

// Cancel cancels a transaction
func (uc *TransactionUseCase) Cancel(userID int64, providerTxID string) (*domain.Transaction, error) {
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

	if err := uc.validateTransactionStatus(originalTx, domain.TransactionStatusPending, "cancelled"); err != nil {
		tx.Rollback()
		return nil, err
	}

	cancelTx := &domain.Transaction{
		UserID:                userID,
		Type:                  domain.TransactionTypeCancel,
		Status:                domain.TransactionStatusSyncing,
		Amount:                originalTx.Amount,
		Currency:              originalTx.Currency,
		ProviderTxID:          fmt.Sprintf("cancel_%s", providerTxID),
		ProviderWithdrawnTxID: &originalTx.ID,
		OldBalance:            0,
		NewBalance:            0,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	if err := txTransactionRepo.Create(cancelTx); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to create cancel transaction", 500, err)
	}

	originalTx.Status = domain.TransactionStatusCancelled
	originalTx.UpdatedAt = time.Now()
	if err := txTransactionRepo.Update(originalTx); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update transaction", 500, err)
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
				Reference: cancelTx.ProviderTxID,
			},
		},
	}

	walletResp, err := uc.walletSvc.Deposit(walletReq)
	if err != nil {
		// If wallet service fails, update transaction status to failed
		cancelTx.Status = domain.TransactionStatusFailed
		cancelTx.UpdatedAt = time.Now()
		if updateErr := uc.transactionRepo.Update(cancelTx); updateErr != nil {
			log.Printf("Failed to update cancel transaction status to failed: %v", updateErr)
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
		return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to process cancel in wallet", 500, err)
	}

	// Parse balance from wallet response
	newBalance, err := strconv.ParseFloat(walletResp.Balance, 64)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid balance format from wallet", 400, nil)
	}

	// If wallet service succeeds, update transaction status to completed
	cancelTx.Status = domain.TransactionStatusCompleted
	cancelTx.OldBalance = newBalance - originalTx.Amount
	cancelTx.NewBalance = newBalance
	cancelTx.UpdatedAt = time.Now()
	if err := uc.transactionRepo.Update(cancelTx); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update cancel transaction status", 500, err)
	}

	return cancelTx, nil
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
		ProviderTxID:          fmt.Sprintf("revert_%s", providerTxID),
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
