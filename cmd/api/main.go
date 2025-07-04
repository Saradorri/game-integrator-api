package main

import (
	"context"
	"github.com/saradorri/gameintegrator/internal/app"
)

func main() {
	application := app.NewApplication(context.Background())
	application.Setup()
}
