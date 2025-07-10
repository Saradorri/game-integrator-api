package app

import (
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/infrastructure/auth"
	"github.com/saradorri/gameintegrator/internal/usecase/transaction"
	"github.com/saradorri/gameintegrator/internal/usecase/user"
	"gorm.io/gorm"
)

func (a *application) InitUserUseCase(ur domain.UserRepository, jwt auth.JWTService) domain.UserUseCase {
	return user.NewUserUseCase(ur, jwt)
}

// InitTransactionUseCase creates the transaction usecase with all dependencies
func (a *application) InitTransactionUseCase(
	tr domain.TransactionRepository,
	ur domain.UserRepository,
	ws domain.WalletService,
	db *gorm.DB,
) domain.TransactionUseCase {
	return transaction.NewTransactionUseCase(tr, ur, ws, db)
}
