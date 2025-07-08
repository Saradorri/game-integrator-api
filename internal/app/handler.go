package app

import (
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/http/handlers"
	"github.com/saradorri/gameintegrator/internal/infrastructure/auth"
)

func (a *application) InitUserHandler(uc domain.UserUseCase, walletSvc domain.WalletService, jwt auth.JWTService) *handlers.UserHandler {
	return handlers.NewUserHandler(uc, walletSvc, jwt)
}

func (a *application) InitTransactionHandler(tc domain.TransactionUseCase) *handlers.TransactionHandler {
	return handlers.NewTransactionHandler(tc)
}
