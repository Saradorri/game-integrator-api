package app

import (
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/infrastructure/repository"
	"gorm.io/gorm"
)

func (a *application) InitUserRepository(db *gorm.DB) domain.UserRepository {
	return repository.NewUserRepository(db)
}

func (a *application) InitTransactionRepository(db *gorm.DB) domain.TransactionRepository {
	return repository.NewTransactionRepository(db)
}

func (a *application) InitOutboxRepository(db *gorm.DB) domain.OutboxRepository {
	return repository.NewOutboxRepository(db)
}
