package usecase

import (
	"errors"
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
		return errors.New("amount must be positive")
	}
	if providerTxID == "" {
		return errors.New("provider transaction ID is required")
	}
	return nil
}

// validateDepositInput validates deposit input parameters
func (uc *TransactionUseCase) validateDepositInput(amount float64, providerTxID string) error {
	if amount <= 0 {
		return errors.New("amount must be positive")
	}
	if providerTxID == "" {
		return errors.New("provider transaction ID is required")
	}
	return nil
}

// validateUser validates user exists and currency matches
func (uc *TransactionUseCase) validateUser(user *domain.User, userID int64, currency string) error {
	if user == nil {
		return errors.New("user not found")
	}
	if user.ID != userID {
		return errors.New("unauthorized to perform this operation")
	}
	if user.Currency != currency {
		return fmt.Errorf("user currency (%s) does not match requested currency (%s)", user.Currency, currency)
	}
	return nil
}

// checkProviderTxIDExists checks if provider transaction ID already exists
func (uc *TransactionUseCase) checkProviderTxIDExists(repo domain.TransactionRepository, providerTxID string) error {
	existingTx, err := repo.GetByProviderTxID(providerTxID)
	if err != nil {
		return fmt.Errorf("failed to check existing transaction: %w", err)
	}
	if existingTx != nil {
		return errors.New("provider transaction ID already exists")
	}
	return nil
}

// updateUserBalance updates user balance and saves to database
func (uc *TransactionUseCase) updateUserBalance(repo domain.UserRepository, user *domain.User, newBalance float64) error {
	return repo.UpdateBalance(user.ID, newBalance)
}

// setupTransactionDB sets up database transaction and repositories
func (uc *TransactionUseCase) setupTransactionDB() (*gorm.DB, domain.TransactionRepository, domain.UserRepository, error) {
	tx := uc.db.Begin()
	if tx.Error != nil {
		return nil, nil, nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	txTransactionRepo := uc.transactionRepo.WithTransaction(tx)
	txUserRepo := uc.userRepo.WithTransaction(tx)

	return tx, txTransactionRepo, txUserRepo, nil
}

// getUserAndValidate retrieves user and validates ownership and currency
func (uc *TransactionUseCase) getUserAndValidate(repo domain.UserRepository, userID int64, currency string) (*domain.User, error) {
	user, err := repo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if err := uc.validateUser(user, userID, currency); err != nil {
		return nil, err
	}

	return user, nil
}

// validateTransactionOwnership validates that a transaction belongs to the user
func (uc *TransactionUseCase) validateTransactionOwnership(tx *domain.Transaction, userID int64) error {
	if tx.UserID != userID {
		return errors.New("unauthorized to perform this operation")
	}
	return nil
}

// validateTransactionStatus validates that a transaction has the expected status
func (uc *TransactionUseCase) validateTransactionStatus(tx *domain.Transaction, expectedStatus domain.TransactionStatus, operation string) error {
	if tx.Status != expectedStatus {
		return fmt.Errorf("transaction cannot be %s", operation)
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

	user, err := uc.getUserAndValidate(txUserRepo, userID, currency)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if user.Balance < amount {
		tx.Rollback()
		return nil, errors.New("insufficient balance")
	}

	if err := uc.checkProviderTxIDExists(txTransactionRepo, providerTxID); err != nil {
		tx.Rollback()
		return nil, err
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
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	if err := uc.updateUserBalance(txUserRepo, user, transaction.NewBalance); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update user balance: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
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

	if err := uc.checkProviderTxIDExists(txTransactionRepo, providerTxID); err != nil {
		tx.Rollback()
		return nil, err
	}

	user, err := uc.getUserAndValidate(txUserRepo, userID, currency)
	if err != nil {
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
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	withdrawnTx, err := txTransactionRepo.GetByID(providerWithdrawnTxID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to get provider withdrawn transaction: %w", err)
	}
	if withdrawnTx == nil {
		tx.Rollback()
		return nil, errors.New("provider withdrawn transaction not found")
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
		return nil, fmt.Errorf("failed to update withdrawn transaction: %w", err)
	}

	if err := uc.updateUserBalance(txUserRepo, user, transaction.NewBalance); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update user balance: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
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

	originalTx, err := txTransactionRepo.GetByProviderTxID(providerTxID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to get original transaction: %w", err)
	}
	if originalTx == nil {
		tx.Rollback()
		return nil, errors.New("original transaction not found")
	}

	if err := uc.validateTransactionOwnership(originalTx, userID); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := uc.validateTransactionStatus(originalTx, domain.TransactionStatusPending, "cancelled"); err != nil {
		tx.Rollback()
		return nil, err
	}

	user, err := txUserRepo.GetByID(userID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		tx.Rollback()
		return nil, errors.New("user not found")
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
		return nil, fmt.Errorf("failed to create cancellation transaction: %w", err)
	}

	originalTx.Status = domain.TransactionStatusCancelled
	originalTx.UpdatedAt = time.Now()
	if err := txTransactionRepo.Update(originalTx); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update original transaction: %w", err)
	}

	if err := uc.updateUserBalance(txUserRepo, user, cancelTx.NewBalance); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update user balance: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return cancelTx, nil
}
