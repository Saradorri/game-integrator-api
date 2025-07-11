package app

import (
	"github.com/saradorri/gameintegrator/internal/config"
	"github.com/saradorri/gameintegrator/internal/infrastructure/logger"
)

// InitLogger creates a new logger instance
func (a *application) InitLogger() *logger.Logger {
	return logger.NewLogger(config.GetEnvironment(), a.config.Log.Level)
}
