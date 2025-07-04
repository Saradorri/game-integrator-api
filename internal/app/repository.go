package app

import (
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/infrastructure/repository"
	"gorm.io/gorm"
)

func (a *application) InitRepository(db *gorm.DB) (domain.UserRepository, domain.TransactionRepository) {
	return repository.NewUserRepository(db), repository.NewTransactionRepository(db)
}
