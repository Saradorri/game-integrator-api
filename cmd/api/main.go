// Package main Game Integrator API
//
// Game Integrator is a financial transaction management system that facilitates third-party casino
// games on our platform. This new service is crucial for handling all financial transactions
// related to gameplay, with two key responsibilities:
//
//  1. Managing user balances through interactions with an existing, somewhat unreliable, backend service.
//
//  2. Creating and updating bets dynamically based on endpoint calls.
//
//     Schemes: http, https
//     Host: localhost:8080
//     BasePath: /api/v1
//     Version: 1.0.0
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
//     Security:
//     - bearer
package main

import (
	"context"

	_ "github.com/saradorri/gameintegrator/docs"
	"github.com/saradorri/gameintegrator/internal/app"
)

// @title Game Integrator API Service
// @version 1.0
// @description Game Integrator is a financial transaction management system that facilitates third-party casino games on our platform.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
func main() {
	ctx := context.Background()
	application := app.NewApplication(ctx)
	application.Setup()
}
