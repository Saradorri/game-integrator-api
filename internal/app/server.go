package app

import (
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/http"
	"github.com/saradorri/gameintegrator/internal/infrastructure/auth"
)

// InitHTTPServer initializes the HTTP server with all dependencies
func (a *application) InitHTTPServer(
	userUseCase domain.UserUseCase,
	jwtService auth.JWTService,
) *http.Server {
	port := a.config.Server.Port
	if port == "" {
		port = "8080" // default port
	}

	return http.NewServer(userUseCase, jwtService, port)
}
