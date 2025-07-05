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

// Withdraw creates a withdrawal transaction and calls the wallet service
func (uc *TransactionUseCase) Withdraw(userID int64, amount float64, providerTxID string, currency string) (*domain.Transaction, error) {
	if amount <= 0 {
		return nil, errors.New("amount must be positive")
	}
	if providerTxID == "" {
		return nil, errors.New("provider transaction ID is required")
	}

	// Start database transaction
	tx := uc.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	txTransactionRepo := uc.transactionRepo.WithTransaction(tx)
	txUserRepo := uc.userRepo.WithTransaction(tx)

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get user to check current balance
	user, err := txUserRepo.GetByID(userID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		tx.Rollback()
		return nil, errors.New("user not found")
	}
	if user.Currency != currency {
		tx.Rollback()
		return nil, fmt.Errorf("user currency (%s) does not match requested currency (%s)", user.Currency, currency)
	}

	// Check if user has sufficient balance
	if user.Balance < amount {
		tx.Rollback()
		return nil, errors.New("insufficient balance")
	}

	//
	existingTx, err := txTransactionRepo.GetByProviderTxID(providerTxID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to check existing transaction: %w", err)
	}
	if existingTx != nil {
		tx.Rollback()
		return nil, errors.New("provider transaction ID already exists")
	}

	// Create transaction record
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

	// Save transaction to database
	if err := txTransactionRepo.Create(transaction); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// TODO: send this transaction to wallet service async

	user.Balance = transaction.NewBalance
	user.UpdatedAt = time.Now()
	if err := txUserRepo.Update(user); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update user balance: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return transaction, nil
}

// Deposit creates a deposit transaction and calls the wallet service
func (uc *TransactionUseCase) Deposit(userID int64, amount float64, providerTxID string, providerWithdrawnTxID int64, currency string) (*domain.Transaction, error) {
	// Validate input
	if amount <= 0 {
		return nil, errors.New("amount must be positive")
	}
	if providerTxID == "" {
		return nil, errors.New("provider transaction ID is required")
	}

	// Start database transaction
	tx := uc.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	// Create repository instances with transaction
	txTransactionRepo := uc.transactionRepo.WithTransaction(tx)
	txUserRepo := uc.userRepo.WithTransaction(tx)

	// Defer rollback in case of error
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Check if provider transaction ID already exists
	existingTx, err := txTransactionRepo.GetByProviderTxID(providerTxID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to check existing transaction: %w", err)
	}
	if existingTx != nil {
		tx.Rollback()
		return nil, errors.New("provider transaction ID already exists")
	}

	// Get user to check current balance
	user, err := txUserRepo.GetByID(userID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		tx.Rollback()
		return nil, errors.New("user not found")
	}
	if user.Currency != currency {
		tx.Rollback()
		return nil, fmt.Errorf("user currency (%s) does not match requested currency (%s)", user.Currency, currency)
	}

	// Create transaction record
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

	// Save transaction to database
	if err := txTransactionRepo.Create(transaction); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	betTx, err := txTransactionRepo.GetByProviderWithdrawnTxID(providerWithdrawnTxID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to get provider withdrawn transaction: %w", err)
	}
	if betTx == nil {
		tx.Rollback()
		return nil, errors.New("provider withdrawn transaction not found")
	}

	betTx.Status = domain.TransactionStatusCompleted
	betTx.UpdatedAt = time.Now()
	if err := txTransactionRepo.Update(betTx); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update bet transaction: %w", err)
	}

	// TODO: send this transaction to wallet service async

	// Update user balance
	user.Balance = user.Balance + amount
	user.UpdatedAt = time.Now()
	if err := txUserRepo.Update(user); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update user balance: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return transaction, nil
}

// Cancel cancels a transaction
func (uc *TransactionUseCase) Cancel(userID int64, providerTxID string) (*domain.Transaction, error) {
	// Start database transaction
	tx := uc.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	// Create repository instances with transaction
	txTransactionRepo := uc.transactionRepo.WithTransaction(tx)
	txUserRepo := uc.userRepo.WithTransaction(tx)

	// Defer rollback in case of error
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get the original transaction
	betTx, err := txTransactionRepo.GetByProviderTxID(providerTxID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to get original transaction: %w", err)
	}
	if betTx == nil {
		tx.Rollback()
		return nil, errors.New("original transaction not found")
	}

	// Check if user owns the transaction
	if betTx.UserID != userID {
		tx.Rollback()
		return nil, errors.New("unauthorized to cancel this transaction")
	}

	// Check if transaction can be cancelled
	if betTx.Status != domain.TransactionStatusPending {
		tx.Rollback()
		return nil, errors.New("transaction cannot be cancelled")
	}

	// Get user to check current balance
	user, err := txUserRepo.GetByID(userID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		tx.Rollback()
		return nil, errors.New("user not found")
	}

	// Create cancellation transaction
	cancelTx := &domain.Transaction{
		UserID:       userID,
		Type:         domain.TransactionTypeCancel,
		Status:       domain.TransactionStatusCompleted,
		Amount:       betTx.Amount,
		Currency:     betTx.Currency,
		ProviderTxID: fmt.Sprintf("cancel_%s", providerTxID),
		OldBalance:   user.Balance,
		NewBalance:   user.Balance + betTx.Amount,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Save cancellation transaction
	if err := txTransactionRepo.Create(cancelTx); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create cancellation transaction: %w", err)
	}

	// Update original transaction status to cancelled
	betTx.Status = domain.TransactionStatusCancelled
	betTx.UpdatedAt = time.Now()
	if err := txTransactionRepo.Update(betTx); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update original transaction: %w", err)
	}

	user.Balance = user.Balance + betTx.Amount
	user.UpdatedAt = time.Now()
	if err := txUserRepo.Update(user); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update user balance: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return cancelTx, nil
}
