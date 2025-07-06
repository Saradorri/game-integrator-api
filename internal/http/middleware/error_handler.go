package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/saradorri/gameintegrator/internal/domain"
)

// ErrorHandler provides centralized error handling
type ErrorHandler struct {
	logger *log.Logger
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(logger *log.Logger) *ErrorHandler {
	return &ErrorHandler{
		logger: logger,
	}
}

// ErrorHandlerMiddleware provides centralized error handling for all requests
func (h *ErrorHandler) ErrorHandlerMiddleware() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		h.handlePanic(c, recovered)
	})
}

// handlePanic handles panic recovery
func (h *ErrorHandler) handlePanic(c *gin.Context, recovered interface{}) {
	requestID := h.getRequestID(c)
	userID := h.getUserID(c)

	h.logger.Printf("PANIC - RequestID: %s, UserID: %s, Path: %s, Method: %s, Error: %v, Stack: %s",
		requestID, userID, c.Request.URL.Path, c.Request.Method, recovered, string(debug.Stack()))

	err := domain.NewInternalError("Internal server error", fmt.Errorf("panic: %v", recovered))
	err.RequestID = requestID
	err.UserID = userID
	err.Path = c.Request.URL.Path
	err.Method = c.Request.Method

	c.JSON(http.StatusInternalServerError, domain.NewErrorResponse(err))
}

// getRequestID gets or generates a request ID
func (h *ErrorHandler) getRequestID(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		return requestID.(string)
	}
	return h.generateRequestID()
}

// getUserID gets the user ID from context
func (h *ErrorHandler) getUserID(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		return userID.(string)
	}
	return ""
}

// RequestIDMiddleware adds a unique request ID to each request
func (h *ErrorHandler) RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = h.generateRequestID()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// TimeoutMiddleware adds timeout context to requests
func (h *ErrorHandler) TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		done := make(chan struct{})
		go func() {
			c.Next()
			done <- struct{}{}
		}()

		select {
		case <-done:
			return
		case <-ctx.Done():
			requestID := h.getRequestID(c)
			userID := h.getUserID(c)

			err := domain.NewAppError("TIMEOUT", "Request timeout", http.StatusRequestTimeout, ctx.Err())
			err.RequestID = requestID
			err.UserID = userID
			err.Path = c.Request.URL.Path
			err.Method = c.Request.Method

			h.logger.Printf("TIMEOUT - RequestID: %s, UserID: %s, Path: %s, Method: %s",
				requestID, userID, c.Request.URL.Path, c.Request.Method)

			c.Abort()
			c.JSON(http.StatusRequestTimeout, domain.NewErrorResponse(err))
			return
		}
	}
}

// generateRequestID generates a unique request ID
func (h *ErrorHandler) generateRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}
