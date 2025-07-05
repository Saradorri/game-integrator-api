package app

import (
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/infrastructure/auth"
	"github.com/saradorri/gameintegrator/internal/usecase"
)

func (a *application) InitUserUseCase(ur domain.UserRepository, jwt auth.JWTService) domain.UserUseCase {
	return usecase.NewUserUseCase(ur, jwt)
}
