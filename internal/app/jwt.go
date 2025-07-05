package app

import (
	"github.com/saradorri/gameintegrator/internal/config"
	"github.com/saradorri/gameintegrator/internal/infrastructure/auth"
)

func (a *application) InitJWTService() auth.JWTService {
	cfg := &config.JWTConfig{
		Secret: a.config.JWT.Secret,
		Expiry: a.config.JWT.Expiry,
	}
	return auth.NewJWTService(cfg)
}
