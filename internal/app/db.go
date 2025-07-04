package app

import (
	"github.com/saradorri/gameintegrator/internal/infrastructure/database"
	"gorm.io/gorm"
)

func (a *application) InitDatabase() (*gorm.DB, error) {
	dbConfig := &database.Config{
		Host:            a.config.Database.Host,
		Port:            a.config.Database.Port,
		User:            a.config.Database.User,
		Password:        a.config.Database.Password,
		Name:            a.config.Database.Name,
		SSLMode:         a.config.Database.SSLMode,
		MaxIdleConns:    a.config.Database.MaxIdleConns,
		MaxOpenConns:    a.config.Database.MaxOpenConns,
		ConnMaxLifetime: a.config.Database.ConnMaxLifetime,
	}
	db, err := database.NewDatabase(dbConfig)
	if err != nil {
		return nil, err
	}
	return db.GetDB(), nil
}
