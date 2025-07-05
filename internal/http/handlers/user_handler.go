package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/infrastructure/auth"
)

// UserHandler handles HTTP requests for user operations
type UserHandler struct {
	userUseCase domain.UserUseCase
	jwtService  auth.JWTService
}

// NewUserHandler creates a new user handler
func NewUserHandler(userUseCase domain.UserUseCase, jwtService auth.JWTService) *UserHandler {
	return &UserHandler{
		userUseCase: userUseCase,
		jwtService:  jwtService,
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
	Username string  `json:"username" example:"user1"`
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
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	token, err := h.userUseCase.Authenticate(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	claims, err := h.jwtService.ValidateToken(token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process token"})
		return
	}

	userID, err := strconv.ParseInt(claims.UserID, 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID in token"})
		return
	}

	user, err := h.userUseCase.GetUserInfo(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user information"})
		return
	}

	response := LoginResponse{
		Token: token,
		User: UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Balance:  user.Balance,
			Currency: user.Currency,
		},
	}

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
// @Failure 401 {object} ErrorResponse
// @Router /users/me [get]
func (h *UserHandler) GetUserInfo(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userID, err := strconv.ParseInt(userIDStr.(string), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := h.userUseCase.GetUserInfo(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user information"})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	response := UserInfo{
		ID:       user.ID,
		Username: user.Username,
		Balance:  user.Balance,
		Currency: user.Currency,
	}

	c.JSON(http.StatusOK, response)
}

// ErrorResponse represents error response
type ErrorResponse struct {
	Error string `json:"error" example:"Invalid request"`
}
