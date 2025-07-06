package http

import (
	"fmt"
	"net/http"
	"time"

	"github.com/saradorri/gameintegrator/internal/infrastructure/auth"

	"github.com/gin-gonic/gin"
	"github.com/saradorri/gameintegrator/internal/http/handlers"
	"github.com/saradorri/gameintegrator/internal/http/middleware"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Server represents the HTTP server
type Server struct {
	router             *gin.Engine
	jwtService         auth.JWTService
	userHandler        *handlers.UserHandler
	transactionHandler *handlers.TransactionHandler
	errorHandler       *middleware.ErrorHandler
	port               string
}

// NewServer creates a new HTTP server
func NewServer(
	jwtService auth.JWTService,
	userHandler *handlers.UserHandler,
	transactionHandler *handlers.TransactionHandler,
	errorHandler *middleware.ErrorHandler,
	port string,
) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	router.Use(errorHandler.RequestIDMiddleware())
	router.Use(errorHandler.TimeoutMiddleware(30 * time.Second))
	router.Use(errorHandler.ErrorHandlerMiddleware())
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	server := &Server{
		router:             router,
		jwtService:         jwtService,
		userHandler:        userHandler,
		transactionHandler: transactionHandler,
		errorHandler:       errorHandler,
		port:               port,
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures all the routes
func (s *Server) setupRoutes() {
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := s.router.Group("/api/v1")
	{
		authRoutes := v1.Group("/auth")
		{
			authRoutes.POST("/login", s.userHandler.Login)
		}

		protected := v1.Group("/")
		protected.Use(middleware.JWTMiddleware(s.jwtService))
		{
			userRoutes := protected.Group("/users")
			{
				userRoutes.GET("/me", s.userHandler.GetUserInfo)
			}

			transactionRoutes := protected.Group("/transactions")
			{
				transactionRoutes.POST("/withdraw", s.transactionHandler.Withdraw)
				transactionRoutes.POST("/deposit", s.transactionHandler.Deposit)
				transactionRoutes.POST("/cancel/:provider_tx_id", s.transactionHandler.Cancel)
			}
		}
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%s", s.port)
	return s.router.Run(addr)
}
