package transaction

import (
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/infrastructure/logger"
	"gorm.io/gorm"
)

// TransactionUseCase defines the interface for transaction business logic
type TransactionUseCase struct {
	transactionRepo domain.TransactionRepository
	userRepo        domain.UserRepository
	walletSvc       domain.WalletService
	db              *gorm.DB
	logger          *logger.Logger
}

// NewTransactionUseCase creates a new transaction usecase
func NewTransactionUseCase(
	transactionRepo domain.TransactionRepository,
	userRepo domain.UserRepository,
	walletSvc domain.WalletService,
	db *gorm.DB,
	logger *logger.Logger,
) domain.TransactionUseCase {
	logger.Info("TransactionUseCase initialized successfully")
	return &TransactionUseCase{
		transactionRepo: transactionRepo,
		userRepo:        userRepo,
		walletSvc:       walletSvc,
		db:              db,
		logger:          logger,
	}
}

// Withdraw creates a withdrawal transaction
func (uc *TransactionUseCase) Withdraw(userID int64, amount float64, providerTxID string, currency string) (*domain.Transaction, error) {
	return uc.withdraw(userID, amount, providerTxID, currency)
}

// Deposit creates a deposit transaction
func (uc *TransactionUseCase) Deposit(userID int64, amount float64, providerTxID string, providerWithdrawnTxID int64, currency string) (*domain.Transaction, error) {
	return uc.deposit(userID, amount, providerTxID, providerWithdrawnTxID, currency)
}

// Cancel cancels a transaction
func (uc *TransactionUseCase) Cancel(userID int64, providerTxID string) (*domain.Transaction, error) {
	return uc.cancel(userID, providerTxID)
}
