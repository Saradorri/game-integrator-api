package wallet

import (
	"errors"
	"github.com/saradorri/gameintegrator/internal/domain"
)

// CreateWalletRequest creates a wallet transaction request with consistent parameters
func CreateWalletRequest(userID int64, currency string, amount float64, betID int64, reference string) domain.WalletTransactionRequest {
	return domain.WalletTransactionRequest{
		UserID:   userID,
		Currency: currency,
		Transactions: []domain.WalletRequestTransaction{
			{
				Amount:    amount,
				BetID:     betID,
				Reference: reference,
			},
		},
	}
}

// Is409Error checks if the error is a 409 conflict
func Is409Error(err error) bool {
	var walletErr *domain.WalletServiceError
	if errors.As(err, &walletErr) {
		return walletErr.StatusCode == 409
	}
	return false
}

// Is4xxError checks if the error is a 4xx client error
func Is4xxError(err error) bool {
	var walletErr *domain.WalletServiceError
	if errors.As(err, &walletErr) {
		return walletErr.StatusCode >= 400 && walletErr.StatusCode < 500
	}
	return false
}

// Is5xxError checks if the error is a 5xx server error
func Is5xxError(err error) bool {
	var walletErr *domain.WalletServiceError
	if errors.As(err, &walletErr) {
		return walletErr.StatusCode >= 500 && walletErr.StatusCode < 600
	}
	return false
}
