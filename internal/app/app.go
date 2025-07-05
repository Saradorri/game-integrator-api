package app

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/saradorri/gameintegrator/internal/config"
	"go.uber.org/fx"
)

// Application provides application level setup
type Application interface {
	Setup()
	GetContext() context.Context
}

// application represents context and configure file
type application struct {
	ctx    context.Context
	config *config.Config
}

// NewApplication creates a new application
func NewApplication(ctx context.Context) Application {
	return &application{ctx: ctx}
}

// GetContext returns application context
func (a *application) GetContext() context.Context {
	return a.ctx
}

// GetConfig returns the application configuration
func (a *application) GetConfig() *config.Config {
	return a.config
}

// Setup creates a new fx application with all modules
func (a *application) Setup() {
	fmt.Println("[x] Starting Game Integrator Service...")

	path := flag.String("e", "./config", "env file directory")
	flag.Parse()

	err := a.setupViper(*path)
	if err != nil {
		log.Panic(err.Error())
	}

	app := fx.New(
		fx.Provide(
			a.GetConfig,
			a.InitDatabase,
			a.InitRepository,
			a.InitWalletService,
			a.InitJWTService,
			a.InitUserUseCase,
		),
	)

	app.Run()
}
