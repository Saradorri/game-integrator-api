package domain

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

// AppError represents an application error
type AppError struct {
	Code       string    `json:"code"`
	Message    string    `json:"message"`
	Details    string    `json:"details,omitempty"`
	HTTPStatus int       `json:"-"`
	Timestamp  time.Time `json:"timestamp"`
	RequestID  string    `json:"request_id,omitempty"`
	UserID     string    `json:"user_id,omitempty"`
	Path       string    `json:"path,omitempty"`
	Method     string    `json:"method,omitempty"`
	Err        error     `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", e.Message, e.Err.Error())
	}
	return e.Message
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError creates a new application error
func NewAppError(code, message string, httpStatus int, err error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Timestamp:  time.Now(),
		Err:        err,
	}
}

// NewValidationError creates a validation error
func NewValidationError(field, message string) *AppError {
	return NewAppError(
		"VALIDATION_ERROR",
		fmt.Sprintf("Validation failed for field '%s': %s", field, message),
		http.StatusBadRequest,
		nil,
	)
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource string) *AppError {
	return NewAppError(
		"NOT_FOUND",
		fmt.Sprintf("%s not found", resource),
		http.StatusNotFound,
		nil,
	)
}

// NewUnauthorizedError creates an unauthorized error
func NewUnauthorizedError(message string) *AppError {
	if message == "" {
		message = "Unauthorized access"
	}
	return NewAppError(
		"UNAUTHORIZED",
		message,
		http.StatusUnauthorized,
		nil,
	)
}

// NewForbiddenError creates a forbidden error
func NewForbiddenError(message string) *AppError {
	if message == "" {
		message = "Access forbidden"
	}
	return NewAppError(
		"FORBIDDEN",
		message,
		http.StatusForbidden,
		nil,
	)
}

// NewConflictError creates a conflict error
func NewConflictError(message string) *AppError {
	return NewAppError(
		"CONFLICT",
		message,
		http.StatusConflict,
		nil,
	)
}

// NewInternalError creates an internal server error
func NewInternalError(message string, err error) *AppError {
	if message == "" {
		message = "Internal server error"
	}
	return NewAppError(
		"INTERNAL_ERROR",
		message,
		http.StatusInternalServerError,
		err,
	)
}

// NewDatabaseError creates a database error
func NewDatabaseError(operation string, err error) *AppError {
	return NewAppError(
		"DATABASE_ERROR",
		fmt.Sprintf("Database operation failed: %s", operation),
		http.StatusInternalServerError,
		err,
	)
}

// NewExternalServiceError creates an external service error
func NewExternalServiceError(service, operation string, err error) *AppError {
	return NewAppError(
		"EXTERNAL_SERVICE_ERROR",
		fmt.Sprintf("External service '%s' operation '%s' failed", service, operation),
		http.StatusServiceUnavailable,
		err,
	)
}

// ErrorResponse represents the standard error response structure
type ErrorResponse struct {
	Error   *AppError `json:"error"`
	Success bool      `json:"success"`
}

// NewErrorResponse creates a new error response
func NewErrorResponse(err *AppError) ErrorResponse {
	return ErrorResponse{
		Error:   err,
		Success: false,
	}
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

// Error codes for different categories of errors
const (
	ErrCodeInvalidCredentials = "INVALID_CREDENTIALS"
	ErrCodeTokenInvalid       = "TOKEN_INVALID"
	ErrCodeTokenMissing       = "TOKEN_MISSING"

	ErrCodeUserNotFound        = "USER_NOT_FOUND"
	ErrCodeInsufficientBalance = "INSUFFICIENT_BALANCE"
	ErrCodeInvalidCurrency     = "INVALID_CURRENCY"

	ErrCodeTransactionNotFound                = "TRANSACTION_NOT_FOUND"
	ErrCodeTransactionAlreadyExists           = "TRANSACTION_ALREADY_EXISTS"
	ErrCodeWithdrawalTransactionDoseNotExists = "TRANSACTION_WITHDRAWAL_DOSE_NOT_EXISTS"
	ErrCodeTransactionCannotCancel            = "TRANSACTION_CANNOT_CANCEL"
	ErrCodeTransactionInvalidStatus           = "TRANSACTION_INVALID_STATUS"
	ErrCodeInvalidAmount                      = "INVALID_AMOUNT"
	ErrCodeTransactionAlreadyDeposited        = "TRANSACTION_ALREADY_DEPOSITED"

	ErrCodeRequiredField    = "REQUIRED_FIELD"
	ErrCodeInvalidFormat    = "INVALID_FORMAT"
	ErrCodeInvalidRange     = "INVALID_RANGE"
	ErrCodeInvalidPrecision = "INVALID_PRECISION"

	ErrCodeDatabaseConnection = "DATABASE_CONNECTION_ERROR"
	ErrCodeDatabaseQuery      = "DATABASE_QUERY_ERROR"
	ErrCodeWalletServiceError = "WALLET_SERVICE_ERROR"
)
