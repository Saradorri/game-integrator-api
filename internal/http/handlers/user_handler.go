package handlers

import (
	"log"
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
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid request format", 400, err))
		return
	}

	token, err := h.userUseCase.Authenticate(req.Username, req.Password)
	if err != nil {
		log.Printf("ERROR - Action: login, Username: %s, Error: %v", req.Username, err)
		c.JSON(http.StatusUnauthorized, err)
		return
	}

	claims, err := h.jwtService.ValidateToken(token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.NewInternalError("Failed to process token", err))
		return
	}

	userID, err := strconv.ParseInt(claims.UserID, 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.NewInternalError("Invalid user ID in token", err))
		return
	}

	user, err := h.userUseCase.GetUserInfo(userID)
	if err != nil {
		log.Printf("ERROR - UserID: %d, Action: get_user_info, Error: %v", userID, err)
		c.JSON(http.StatusInternalServerError, err)
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
// @Failure 401 {object} domain.ErrorResponse
// @Router /users/me [get]
func (h *UserHandler) GetUserInfo(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, domain.NewUnauthorizedError("User not authenticated"))
		return
	}

	userID, err := strconv.ParseInt(userIDStr.(string), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid user ID format", 400, err))
		return
	}

	user, err := h.userUseCase.GetUserInfo(userID)
	if err != nil {
		log.Printf("ERROR - UserID: %d, Action: get_user_info, Error: %v", userID, err)
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, domain.NewNotFoundError("user"))
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
