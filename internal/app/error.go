package app

import (
	"github.com/saradorri/gameintegrator/internal/config"
	"github.com/saradorri/gameintegrator/internal/http/middleware"
	"github.com/saradorri/gameintegrator/internal/infrastructure/logger"
)

func (a *application) InitErrorHandler() *middleware.ErrorHandler {
	log := logger.NewLogger(config.GetEnvironment(), a.config.Log.Level)
	return middleware.NewErrorHandler(log)
}
