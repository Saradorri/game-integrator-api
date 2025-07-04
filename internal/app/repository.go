package app

import (
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/infrastructure/repository"
	"gorm.io/gorm"
)

func (a *application) InitUserRepository(db *gorm.DB) domain.UserRepository {
	return repository.NewUserRepository(db)
}
