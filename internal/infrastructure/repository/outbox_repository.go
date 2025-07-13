package repository

import (
	"time"

	"github.com/saradorri/gameintegrator/internal/domain"
	"gorm.io/gorm"
)

// OutboxRepository implements domain.OutboxRepository
type OutboxRepository struct {
	db *gorm.DB
}

// NewOutboxRepository creates a new outbox repository
func NewOutboxRepository(db *gorm.DB) domain.OutboxRepository {
	return &OutboxRepository{
		db: db,
	}
}

// Save saves an outbox event to the database
func (r *OutboxRepository) Save(event *domain.OutboxEvent) error {
	return r.db.Create(event).Error
}

// GetPendingEvents retrieves pending events from the database
func (r *OutboxRepository) GetPendingEvents(limit int) ([]*domain.OutboxEvent, error) {
	var events []*domain.OutboxEvent
	err := r.db.Where("status = ?", domain.EventStatusPending).
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

// MarkAsProcessed marks an event as processed
func (r *OutboxRepository) MarkAsProcessed(eventID string) error {
	now := time.Now()
	return r.db.Model(&domain.OutboxEvent{}).
		Where("id = ?", eventID).
		Updates(map[string]interface{}{
			"status":       domain.EventStatusProcessed,
			"processed_at": &now,
		}).Error
}

// MarkAsFailed marks an event as failed
func (r *OutboxRepository) MarkAsFailed(eventID string, errMsg string) error {
	return r.db.Model(&domain.OutboxEvent{}).
		Where("id = ?", eventID).
		Updates(map[string]interface{}{
			"status": domain.EventStatusFailed,
			"error":  &errMsg,
		}).Error
}

// IncrementRetryCount increments the retry count for an event
func (r *OutboxRepository) IncrementRetryCount(eventID string) error {
	var event domain.OutboxEvent
	if err := r.db.Where("id = ?", eventID).First(&event).Error; err != nil {
		return err
	}

	newRetryCount := event.RetryCount + 1

	return r.db.Model(&domain.OutboxEvent{}).
		Where("id = ?", eventID).
		Update("retry_count", newRetryCount).Error
}
