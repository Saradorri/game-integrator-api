package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/infrastructure/auth"
	"github.com/saradorri/gameintegrator/internal/infrastructure/logger"
	"go.uber.org/zap"
)

// UserHandler handles HTTP requests for user operations
type UserHandler struct {
	userUseCase domain.UserUseCase
	walletSvc   domain.WalletService
	jwtService  auth.JWTService
	logger      *logger.Logger
}

// NewUserHandler creates a new user handler
func NewUserHandler(userUseCase domain.UserUseCase, walletSvc domain.WalletService, jwtService auth.JWTService, logger *logger.Logger) *UserHandler {
	return &UserHandler{
		userUseCase: userUseCase,
		walletSvc:   walletSvc,
		jwtService:  jwtService,
		logger:      logger,
	}
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"user1"`
	Password string `json:"password" binding:"required" example:"password123"`
}

// LoginResponse represents the login response body
type LoginResponse struct {
	Token string   `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User  UserInfo `json:"user"`
}

// UserInfo represents user information
type UserInfo struct {
	ID       int64   `json:"id" example:"123"`
	Username string  `json:"username" example:"john_doe"`
	Balance  float64 `json:"balance" example:"1000.50"`
	Currency string  `json:"currency" example:"USD"`
}

// Login handles user authentication
// @Summary User login
// @Description Authenticate user and return JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} domain.ErrorResponse
// @Failure 401 {object} domain.ErrorResponse
// @Router /auth/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	h.logger.Info("Processing login request",
		zap.String("client_ip", c.ClientIP()),
		zap.String("user_agent", c.GetHeader("User-Agent")))

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid login request format",
			zap.String("client_ip", c.ClientIP()),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid request format", 400, err))
		return
	}

	h.logger.Info("Authenticating user",
		zap.String("username", req.Username),
		zap.String("client_ip", c.ClientIP()))

	token, err := h.userUseCase.Authenticate(req.Username, req.Password)
	if err != nil {
		h.logger.Error("Login failed for username",
			zap.String("username", req.Username),
			zap.String("client_ip", c.ClientIP()),
			zap.Error(err))
		c.JSON(http.StatusUnauthorized, err)
		return
	}

	h.logger.Debug("Validating JWT token")

	claims, err := h.jwtService.ValidateToken(token)
	if err != nil {
		h.logger.Error("Failed to validate JWT token",
			zap.String("username", req.Username),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, domain.NewInternalError("Failed to process token", err))
		return
	}

	userID, err := strconv.ParseInt(claims.UserID, 10, 64)
	if err != nil {
		h.logger.Error("Invalid user ID in JWT token",
			zap.String("username", req.Username),
			zap.String("user_id_from_token", claims.UserID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, domain.NewInternalError("Invalid user ID in token", err))
		return
	}

	h.logger.Debug("Retrieving user information",
		zap.Int64("user_id", userID),
		zap.String("username", req.Username))

	user, err := h.userUseCase.GetUserInfo(userID)
	if err != nil {
		h.logger.Error("Failed to get user info for user_id",
			zap.Int64("user_id", userID),
			zap.String("username", req.Username),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	if user == nil {
		h.logger.Warn("User not found after authentication",
			zap.Int64("user_id", userID),
			zap.String("username", req.Username))
		c.JSON(http.StatusNotFound, domain.NewNotFoundError("user"))
		return
	}

	h.logger.Debug("Retrieving wallet balance",
		zap.Int64("user_id", userID),
		zap.String("username", req.Username))

	// Get balance from wallet service
	walletBalance, err := h.walletSvc.GetBalance(userID)
	if err != nil {
		h.logger.Error("Failed to get wallet balance for user_id",
			zap.Int64("user_id", userID),
			zap.String("username", req.Username),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to get wallet balance", 500, err))
		return
	}

	balance, err := strconv.ParseFloat(walletBalance.Balance, 64)
	if err != nil {
		h.logger.Error("Failed to parse balance for user_id",
			zap.Int64("user_id", userID),
			zap.String("username", req.Username),
			zap.String("wallet_balance", walletBalance.Balance),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid balance format", 400, err))
		return
	}

	response := LoginResponse{
		Token: token,
		User: UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Balance:  balance,
			Currency: user.Currency,
		},
	}

	h.logger.Info("Login successful",
		zap.Int64("user_id", user.ID),
		zap.String("username", user.Username),
		zap.String("currency", user.Currency),
		zap.Float64("balance", balance),
		zap.String("client_ip", c.ClientIP()))

	c.JSON(http.StatusOK, response)
}

// GetUserInfo handles getting user information
// @Summary Get user information
// @Description Get current user information from JWT token
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserInfo
// @Failure 401 {object} domain.ErrorResponse
// @Router /users/me [get]
func (h *UserHandler) GetUserInfo(c *gin.Context) {
	h.logger.Info("Processing get user info request",
		zap.String("client_ip", c.ClientIP()),
		zap.String("user_agent", c.GetHeader("User-Agent")))

	userIDStr, exists := c.Get("user_id")
	if !exists {
		h.logger.Warn("Get user info request without authentication",
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusUnauthorized, domain.NewUnauthorizedError("User not authenticated"))
		return
	}

	userID, err := strconv.ParseInt(userIDStr.(string), 10, 64)
	if err != nil {
		h.logger.Error("Invalid user ID format in context",
			zap.String("user_id_str", userIDStr.(string)),
			zap.String("client_ip", c.ClientIP()),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid user ID format", 400, err))
		return
	}

	h.logger.Info("Retrieving user information",
		zap.Int64("user_id", userID),
		zap.String("client_ip", c.ClientIP()))

	user, err := h.userUseCase.GetUserInfo(userID)
	if err != nil {
		h.logger.Error("Failed to get user info for user_id",
			zap.Int64("user_id", userID),
			zap.String("client_ip", c.ClientIP()),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	if user == nil {
		h.logger.Warn("User not found during get user info",
			zap.Int64("user_id", userID),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusNotFound, domain.NewNotFoundError("user"))
		return
	}

	h.logger.Debug("Retrieving wallet balance for user info",
		zap.Int64("user_id", userID),
		zap.String("username", user.Username))

	// Get balance from wallet service
	walletBalance, err := h.walletSvc.GetBalance(userID)
	if err != nil {
		h.logger.Error("Failed to get wallet balance for user_id",
			zap.Int64("user_id", userID),
			zap.String("username", user.Username),
			zap.String("client_ip", c.ClientIP()),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, domain.NewAppError(domain.ErrCodeWalletServiceError, "Failed to get wallet balance", 500, err))
		return
	}

	balance, err := strconv.ParseFloat(walletBalance.Balance, 64)
	if err != nil {
		h.logger.Error("Failed to parse balance for user_id",
			zap.Int64("user_id", userID),
			zap.String("username", user.Username),
			zap.String("wallet_balance", walletBalance.Balance),
			zap.String("client_ip", c.ClientIP()),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid balance format", 400, err))
		return
	}

	response := UserInfo{
		ID:       user.ID,
		Username: user.Username,
		Balance:  balance,
		Currency: user.Currency,
	}

	h.logger.Info("User info retrieved successfully",
		zap.Int64("user_id", user.ID),
		zap.String("username", user.Username),
		zap.String("currency", user.Currency),
		zap.Float64("balance", balance),
		zap.String("client_ip", c.ClientIP()))

	c.JSON(http.StatusOK, response)
}
