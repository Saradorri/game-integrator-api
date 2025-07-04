package domain

import (
	"time"

	"gorm.io/gorm"
)

// TransactionType represents the type of transaction
type TransactionType string

const (
	TransactionTypeWithdraw TransactionType = "withdraw"
	TransactionTypeDeposit  TransactionType = "deposit"
	TransactionTypeCancel   TransactionType = "cancel"
)

// TransactionStatus represents the status of a transaction
type TransactionStatus string

const (
	TransactionStatusPending   TransactionStatus = "pending"
	TransactionStatusCompleted TransactionStatus = "completed"
	TransactionStatusFailed    TransactionStatus = "failed"
	TransactionStatusCancelled TransactionStatus = "cancelled"
)

// Transaction represents a financial transaction in the system
type Transaction struct {
	ID           string            `json:"transaction_id" gorm:"primaryKey;column:id;type:varchar(32)"`
	UserID       string            `json:"user_id" gorm:"index;not null;type:varchar(32)"`
	Type         TransactionType   `json:"type" gorm:"type:varchar(16);not null"`
	Status       TransactionStatus `json:"status" gorm:"type:varchar(16);not null;default:'pending'"`
	Amount       float64           `json:"amount" gorm:"type:numeric(20,2);not null"`
	Currency     string            `json:"currency" gorm:"type:varchar(8);not null"`
	GameID       string            `json:"game_id" gorm:"type:varchar(64)"`
	SessionID    *string           `json:"session_id,omitempty" gorm:"type:varchar(64)"`
	ProviderTxID string            `json:"provider_tx_id" gorm:"uniqueIndex;type:varchar(64)"`
	Description  string            `json:"description" gorm:"type:text"`
	CreatedAt    time.Time         `json:"created_at" gorm:"not null"`
	UpdatedAt    time.Time         `json:"updated_at" gorm:"not null"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty" gorm:"index"`
	DeletedAt    gorm.DeletedAt    `json:"-" gorm:"index"`

	User User `json:"-" gorm:"foreignKey:UserID"`
}

// TableName specifies the table name for Transaction
func (T Transaction) TableName() string {
	return "transactions"
}

// TransactionRepository defines the interface for transaction data
type TransactionRepository interface {
	Create(transaction *Transaction) error
	GetByID(id string) (*Transaction, error)
	GetByProviderID(providerID string) (*Transaction, error)
	GetByUserID(userID string, limit, offset int) ([]*Transaction, error)
	Update(transaction *Transaction) error
	UpdateStatus(id string, status TransactionStatus) error
	GetPendingByUserID(userID string) ([]*Transaction, error)
}

// TransactionUseCase defines the interface for transaction business logic
type TransactionUseCase interface {
	Withdraw(userID string, amount float64, gameID, sessionID, referenceID string) (*Transaction, error)
	Deposit(userID string, amount float64, gameID, sessionID, referenceID string) (*Transaction, error)
	Cancel(userID string, referenceID string) (*Transaction, error)
	GetTransactionHistory(userID string, limit, offset int) ([]*Transaction, error)
}
