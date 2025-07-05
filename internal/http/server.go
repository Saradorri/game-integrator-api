package http

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/http/handlers"
	"github.com/saradorri/gameintegrator/internal/http/middleware"
	"github.com/saradorri/gameintegrator/internal/infrastructure/auth"
)

// Server represents the HTTP server
type Server struct {
	router             *gin.Engine
	userHandler        *handlers.UserHandler
	transactionHandler *handlers.TransactionHandler
	jwtService         auth.JWTService
	port               string
}

// NewServer creates a new HTTP server
func NewServer(
	userUseCase domain.UserUseCase,
	transactionUseCase domain.TransactionUseCase,
	jwtService auth.JWTService,
	port string,
) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	userHandler := handlers.NewUserHandler(userUseCase, jwtService)
	transactionHandler := handlers.NewTransactionHandler(transactionUseCase)

	server := &Server{
		router:             router,
		userHandler:        userHandler,
		transactionHandler: transactionHandler,
		jwtService:         jwtService,
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
