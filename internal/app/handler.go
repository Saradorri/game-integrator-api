package app

import (
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/http/handlers"
	"github.com/saradorri/gameintegrator/internal/infrastructure/auth"
	"github.com/saradorri/gameintegrator/internal/infrastructure/logger"
)

func (a *application) InitUserHandler(uc domain.UserUseCase, walletSvc domain.WalletService, jwt auth.JWTService, log *logger.Logger) *handlers.UserHandler {
	return handlers.NewUserHandler(uc, walletSvc, jwt, log)
}

func (a *application) InitTransactionHandler(tc domain.TransactionUseCase, log *logger.Logger) *handlers.TransactionHandler {
	return handlers.NewTransactionHandler(tc, log)
}
