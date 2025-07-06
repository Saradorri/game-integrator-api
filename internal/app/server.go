package app

import (
	"github.com/saradorri/gameintegrator/internal/http"
	"github.com/saradorri/gameintegrator/internal/http/handlers"
	"github.com/saradorri/gameintegrator/internal/http/middleware"
	"github.com/saradorri/gameintegrator/internal/infrastructure/auth"
)

// InitHTTPServer initializes the HTTP server with all dependencies
func (a *application) InitHTTPServer(
	userHandler *handlers.UserHandler,
	jwtService auth.JWTService,
	transactionHandler *handlers.TransactionHandler,
	errorHandler *middleware.ErrorHandler,
) *http.Server {
	port := a.config.Server.Port
	if port == "" {
		port = "8080" // default port
	}

	return http.NewServer(jwtService, userHandler, transactionHandler, errorHandler, port)
}
