package app

import (
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/infrastructure/logger"
	"github.com/saradorri/gameintegrator/internal/infrastructure/outbox"
)

func (a *application) InitOutboxProcessor(
	outboxRepo domain.OutboxRepository,
	transactionUC domain.TransactionUseCase,
	logger *logger.Logger,
) domain.OutboxProcessor {
	return outbox.NewProcessor(outboxRepo, transactionUC, logger)
}
