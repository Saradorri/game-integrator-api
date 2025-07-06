package app

import (
	"github.com/saradorri/gameintegrator/internal/http/middleware"
	"log"
	"os"
)

func (a *application) InitErrorHandler() *middleware.ErrorHandler {
	logger := log.New(os.Stdout, "[ERROR] ", log.LstdFlags|log.Lshortfile)
	return middleware.NewErrorHandler(logger)
}
