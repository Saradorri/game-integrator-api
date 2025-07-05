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
	Amount       float64 `json:"amount" binding:"required,gt=0"`
	ProviderTxID string  `json:"provider_tx_id" binding:"required"`
	Currency     string  `json:"currency" binding:"required"`
}

// DepositRequest represents the deposit request body
type DepositRequest struct {
	Amount                float64 `json:"amount" binding:"required,gt=0"`
	ProviderTxID          string  `json:"provider_tx_id" binding:"required"`
	ProviderWithdrawnTxID int64   `json:"provider_withdrawn_tx_id" binding:"required"`
	Currency              string  `json:"currency" binding:"required"`
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
func (h *TransactionHandler) createTransactionResponse(transaction *domain.Transaction) gin.H {
	response := gin.H{
		"transaction_id": transaction.ID,
		"user_id":        transaction.UserID,
		"type":           string(transaction.Type),
		"status":         string(transaction.Status),
		"amount":         transaction.Amount,
		"currency":       transaction.Currency,
		"provider_tx_id": transaction.ProviderTxID,
		"old_balance":    transaction.OldBalance,
		"new_balance":    transaction.NewBalance,
		"created_at":     transaction.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at":     transaction.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if transaction.ProviderWithdrawnTxID != nil {
		response["provider_withdrawn_tx_id"] = *transaction.ProviderWithdrawnTxID
	}

	return response
}

// Withdraw handles withdrawal transactions
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
