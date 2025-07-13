package app

import (
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/infrastructure/auth"
	"github.com/saradorri/gameintegrator/internal/infrastructure/lock"
	"github.com/saradorri/gameintegrator/internal/infrastructure/logger"
	"github.com/saradorri/gameintegrator/internal/usecase/transaction"
	"github.com/saradorri/gameintegrator/internal/usecase/user"
	"gorm.io/gorm"
)

func (a *application) InitUserUseCase(ur domain.UserRepository, jwt auth.JWTService, logger *logger.Logger) domain.UserUseCase {
	return user.NewUserUseCase(ur, jwt, logger)
}

// InitTransactionUseCase creates the transaction usecase with all dependencies
func (a *application) InitTransactionUseCase(
	tr domain.TransactionRepository,
	ur domain.UserRepository,
	ws domain.WalletService,
	outboxRepo domain.OutboxRepository,
	db *gorm.DB,
	logger *logger.Logger,
	lock *lock.UserLockManager,
) domain.TransactionUseCase {
	return transaction.NewTransactionUseCase(tr, ur, ws, outboxRepo, db, logger, lock)
}
