package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// JSONB is a type for handle JSONB field that GORM can automatically marshal/unmarshal JSONB fields.
type JSONB map[string]interface{}

// Scan implements the sql.Scanner interface
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal JSONB value: %v", value)
	}
	return json.Unmarshal(bytes, j)
}

// Value implements the driver.Valuer interface
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// OutboxEvent represents an event stored in the outbox
type OutboxEvent struct {
	ID          string     `json:"id" gorm:"primaryKey;column:id;type:varchar(64)"`
	Type        string     `json:"type" gorm:"type:varchar(64);not null"`
	Data        JSONB      `json:"data" gorm:"type:jsonb"`
	Status      string     `json:"status" gorm:"type:varchar(16);not null;default:'PENDING'"`
	CreatedAt   time.Time  `json:"created_at" gorm:"not null"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
	Error       *string    `json:"error,omitempty"`
	RetryCount  int        `json:"retry_count" gorm:"default:0"`
}

// TableName specifies the table name for OutboxEvent
func (o OutboxEvent) TableName() string {
	return "outbox_events"
}

// OutboxRepository defines the interface for outbox persistence
type OutboxRepository interface {
	Save(event *OutboxEvent) error
	GetPendingEvents(limit int) ([]*OutboxEvent, error)
	MarkAsProcessed(eventID string) error
	MarkAsFailed(eventID string, errMsg string) error
	IncrementRetryCount(eventID string) error
}

// OutboxProcessor defines the interface for processing outbox events
type OutboxProcessor interface {
	ProcessEvents() error
	ProcessEvent(event *OutboxEvent) error
	StartBackgroundProcessing()
	StopBackgroundProcessing()
}

const EventTypeWithdrawRevert = "WITHDRAW_REVERT"

// Event statuses
const (
	EventStatusPending   = "PENDING"
	EventStatusProcessed = "PROCESSED"
	EventStatusFailed    = "FAILED"
)
