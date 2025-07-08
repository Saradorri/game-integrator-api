package usecase

import (
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
	_, err = uc.getUserAndValidate(txUserRepo, userID, currency)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Get balance from wallet service
	walletBalance, err := uc.walletSvc.GetBalance(userID)
	if err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to get wallet balance", 500, err)
	}

	balance, err := strconv.ParseFloat(walletBalance.Balance, 64)
	if err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid balance format", 400, nil)
	}

	if balance < amount {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeInsufficientBalance, "Insufficient balance", 400, nil)
	}

	transaction := &domain.Transaction{
		UserID:       userID,
		Type:         domain.TransactionTypeWithdraw,
		Status:       domain.TransactionStatusSyncing,
		Amount:       amount,
		Currency:     currency,
		ProviderTxID: providerTxID,
		OldBalance:   balance,
		NewBalance:   balance - amount,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := txTransactionRepo.Create(transaction); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to create transaction", 500, err)
	}

	// Commit the transaction first to get the transaction ID
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

	_, err = uc.walletSvc.Withdraw(walletReq)
	if err != nil {
		// If wallet service fails, update transaction status to failed
		transaction.Status = domain.TransactionStatusFailed
		transaction.UpdatedAt = time.Now()
		if updateErr := uc.transactionRepo.Update(transaction); updateErr != nil {
			log.Printf("Failed to update transaction status to failed: %v", updateErr)
		}
		return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to process withdrawal in wallet and transaction failed", 500, err)
	}

	// If wallet service succeeds, update transaction status to pending
	transaction.Status = domain.TransactionStatusPending
	transaction.UpdatedAt = time.Now()
	if err := uc.transactionRepo.Update(transaction); err != nil {
		//TODO: due to transaction was sent to wallet, need a job to retry this update status to be synced with wallet
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update transaction status", 500, err)
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
	_, err = uc.getUserAndValidate(txUserRepo, userID, currency)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := uc.checkProviderTxIDExists(txTransactionRepo, providerTxID); err != nil {
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

	walletBalance, err := uc.walletSvc.GetBalance(userID)
	if err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to get wallet balance", 500, err)
	}

	balance, err := strconv.ParseFloat(walletBalance.Balance, 64)
	if err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid balance format", 400, nil)
	}

	transaction := &domain.Transaction{
		UserID:                userID,
		Type:                  domain.TransactionTypeDeposit,
		Status:                domain.TransactionStatusSyncing,
		Amount:                amount,
		Currency:              currency,
		ProviderTxID:          providerTxID,
		ProviderWithdrawnTxID: &providerWithdrawnTxID,
		OldBalance:            balance,
		NewBalance:            balance + amount,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	if err := txTransactionRepo.Create(transaction); err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to create DB transaction", 500, err)
	}

	if err := tx.Commit().Error; err != nil {
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

	_, err = uc.walletSvc.Deposit(walletReq)
	if err != nil {
		transaction.Status = domain.TransactionStatusFailed
		transaction.UpdatedAt = time.Now()
		if updateErr := uc.transactionRepo.Update(transaction); updateErr != nil {
			log.Printf("Failed to update transaction status to failed: %v", updateErr)
		}
		return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to process deposit in wallet", 500, err)
	}

	transaction.Status = domain.TransactionStatusCompleted
	transaction.UpdatedAt = time.Now()
	if err := uc.transactionRepo.Update(transaction); err != nil {
		//TODO: due to transaction was sent to wallet, need a job to retry this update status to be synced with wallet
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update transaction status", 500, err)
	}

	withdrawnTx.Status = domain.TransactionStatusCompleted
	withdrawnTx.UpdatedAt = time.Now()
	if err := uc.transactionRepo.Update(withdrawnTx); err != nil {
		//TODO: due to transaction was sent to wallet, need a job to retry this update status to be synced with wallet
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update withdrawn transaction", 500, err)
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

	if err := uc.validateTransactionOwnership(originalTx, userID); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := uc.validateTransactionStatus(originalTx, domain.TransactionStatusPending, "cancelled"); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Get balance from wallet service
	walletBalance, err := uc.walletSvc.GetBalance(userID)
	if err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to get wallet balance", 500, err)
	}

	balance, err := strconv.ParseFloat(walletBalance.Balance, 64)
	if err != nil {
		tx.Rollback()
		return nil, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid balance format", 400, nil)
	}

	cancelTx := &domain.Transaction{
		UserID:       userID,
		Type:         domain.TransactionTypeCancel,
		Status:       domain.TransactionStatusSyncing,
		Amount:       originalTx.Amount,
		Currency:     originalTx.Currency,
		ProviderTxID: fmt.Sprintf("cancel_%s", providerTxID),
		OldBalance:   balance,
		NewBalance:   balance + originalTx.Amount,
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

	_, err = uc.walletSvc.Withdraw(walletReq)
	if err != nil {
		// If wallet service fails, update transaction status to failed
		cancelTx.Status = domain.TransactionStatusFailed
		cancelTx.UpdatedAt = time.Now()
		if updateErr := uc.transactionRepo.Update(cancelTx); updateErr != nil {
			log.Printf("Failed to update cancel transaction status to failed: %v", updateErr)
		}
		return nil, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to process cancel in wallet", 500, err)
	}

	// If wallet service succeeds, update transaction status to completed
	cancelTx.Status = domain.TransactionStatusCompleted
	cancelTx.UpdatedAt = time.Now()
	if err := uc.transactionRepo.Update(cancelTx); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeDatabaseQuery, "Failed to update cancel transaction status", 500, err)
	}

	return cancelTx, nil
}
