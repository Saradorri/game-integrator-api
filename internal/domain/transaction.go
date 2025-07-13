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
	TransactionTypeRevert   TransactionType = "revert"
)

// TransactionStatus represents the status of a transaction
type TransactionStatus string

const (
	// TransactionStatusSyncing bet placement trying to sync transaction with wallet
	TransactionStatusSyncing TransactionStatus = "syncing"

	// TransactionStatusPending bet placement transaction successfully synced with wallet
	TransactionStatusPending TransactionStatus = "pending"

	// TransactionStatusCompleted a bet settlement successfully completed
	TransactionStatusCompleted TransactionStatus = "completed"

	// TransactionStatusFailed bet failed due to wallet service error or db error
	TransactionStatusFailed TransactionStatus = "failed"

	// TransactionStatusCancelled bet canceled by provider
	TransactionStatusCancelled TransactionStatus = "cancelled"
)

// Transaction represents a financial transaction in the system
type Transaction struct {
	ID                    int64             `json:"transaction_id" gorm:"primaryKey;column:id;type:bigint;autoIncrement"`
	UserID                int64             `json:"user_id" gorm:"index;not null;type:bigint"`
	Type                  TransactionType   `json:"type" gorm:"type:varchar(16);not null"`
	Status                TransactionStatus `json:"status" gorm:"type:varchar(16);not null;default:'pending'"`
	Amount                float64           `json:"amount" gorm:"type:numeric(20,2);not null"`
	Currency              string            `json:"currency" gorm:"type:varchar(8);not null"`
	ProviderTxID          string            `json:"provider_tx_id" gorm:"uniqueIndex;type:varchar(64);not null"` // For withdraw
	ProviderWithdrawnTxID *int64            `json:"provider_withdrawn_tx_id,omitempty" gorm:"type:bigint"`       // For deposit
	OldBalance            float64           `json:"old_balance" gorm:"type:numeric(20,2);not null"`
	NewBalance            float64           `json:"new_balance" gorm:"type:numeric(20,2);not null"`
	CreatedAt             time.Time         `json:"created_at" gorm:"not null"`
	UpdatedAt             time.Time         `json:"updated_at" gorm:"not null"`

	User User `json:"-" gorm:"foreignKey:UserID"`

	// Self-referencing relationship for provider_withdrawn_tx_id
	WithdrawnTransaction *Transaction `json:"-" gorm:"foreignKey:ProviderWithdrawnTxID"`
}

// TableName specifies the table name for Transaction
func (T Transaction) TableName() string {
	return "transactions"
}

// TransactionRepository defines the interface for transaction data
type TransactionRepository interface {
	Create(transaction *Transaction) error
	GetByID(id int64) (*Transaction, error)
	GetByIDForUpdate(id int64) (*Transaction, error)
	GetByProviderTxID(providerTxID string) (*Transaction, error)
	GetByProviderTxIDForUpdate(providerTxID string) (*Transaction, error)
	GetByUserID(userID int64, limit, offset int) ([]*Transaction, error)
	Update(transaction *Transaction) error
	UpdateStatus(id int64, status TransactionStatus) error
	GetPendingByUserID(userID int64) ([]*Transaction, error)
	GetByProviderWithdrawnTxID(providerWithdrawnTxID int64) (*Transaction, error)
	WithTransaction(tx *gorm.DB) TransactionRepository
}

// TransactionUseCase defines the interface for transaction business logic
type TransactionUseCase interface {
	Withdraw(userID int64, amount float64, providerTxID string, currency string) (*Transaction, error)
	Deposit(userID int64, amount float64, providerTxID string, providerWithdrawnTxID int64, currency string) (*Transaction, error)
	Cancel(userID int64, providerTxID string) (*Transaction, error)
	Revert(userID int64, providerTxID string, amount float64, txType TransactionType) (*Transaction, error)
}
