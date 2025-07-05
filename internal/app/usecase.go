package app

import (
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/infrastructure/auth"
	"github.com/saradorri/gameintegrator/internal/usecase"
	"gorm.io/gorm"
)

func (a *application) InitUserUseCase(ur domain.UserRepository, jwt auth.JWTService) domain.UserUseCase {
	return usecase.NewUserUseCase(ur, jwt)
}

func (a *application) InitTransactionUseCase(
	tr domain.TransactionRepository,
	ur domain.UserRepository,
	ws domain.WalletService,
	db *gorm.DB,
) domain.TransactionUseCase {
	return usecase.NewTransactionUseCase(tr, ur, ws, db)
}
