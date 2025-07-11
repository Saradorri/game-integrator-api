package lock

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

type UserLockManager struct {
	locks  sync.Map // map[int64]*sync.Mutex
	logger *zap.Logger
}

func NewUserLockManager() *UserLockManager {
	logger, _ := zap.NewProduction()
	logger.Info("UserLockManager initialized")
	return &UserLockManager{
		logger: logger,
	}
}

// Lock acquires a lock for the given userID with timeout
func (m *UserLockManager) Lock(ctx context.Context, userID int64) error {
	m.logger.Info("Attempting to acquire lock", zap.Int64("userID", userID))
	mu := m.getOrCreateMutex(userID)

	// acquire lock with context timeout
	lockChan := make(chan struct{})
	go func() {
		m.logger.Debug("Waiting for lock to become available", zap.Int64("userID", userID))
		mu.Lock()
		m.logger.Debug("Lock acquired", zap.Int64("userID", userID))
		close(lockChan)
	}()

	select {
	case <-lockChan:
		m.logger.Info("Successfully acquired lock", zap.Int64("userID", userID))
		return nil
	case <-ctx.Done():
		m.logger.Error("Failed to acquire lock: context cancelled", zap.Int64("userID", userID), zap.Error(ctx.Err()))
		return fmt.Errorf("failed to acquire lock for user %d: %w", userID, ctx.Err())
	case <-time.After(5 * time.Second):
		m.logger.Error("Failed to acquire lock: timeout", zap.Int64("userID", userID), zap.Duration("timeout", 5*time.Second))
		return fmt.Errorf("failed to acquire lock for user %d: timeout", userID)
	}
}

// Unlock releases the lock for the given userID
func (m *UserLockManager) Unlock(userID int64) {
	m.logger.Info("Attempting to release lock", zap.Int64("userID", userID))
	muInterface, ok := m.locks.Load(userID)
	if !ok {
		m.logger.Warn("No lock found during unlock", zap.Int64("userID", userID))
		return
	}
	mu := muInterface.(*sync.Mutex)
	mu.Unlock()
	m.logger.Info("Successfully released lock", zap.Int64("userID", userID))
}

// TryLock attempts to acquire a lock without blocking
func (m *UserLockManager) TryLock(userID int64) bool {
	m.logger.Debug("Attempting to try-lock", zap.Int64("userID", userID))
	mu := m.getOrCreateMutex(userID)
	acquired := mu.TryLock()
	if acquired {
		m.logger.Info("Successfully acquired try-lock", zap.Int64("userID", userID))
	} else {
		m.logger.Debug("Failed to acquire try-lock: lock is busy", zap.Int64("userID", userID))
	}
	return acquired
}

func (m *UserLockManager) getOrCreateMutex(userID int64) *sync.Mutex {
	mu, ok := m.locks.Load(userID)
	if ok {
		m.logger.Debug("Reusing existing mutex", zap.Int64("userID", userID))
		return mu.(*sync.Mutex)
	}

	m.logger.Debug("Creating new mutex", zap.Int64("userID", userID))
	newMu := &sync.Mutex{}
	actual, _ := m.locks.LoadOrStore(userID, newMu)
	return actual.(*sync.Mutex)
}
