package domain

import (
	"fmt"
)

// WalletService defines the interface for external wallet service
type WalletService interface {
	GetBalance(userID int64) (WalletBalanceResponse, error)
	Deposit(req WalletTransactionRequest) (WalletTransactionResponse, error)
	Withdraw(req WalletTransactionRequest) (WalletTransactionResponse, error)
}

// WalletRequestTransaction represents a transaction in the wallet service
type WalletRequestTransaction struct {
	Amount    float64 `json:"amount"`
	BetID     int64   `json:"betId"`
	Reference string  `json:"reference"`
}

// WalletTransactionRequest represents a deposit/withdraw request to the wallet service
type WalletTransactionRequest struct {
	UserID       int64                      `json:"userId"`
	Currency     string                     `json:"currency"`
	Transactions []WalletRequestTransaction `json:"transactions"`
}

// WalletBalanceResponse represents the response from the balance endpoint
type WalletBalanceResponse struct {
	Balance  string `json:"balance"`
	Currency string `json:"currency"`
}

// WalletTransactionResponse represents the response from deposit/withdraw endpoints
type WalletTransactionResponse struct {
	Balance      string                      `json:"balance"`
	Transactions []WalletResponseTransaction `json:"transactions"`
}

// WalletResponseTransaction represents a transaction in operation response
type WalletResponseTransaction struct {
	ID        int    `json:"id"`
	Reference string `json:"reference"`
}

// WalletErrorResponse represents error responses from the wallet service
type WalletErrorResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

// WalletServiceError represents a wallet service error with status code
type WalletServiceError struct {
	StatusCode int
	Code       string
	Message    string
}

// Error implements the error interface
func (e *WalletServiceError) Error() string {
	return fmt.Sprintf("%s", e.Message)
}

// Is4xxError checks if the error is a 4xx client error
func (e *WalletServiceError) Is4xxError() bool {
	return e.StatusCode >= 400 && e.StatusCode < 500
}
