package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/infrastructure/logger"
	"go.uber.org/zap"
)

// TransactionHandler handles HTTP requests for transaction operations
type TransactionHandler struct {
	txUseCase domain.TransactionUseCase
	logger    *logger.Logger
}

// NewTransactionHandler creates a new transaction handler
func NewTransactionHandler(txUseCase domain.TransactionUseCase, logger *logger.Logger) *TransactionHandler {
	return &TransactionHandler{
		txUseCase: txUseCase,
		logger:    logger,
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
	Amount                *float64 `json:"amount" binding:"required" example:"50.25"`
	ProviderTxID          string   `json:"provider_tx_id" binding:"required" example:"provider_67890"`
	ProviderWithdrawnTxID int64    `json:"provider_withdrawn_tx_id" binding:"required" example:"1"`
	Currency              string   `json:"currency" binding:"required" example:"USD"`
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
		c.JSON(http.StatusUnauthorized, domain.NewUnauthorizedError("User not authenticated"))
		return 0, false
	}

	userID, err := strconv.ParseInt(userIDStr.(string), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid user ID format", 400, err))
		return 0, false
	}

	return userID, true
}

// validateCurrency validates currency format
func (h *TransactionHandler) validateCurrency(currency string) bool {
	return len(currency) == 3 && strings.ToUpper(currency) == currency
}

// validateAmount validates amount precision
func (h *TransactionHandler) validateAmount(amount float64) bool {
	amountStr := strconv.FormatFloat(amount, 'f', -1, 64)
	parts := strings.Split(amountStr, ".")
	return len(parts) <= 1 || len(parts[1]) <= 2
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
// @Failure 400 {object} domain.ErrorResponse
// @Failure 401 {object} domain.ErrorResponse
// @Failure 403 {object} domain.ErrorResponse
// @Router /transactions/withdraw [post]
func (h *TransactionHandler) Withdraw(c *gin.Context) {
	userID, ok := h.getAuthenticatedUserID(c)
	if !ok {
		return
	}

	var req WithdrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid format", 400, err))
		return
	}

	if !h.validateCurrency(req.Currency) {
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid currency format", 400, nil))
		return
	}

	if !h.validateAmount(req.Amount) {
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeInvalidPrecision, "Invalid amount precision or range", 400, nil))
		return
	}

	if len(req.ProviderTxID) > 64 {
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeInvalidRange, "Provider transaction ID too long", 400, nil))
		return
	}

	transaction, err := h.txUseCase.Withdraw(userID, req.Amount, req.ProviderTxID, req.Currency)
	if err != nil {
		h.logger.Error("Withdraw failed", zap.Int64("user_id", userID), zap.Float64("amount", req.Amount), zap.String("currency", req.Currency), zap.String("provider_tx_id", req.ProviderTxID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, err)
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
// @Failure 400 {object} domain.ErrorResponse
// @Failure 401 {object} domain.ErrorResponse
// @Failure 403 {object} domain.ErrorResponse
// @Router /transactions/deposit [post]
func (h *TransactionHandler) Deposit(c *gin.Context) {
	h.logger.Info("Deposit handler called")
	userID, ok := h.getAuthenticatedUserID(c)
	if !ok {
		h.logger.Error("Failed to get authenticated user ID")
		return
	}
	h.logger.Info("User authenticated", zap.Int64("userID", userID))

	var req DepositRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("JSON binding failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid format", 400, err))
		return
	}

	if req.Amount == nil {
		h.logger.Error("Amount validation failed - field is nil")
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeInvalidFormat, "Amount field is required", 400, nil))
		return
	}

	if *req.Amount < 0 {
		h.logger.Error("Amount validation failed - negative value", zap.Float64("amount", *req.Amount))
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeInvalidFormat, "Amount must be non-negative", 400, nil))
		return
	}

	amount := *req.Amount
	h.logger.Info("Request bound successfully", zap.Float64("amount", amount), zap.String("currency", req.Currency), zap.String("providerTxID", req.ProviderTxID), zap.Int64("providerWithdrawnTxID", req.ProviderWithdrawnTxID))

	if !h.validateCurrency(req.Currency) {
		h.logger.Error("Currency validation failed", zap.String("currency", req.Currency))
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeInvalidFormat, "Invalid currency format", 400, nil))
		return
	}
	h.logger.Info("Currency validation passed")

	if !h.validateAmount(amount) {
		h.logger.Error("Amount validation failed", zap.Float64("amount", amount))
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeInvalidPrecision, "Invalid amount precision or range", 400, nil))
		return
	}
	h.logger.Info("Amount validation passed")

	if len(req.ProviderTxID) > 64 {
		h.logger.Error("ProviderTxID too long", zap.String("providerTxID", req.ProviderTxID))
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeInvalidRange, "Provider transaction ID too long", 400, nil))
		return
	}
	h.logger.Info("ProviderTxID validation passed")

	if req.ProviderWithdrawnTxID <= 0 {
		h.logger.Error("ProviderWithdrawnTxID validation failed", zap.Int64("providerWithdrawnTxID", req.ProviderWithdrawnTxID))
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeInvalidRange, "Provider withdrawn transaction ID must be positive", 400, nil))
		return
	}
	h.logger.Info("ProviderWithdrawnTxID validation passed")

	h.logger.Info("All validations passed, calling usecase")
	transaction, err := h.txUseCase.Deposit(userID, amount, req.ProviderTxID, req.ProviderWithdrawnTxID, req.Currency)
	if err != nil {
		h.logger.Error("Deposit failed", zap.Int64("user_id", userID), zap.Float64("amount", amount), zap.String("currency", req.Currency), zap.String("provider_tx_id", req.ProviderTxID), zap.Int64("provider_withdrawn_tx_id", req.ProviderWithdrawnTxID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	h.logger.Info("Deposit successful", zap.Int64("transactionID", transaction.ID))
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
// @Failure 400 {object} domain.ErrorResponse
// @Failure 401 {object} domain.ErrorResponse
// @Failure 403 {object} domain.ErrorResponse
// @Failure 404 {object} domain.ErrorResponse
// @Router /transactions/cancel/{provider_tx_id} [post]
func (h *TransactionHandler) Cancel(c *gin.Context) {
	userID, ok := h.getAuthenticatedUserID(c)
	if !ok {
		return
	}

	providerTxID := c.Param("provider_tx_id")
	if providerTxID == "" {
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeRequiredField, "Provider transaction ID required", 400, nil))
		return
	}

	if len(providerTxID) > 64 {
		c.JSON(http.StatusBadRequest, domain.NewAppError(domain.ErrCodeInvalidRange, "Provider transaction ID too long", 400, nil))
		return
	}

	transaction, err := h.txUseCase.Cancel(userID, providerTxID)
	if err != nil {
		h.logger.Error("Cancel transaction failed", zap.Int64("user_id", userID), zap.String("provider_tx_id", providerTxID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, h.createTransactionResponse(transaction))
}
