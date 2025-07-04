package app

import (
	"context"
	"flag"
	"fmt"
	"github.com/saradorri/gameintegrator/internal/config"
	"go.uber.org/fx"
	"log"
)

type Application interface {
	Setup()
	GetContext() context.Context
}

type application struct {
	ctx    context.Context
	config *config.Config
}

func NewApplication(ctx context.Context) Application {
	return &application{ctx: ctx}
}

func (a *application) GetContext() context.Context {
	return a.ctx
}

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
			a.InitDatabase,
		))

	app.Run()
}
