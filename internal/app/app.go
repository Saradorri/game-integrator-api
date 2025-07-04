package app

import (
	"context"
	"fmt"
	"go.uber.org/fx"
)

type Application interface {
	Setup()
	GetContext() context.Context
}

type application struct {
	ctx context.Context
}

func NewApplication(ctx context.Context) Application {
	return &application{ctx: ctx}
}

func (a *application) GetContext() context.Context {
	return a.ctx
}

func (a *application) Setup() {
	app := fx.New()
	fmt.Println("[x] Starting Game Integrator Service...")
	app.Run()
}
