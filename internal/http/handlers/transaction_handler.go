package handlers

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/saradorri/gameintegrator/internal/domain"
)

// TransactionHandler handles HTTP requests for transaction operations
type TransactionHandler struct {
	txUseCase domain.TransactionUseCase
}

// NewTransactionHandler creates a new transaction handler
func NewTransactionHandler(txUseCase domain.TransactionUseCase) *TransactionHandler {
	return &TransactionHandler{
		txUseCase: txUseCase,
	}
}

// WithdrawRequest represents the withdrawal request body
type WithdrawRequest struct {
	Amount       float64 `json:"amount" binding:"required,gt=0" example:"100.50"`
	ProviderTxID string  `json:"provider_tx_id" binding:"required" example:"provider_12345"`
	Currency     string  `json:"currency" binding:"required" example:"USD"`
}

// DepositRequest represents the deposit request body
type DepositRequest struct {
	Amount                float64 `json:"amount" binding:"required,gt=0" example:"50.25"`
	ProviderTxID          string  `json:"provider_tx_id" binding:"required" example:"provider_67890"`
	ProviderWithdrawnTxID int64   `json:"provider_withdrawn_tx_id" binding:"required" example:"1"`
	Currency              string  `json:"currency" binding:"required" example:"USD"`
}

// TransactionResponse represents the transaction response body
type TransactionResponse struct {
	TransactionID         int64   `json:"transaction_id" example:"1"`
	UserID                int64   `json:"user_id" example:"123"`
	Type                  string  `json:"type" example:"withdraw"`
	Status                string  `json:"status" example:"pending"`
	Amount                float64 `json:"amount" example:"100.50"`
	Currency              string  `json:"currency" example:"USD"`
	ProviderTxID          string  `json:"provider_tx_id" example:"provider_12345"`
	ProviderWithdrawnTxID *int64  `json:"provider_withdrawn_tx_id,omitempty" example:"1"`
	OldBalance            float64 `json:"old_balance" example:"500.00"`
	NewBalance            float64 `json:"new_balance" example:"399.50"`
	CreatedAt             string  `json:"created_at" example:"2024-01-15T10:30:00Z"`
	UpdatedAt             string  `json:"updated_at" example:"2024-01-15T10:30:00Z"`
}

// getAuthenticatedUserID extracts and validates the authenticated user ID from the context
func (h *TransactionHandler) getAuthenticatedUserID(c *gin.Context) (int64, bool) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		h.logError(c, "User not authenticated", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return 0, false
	}

	userID, err := strconv.ParseInt(userIDStr.(string), 10, 64)
	if err != nil {
		h.logError(c, "Invalid user ID format", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return 0, false
	}

	return userID, true
}

// validateCurrency validates currency format
func (h *TransactionHandler) validateCurrency(currency string) bool {
	if len(currency) != 3 {
		return false
	}
	return strings.ToUpper(currency) == currency
}

// validateAmount validates amount precision and range
func (h *TransactionHandler) validateAmount(amount float64) bool {
	// Check precision (max 2 decimal places)
	amountStr := strconv.FormatFloat(amount, 'f', -1, 64)
	parts := strings.Split(amountStr, ".")
	if len(parts) > 1 && len(parts[1]) > 2 {
		return false
	}
	return true
}

// logError logs errors with context
func (h *TransactionHandler) logError(c *gin.Context, message string, err error) {
	userID, _ := c.Get("user_id")
	log.Printf("Transaction Handler Error - User: %v, Message: %s, Error: %v, Path: %s",
		userID, message, err, c.Request.URL.Path)
}

// handleUseCaseError handles use case errors with appropriate HTTP status codes
func (h *TransactionHandler) handleUseCaseError(c *gin.Context, err error) {
	h.logError(c, "UseCase error", err)

	errMsg := err.Error()

	switch {
	case strings.Contains(errMsg, "not found"):
		c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
	case strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "Access denied"):
		c.JSON(http.StatusForbidden, gin.H{"error": errMsg})
	case strings.Contains(errMsg, "insufficient balance") ||
		strings.Contains(errMsg, "already exists") ||
		strings.Contains(errMsg, "cannot be cancelled") ||
		strings.Contains(errMsg, "not pending"):
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
	case strings.Contains(errMsg, "currency") ||
		strings.Contains(errMsg, "amount") ||
		strings.Contains(errMsg, "required"):
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	}
}

// createTransactionResponse creates a standardized transaction response
func (h *TransactionHandler) createTransactionResponse(transaction *domain.Transaction) TransactionResponse {
	response := TransactionResponse{
		TransactionID: transaction.ID,
		UserID:        transaction.UserID,
		Type:          string(transaction.Type),
		Status:        string(transaction.Status),
		Amount:        transaction.Amount,
		Currency:      transaction.Currency,
		ProviderTxID:  transaction.ProviderTxID,
		OldBalance:    transaction.OldBalance,
		NewBalance:    transaction.NewBalance,
		CreatedAt:     transaction.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     transaction.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if transaction.ProviderWithdrawnTxID != nil {
		response.ProviderWithdrawnTxID = transaction.ProviderWithdrawnTxID
	}

	return response
}

// Withdraw handles withdrawal transactions
// @Summary Create withdrawal transaction
// @Description Create a withdrawal transaction for the authenticated user
// @Tags transactions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body WithdrawRequest true "Withdrawal details"
// @Success 200 {object} TransactionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /transactions/withdraw [post]
func (h *TransactionHandler) Withdraw(c *gin.Context) {
	userID, ok := h.getAuthenticatedUserID(c)
	if !ok {
		return
	}

	var req WithdrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logError(c, "Invalid withdraw request body", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format. Please check amount, provider_tx_id, and currency fields."})
		return
	}

	if !h.validateCurrency(req.Currency) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid currency format. Use 3-letter currency code (e.g., USD, EUR)"})
		return
	}

	if !h.validateAmount(req.Amount) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid amount. Amount must be positive, less than 1,000,000, and have maximum 2 decimal places."})
		return
	}

	if len(req.ProviderTxID) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider transaction ID too long. Maximum 64 characters allowed."})
		return
	}

	transaction, err := h.txUseCase.Withdraw(userID, req.Amount, req.ProviderTxID, req.Currency)
	if err != nil {
		h.handleUseCaseError(c, err)
		return
	}

	c.JSON(http.StatusOK, h.createTransactionResponse(transaction))
}

// Deposit handles deposit transactions
// @Summary Create deposit transaction
// @Description Create a deposit transaction for the authenticated user
// @Tags transactions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body DepositRequest true "Deposit details"
// @Success 200 {object} TransactionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /transactions/deposit [post]
func (h *TransactionHandler) Deposit(c *gin.Context) {
	userID, ok := h.getAuthenticatedUserID(c)
	if !ok {
		return
	}

	var req DepositRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logError(c, "Invalid deposit request body", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format. Please check amount, provider_tx_id, provider_withdrawn_tx_id, and currency fields."})
		return
	}

	if !h.validateCurrency(req.Currency) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid currency format. Use 3-letter currency code (e.g., USD, EUR)"})
		return
	}

	if !h.validateAmount(req.Amount) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid amount. Amount must be positive, less than 1,000,000, and have maximum 2 decimal places."})
		return
	}

	if len(req.ProviderTxID) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider transaction ID too long. Maximum 64 characters allowed."})
		return
	}

	if req.ProviderWithdrawnTxID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider withdrawn transaction ID must be positive"})
		return
	}

	transaction, err := h.txUseCase.Deposit(userID, req.Amount, req.ProviderTxID, req.ProviderWithdrawnTxID, req.Currency)
	if err != nil {
		h.handleUseCaseError(c, err)
		return
	}

	c.JSON(http.StatusOK, h.createTransactionResponse(transaction))
}

// Cancel handles transaction cancellation
// @Summary Cancel transaction
// @Description Cancel a pending transaction for the authenticated user
// @Tags transactions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param provider_tx_id path string true "Provider transaction ID" example:"provider_12345"
// @Success 200 {object} TransactionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /transactions/cancel/{provider_tx_id} [post]
func (h *TransactionHandler) Cancel(c *gin.Context) {
	userID, ok := h.getAuthenticatedUserID(c)
	if !ok {
		return
	}

	providerTxID := c.Param("provider_tx_id")
	if providerTxID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider transaction ID is required"})
		return
	}

	if len(providerTxID) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider transaction ID too long. Maximum 64 characters allowed."})
		return
	}

	transaction, err := h.txUseCase.Cancel(userID, providerTxID)
	if err != nil {
		h.handleUseCaseError(c, err)
		return
	}

	c.JSON(http.StatusOK, h.createTransactionResponse(transaction))
}
