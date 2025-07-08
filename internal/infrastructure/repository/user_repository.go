package repository

import (
	"errors"
	"gorm.io/gorm/clause"
	"time"

	"github.com/saradorri/gameintegrator/internal/domain"
	"gorm.io/gorm"
)

// UserRepository implements domain.UserRepository
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB) domain.UserRepository {
	return &UserRepository{db: db}
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(id int64) (*domain.User, error) {
	var user domain.User
	result := r.db.Where("id = ?", id).First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &user, nil
}

// GetByUsername retrieves a user by username
func (r *UserRepository) GetByUsername(username string) (*domain.User, error) {
	var user domain.User
	result := r.db.Where("username = ?", username).First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &user, nil
}

// Create creates a new user
func (r *UserRepository) Create(user *domain.User) error {
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	return r.db.Create(user).Error
}

// Update updates an existing user
func (r *UserRepository) Update(user *domain.User) error {
	user.UpdatedAt = time.Now()
	return r.db.Save(user).Error
}

// WithTransaction returns a new repository instance with the given transaction
func (r *UserRepository) WithTransaction(tx *gorm.DB) domain.UserRepository {
	return &UserRepository{db: tx}
}

// GetByIDForUpdate returns user and lock it for preventing race condition
func (r *UserRepository) GetByIDForUpdate(id int64) (*domain.User, error) {
	var user domain.User
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
