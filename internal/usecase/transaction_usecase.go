package usecase

import (
	"fmt"
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

// updateUserBalance updates user balance and saves to database
func (uc *TransactionUseCase) updateUserBalance(repo domain.UserRepository, user *domain.User, newBalance float64) error {
	err := repo.UpdateBalance(user.ID, newBalance)
	if err != nil {
		return domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update user balance", 500, err)
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
	// lock user
	user, err := repo.GetByIDForUpdate(userID)
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
	user, err := repo.GetByIDForUpdate(userID)
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
		return domain.NewAppError(domain.ErrCodeTransactionCannotCancel, "Transaction cannot be cancelled", 400, nil)
	}
	return nil
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

	if err := uc.checkProviderTxIDExists(txTransactionRepo, providerTxID); err != nil {
		tx.Rollback()
		return nil, err
	}

	// User will be locked
	user, err := uc.getUserAndValidate(txUserRepo, userID, currency)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if user.Balance < amount {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeInsufficientBalance, "Insufficient balance", 400, nil)
	}

	transaction := &domain.Transaction{
		UserID:       userID,
		Type:         domain.TransactionTypeWithdraw,
		Status:       domain.TransactionStatusPending,
		Amount:       amount,
		Currency:     currency,
		ProviderTxID: providerTxID,
		OldBalance:   user.Balance,
		NewBalance:   user.Balance - amount,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := txTransactionRepo.Create(transaction); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to create transaction", 500, err)
	}

	if err := uc.updateUserBalance(txUserRepo, user, transaction.NewBalance); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
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

	// User will be locked
	user, err := uc.getUserAndValidate(txUserRepo, userID, currency)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := uc.checkProviderTxIDExists(txTransactionRepo, providerTxID); err != nil {
		tx.Rollback()
		return nil, err
	}

	transaction := &domain.Transaction{
		UserID:                userID,
		Type:                  domain.TransactionTypeDeposit,
		Status:                domain.TransactionStatusCompleted,
		Amount:                amount,
		Currency:              currency,
		ProviderTxID:          providerTxID,
		ProviderWithdrawnTxID: &providerWithdrawnTxID,
		OldBalance:            user.Balance,
		NewBalance:            user.Balance + amount,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	if err := txTransactionRepo.Create(transaction); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to create transaction", 500, err)
	}

	withdrawnTx, err := txTransactionRepo.GetByID(providerWithdrawnTxID)
	if err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to get withdrawn transaction", 500, err)
	}
	if withdrawnTx == nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeTransactionNotFound, "Withdrawn transaction not found", 404, nil)
	}

	if err := uc.validateTransactionOwnership(withdrawnTx, userID); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := uc.validateTransactionStatus(withdrawnTx, domain.TransactionStatusPending, "deposited"); err != nil {
		tx.Rollback()
		return nil, err
	}

	withdrawnTx.Status = domain.TransactionStatusCompleted
	withdrawnTx.UpdatedAt = time.Now()
	if err := txTransactionRepo.Update(withdrawnTx); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update withdrawn transaction", 500, err)
	}

	if err := uc.updateUserBalance(txUserRepo, user, transaction.NewBalance); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
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

	// User will be locked
	user, err := uc.getUserAndValidateWithoutCurrency(txUserRepo, userID)
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

	if err := uc.validateTransactionOwnership(originalTx, userID); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := uc.validateTransactionStatus(originalTx, domain.TransactionStatusPending, "cancelled"); err != nil {
		tx.Rollback()
		return nil, err
	}

	cancelTx := &domain.Transaction{
		UserID:       userID,
		Type:         domain.TransactionTypeCancel,
		Status:       domain.TransactionStatusCompleted,
		Amount:       originalTx.Amount,
		Currency:     originalTx.Currency,
		ProviderTxID: fmt.Sprintf("cancel_%s", providerTxID),
		OldBalance:   user.Balance,
		NewBalance:   user.Balance + originalTx.Amount,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
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

	if err := uc.updateUserBalance(txUserRepo, user, cancelTx.NewBalance); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseConnection, "Failed to commit transaction", 500, err)
	}

	return cancelTx, nil
}
