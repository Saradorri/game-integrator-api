package repository

import (
	"errors"
	"time"

	"github.com/saradorri/gameintegrator/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TransactionRepository implements domain.TransactionRepository
type TransactionRepository struct {
	db *gorm.DB
}

// NewTransactionRepository creates a new transaction repository
func NewTransactionRepository(db *gorm.DB) domain.TransactionRepository {
	return &TransactionRepository{db: db}
}

// Create creates a new transaction
func (r *TransactionRepository) Create(transaction *domain.Transaction) error {
	transaction.CreatedAt = time.Now()
	transaction.UpdatedAt = time.Now()
	return r.db.Create(transaction).Error
}

// GetByID retrieves a transaction by ID
func (r *TransactionRepository) GetByID(id int64) (*domain.Transaction, error) {
	var transaction domain.Transaction
	result := r.db.Where("id = ?", id).First(&transaction)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &transaction, nil
}

// GetByIDForUpdate retrieves a transaction by ID and `locks` it for update
func (r *TransactionRepository) GetByIDForUpdate(id int64) (*domain.Transaction, error) {
	var transaction domain.Transaction
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).First(&transaction).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &transaction, nil
}

// GetByProviderTxID retrieves a transaction by provider transaction ID
func (r *TransactionRepository) GetByProviderTxID(providerTxID string) (*domain.Transaction, error) {
	var transaction domain.Transaction
	result := r.db.Where("provider_tx_id = ?", providerTxID).First(&transaction)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &transaction, nil
}

// GetByProviderTxIDForUpdate retrieves a transaction by provider transaction ID and locks it for update
func (r *TransactionRepository) GetByProviderTxIDForUpdate(providerTxID string) (*domain.Transaction, error) {
	var transaction domain.Transaction
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("provider_tx_id = ?", providerTxID).First(&transaction).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &transaction, nil
}

// GetByUserID retrieves transactions for a user with pagination
func (r *TransactionRepository) GetByUserID(userID int64, limit, offset int) ([]*domain.Transaction, error) {
	var transactions []*domain.Transaction
	result := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&transactions)

	if result.Error != nil {
		return nil, result.Error
	}

	return transactions, nil
}

// Update updates an existing transaction
func (r *TransactionRepository) Update(transaction *domain.Transaction) error {
	transaction.UpdatedAt = time.Now()
	return r.db.Save(transaction).Error
}

// UpdateStatus updates only the status of a transaction
func (r *TransactionRepository) UpdateStatus(id int64, status domain.TransactionStatus) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	if status == domain.TransactionStatusCompleted {
		now := time.Now()
		updates["completed_at"] = &now
	}

	return r.db.Model(&domain.Transaction{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// GetPendingByUserID retrieves pending transactions for a user
func (r *TransactionRepository) GetPendingByUserID(userID int64) ([]*domain.Transaction, error) {
	var transactions []*domain.Transaction
	result := r.db.Where("user_id = ? AND status = ?", userID, domain.TransactionStatusPending).
		Order("created_at ASC").
		Find(&transactions)

	if result.Error != nil {
		return nil, result.Error
	}

	return transactions, nil
}

// GetByProviderWithdrawnTxID retrieves a transaction by provider withdrawn transaction ID
func (r *TransactionRepository) GetByProviderWithdrawnTxID(providerWithdrawnTxID int64) (*domain.Transaction, error) {
	var transaction domain.Transaction
	result := r.db.Where("provider_withdrawn_tx_id = ?", providerWithdrawnTxID).First(&transaction)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &transaction, nil
}

// WithTransaction returns a new repository instance with the given transaction
func (r *TransactionRepository) WithTransaction(tx *gorm.DB) domain.TransactionRepository {
	return &TransactionRepository{db: tx}
}
