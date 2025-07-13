package outbox

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/infrastructure/logger"
	"go.uber.org/zap"
)

// Processor implements domain.OutboxProcessor
type Processor struct {
	outboxRepo    domain.OutboxRepository
	transactionUC domain.TransactionUseCase
	logger        *logger.Logger
	maxRetries    int

	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
	isRunning bool
}

// NewProcessor creates a new outbox processor
func NewProcessor(
	outboxRepo domain.OutboxRepository,
	transactionUC domain.TransactionUseCase,
	logger *logger.Logger,
) *Processor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Processor{
		outboxRepo:    outboxRepo,
		transactionUC: transactionUC,
		logger:        logger,
		maxRetries:    5,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// ProcessEvents processes all pending events
func (p *Processor) ProcessEvents() error {
	if err := p.checkCancellation(); err != nil {
		return err
	}

	events, err := p.outboxRepo.GetPendingEvents(100)
	if err != nil {
		p.logger.Error("Failed to get pending events", zap.Error(err))
		return err
	}

	for _, event := range events {
		select {
		case <-p.ctx.Done():
			return fmt.Errorf("processor cancelled")
		default:
		}

		if err := p.ProcessEvent(event); err != nil {
			p.logger.Error("Failed to process event",
				zap.String("eventID", event.ID),
				zap.String("eventType", event.Type),
				zap.Error(err))

			if event.RetryCount < p.maxRetries {
				if retryErr := p.outboxRepo.IncrementRetryCount(event.ID); retryErr != nil {
					p.logger.Error("Failed to increment retry count", zap.Error(retryErr))
				}
			} else {
				if failErr := p.outboxRepo.MarkAsFailed(event.ID, err.Error()); failErr != nil {
					p.logger.Error("Failed to mark event as failed", zap.Error(failErr))
				}
			}
		}
	}

	return nil
}

// ProcessEvent processes a single outbox event
func (p *Processor) ProcessEvent(event *domain.OutboxEvent) error {
	p.logger.Info("Processing outbox event",
		zap.String("eventID", event.ID),
		zap.String("eventType", event.Type))

	if event.Type == domain.EventTypeWithdrawRevert {
		return p.handleWithdrawRevert(event)
	}

	p.logger.Warn("Unknown event type",
		zap.String("eventID", event.ID),
		zap.String("eventType", event.Type))
	return fmt.Errorf("unknown event type: %s", event.Type)
}

// extractCommonData extracts common fields from event data
func (p *Processor) extractCommonData(event *domain.OutboxEvent) (int64, float64, string, error) {
	userID, ok := event.Data["user_id"].(float64)
	if !ok {
		return 0, 0, "", fmt.Errorf("invalid user_id in event data")
	}

	amount, ok := event.Data["amount"].(float64)
	if !ok {
		return 0, 0, "", fmt.Errorf("invalid amount in event data")
	}

	providerTxID, ok := event.Data["provider_tx_id"].(string)
	if !ok {
		return 0, 0, "", fmt.Errorf("invalid provider_tx_id in event data")
	}

	return int64(userID), amount, providerTxID, nil
}

// checkCancellation checks if the processor has been cancelled
func (p *Processor) checkCancellation() error {
	select {
	case <-p.ctx.Done():
		return fmt.Errorf("processor cancelled")
	default:
		return nil
	}
}

// handleWithdrawRevert handles withdraw revert events (revert by depositing back)
func (p *Processor) handleWithdrawRevert(event *domain.OutboxEvent) error {
	if err := p.checkCancellation(); err != nil {
		return err
	}

	userID, amount, providerTxID, err := p.extractCommonData(event)
	if err != nil {
		return err
	}

	// Handle amount = 0 (user lost bet) - skip wallet service call
	if amount == 0 {
		p.logger.Info("Withdraw revert amount is 0 (user lost bet), skipping wallet service call",
			zap.String("eventID", event.ID),
			zap.String("providerTxID", providerTxID))
		return p.outboxRepo.MarkAsProcessed(event.ID)
	}

	_, err = p.transactionUC.Revert(userID, providerTxID, amount, domain.TransactionTypeDeposit)
	if err != nil {
		p.logger.Error("Failed to revert withdrawal using transaction use case",
			zap.String("eventID", event.ID),
			zap.String("providerTxID", providerTxID),
			zap.Error(err))

		return fmt.Errorf("failed to revert withdrawal using transaction use case: %w", err)
	}

	p.logger.Info("Successfully reverted withdrawal using transaction use case",
		zap.String("eventID", event.ID),
		zap.String("providerTxID", providerTxID))

	return p.outboxRepo.MarkAsProcessed(event.ID)
}

// StartBackgroundProcessing starts the background processing loop
func (p *Processor) StartBackgroundProcessing() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning {
		p.logger.Warn("Outbox processor is already running")
		return
	}

	p.isRunning = true
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(5 * time.Second) // Process every 5 seconds
		defer ticker.Stop()

		p.logger.Info("Outbox background processing started")

		for {
			select {
			case <-p.ctx.Done():
				p.logger.Info("Outbox background processing stopped")
				return
			case <-ticker.C:
				if err := p.ProcessEvents(); err != nil {
					p.logger.Error("Background processing failed", zap.Error(err))
				}
			}
		}
	}()
}

// StopBackgroundProcessing stops the background processing loop
func (p *Processor) StopBackgroundProcessing() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning {
		p.logger.Warn("Outbox processor is not running")
		return
	}

	p.logger.Info("Stopping outbox background processing...")
	p.cancel()
	p.wg.Wait()
	p.isRunning = false
	p.logger.Info("Outbox background processing stopped")
}
