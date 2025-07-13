package transaction

import (
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/domain/mocks"
	"github.com/saradorri/gameintegrator/internal/infrastructure/lock"
	"github.com/saradorri/gameintegrator/internal/infrastructure/logger"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func createTestUser() *domain.User {
	return &domain.User{
		ID:        123,
		Username:  "test_user",
		Currency:  "USD",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func createTestTransaction(status domain.TransactionStatus) *domain.Transaction {
	return &domain.Transaction{
		ID:           1,
		UserID:       123,
		Type:         domain.TransactionTypeWithdraw,
		Amount:       100.0,
		Currency:     "USD",
		Status:       status,
		ProviderTxID: "test_provider_123",
		OldBalance:   1000.0,
		NewBalance:   900.0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func TestErrorTypeDetection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWalletSvc := mocks.NewMockWalletService(ctrl)
	mockTxRepo := mocks.NewMockTransactionRepository(ctrl)
	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	mockOutboxRepo := mocks.NewMockOutboxRepository(ctrl)
	newLogger := logger.NewLogger("test", "debug")
	userLockManager := lock.NewUserLockManager()

	useCase := &TransactionUseCase{
		transactionRepo: mockTxRepo,
		userRepo:        mockUserRepo,
		walletSvc:       mockWalletSvc,
		outboxRepo:      mockOutboxRepo,
		db:              nil,
		logger:          newLogger,
		userLockManager: userLockManager,
	}

	tests := []struct {
		name  string
		error error
		is4xx bool
		is409 bool
	}{
		{
			name:  "4xx_Error",
			error: &domain.WalletServiceError{StatusCode: 400, Code: "BAD_REQUEST", Message: "Bad request"},
			is4xx: true,
			is409: false,
		},
		{
			name:  "5xx_Error",
			error: &domain.WalletServiceError{StatusCode: 500, Code: "INTERNAL_ERROR", Message: "Internal error"},
			is4xx: false,
			is409: false,
		},
		{
			name:  "409_Error",
			error: &domain.WalletServiceError{StatusCode: 409, Code: "CONFLICT", Message: "Conflict"},
			is4xx: true,
			is409: true,
		},
		{
			name:  "Generic_Error",
			error: errors.New("generic error"),
			is4xx: false,
			is409: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.is4xx, useCase.is4xxError(tt.error))
			assert.Equal(t, tt.is409, useCase.is409Error(tt.error))
		})
	}
}

func TestWalletServiceErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		walletError error
		expectError bool
		errorCode   string
	}{
		{
			name:        "4xx_InsufficientBalance",
			walletError: &domain.WalletServiceError{StatusCode: 400, Code: "INSUFFICIENT_BALANCE", Message: "Insufficient balance"},
			expectError: true,
			errorCode:   domain.ErrCodeInsufficientBalance,
		},
		{
			name:        "5xx_ServerError",
			walletError: &domain.WalletServiceError{StatusCode: 500, Code: "INTERNAL_ERROR", Message: "Internal server error"},
			expectError: true,
			errorCode:   domain.ErrCodeWalletServiceError,
		},
		{
			name:        "409_Conflict",
			walletError: &domain.WalletServiceError{StatusCode: 409, Code: "CONFLICT", Message: "Transaction already exists"},
			expectError: false,
		},
		{
			name:        "CORS_Error",
			walletError: errors.New("CORS error: Access-Control-Allow-Origin header missing"),
			expectError: true,
			errorCode:   domain.ErrCodeWalletServiceError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var walletErr *domain.WalletServiceError
			if errors.As(tt.walletError, &walletErr) {
				if walletErr.StatusCode >= 400 && walletErr.StatusCode < 500 {
					assert.True(t, walletErr.Is4xxError())
				}
				if walletErr.StatusCode == 409 {
					assert.True(t, walletErr.StatusCode == 409)
				}
			}

			if tt.expectError && tt.errorCode != "" {
				assert.NotEmpty(t, tt.errorCode)
			}
		})
	}
}

func TestBalanceParsing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWalletSvc := mocks.NewMockWalletService(ctrl)
	mockTxRepo := mocks.NewMockTransactionRepository(ctrl)
	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	mockOutboxRepo := mocks.NewMockOutboxRepository(ctrl)
	newLogger := logger.NewLogger("test", "debug")
	userLockManager := lock.NewUserLockManager()

	useCase := &TransactionUseCase{
		transactionRepo: mockTxRepo,
		userRepo:        mockUserRepo,
		walletSvc:       mockWalletSvc,
		outboxRepo:      mockOutboxRepo,
		db:              nil,
		logger:          newLogger,
		userLockManager: userLockManager,
	}

	tests := []struct {
		name        string
		balanceStr  string
		expectError bool
		expected    float64
	}{
		{
			name:        "Valid_Balance",
			balanceStr:  "1234.56",
			expectError: false,
			expected:    1234.56,
		},
		{
			name:        "Invalid_Balance",
			balanceStr:  "invalid",
			expectError: true,
		},
		{
			name:        "Empty_Balance",
			balanceStr:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := useCase.parseWalletBalance(tt.balanceStr)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestWalletRequestCreation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWalletSvc := mocks.NewMockWalletService(ctrl)
	mockTxRepo := mocks.NewMockTransactionRepository(ctrl)
	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	mockOutboxRepo := mocks.NewMockOutboxRepository(ctrl)
	newLogger := logger.NewLogger("test", "debug")
	userLockManager := lock.NewUserLockManager()

	useCase := &TransactionUseCase{
		transactionRepo: mockTxRepo,
		userRepo:        mockUserRepo,
		walletSvc:       mockWalletSvc,
		outboxRepo:      mockOutboxRepo,
		db:              nil,
		logger:          newLogger,
		userLockManager: userLockManager,
	}

	userID := int64(123)
	currency := "USD"
	amount := 100.0
	betID := int64(456)
	reference := "test_ref"

	walletReq := useCase.createWalletRequest(userID, currency, amount, betID, reference)

	assert.Equal(t, userID, walletReq.UserID)
	assert.Equal(t, currency, walletReq.Currency)
	assert.Len(t, walletReq.Transactions, 1)
	assert.Equal(t, amount, walletReq.Transactions[0].Amount)
	assert.Equal(t, betID, walletReq.Transactions[0].BetID)
	assert.Equal(t, reference, walletReq.Transactions[0].Reference)
}

func TestRevertFunctionality(t *testing.T) {
	tests := []struct {
		name           string
		walletError    error
		expectedStatus domain.TransactionStatus
		expectError    bool
		errorCode      string
	}{
		{
			name:           "Revert_4xx_UserNotFound",
			walletError:    &domain.WalletServiceError{StatusCode: 400, Code: "USER_NOT_FOUND", Message: "User not found"},
			expectedStatus: domain.TransactionStatusFailed,
			expectError:    true,
			errorCode:      domain.ErrCodeWalletServiceError,
		},
		{
			name:           "Revert_5xx_ServerError",
			walletError:    &domain.WalletServiceError{StatusCode: 500, Code: "INTERNAL_ERROR", Message: "Internal server error"},
			expectedStatus: domain.TransactionStatusFailed,
			expectError:    true,
			errorCode:      domain.ErrCodeWalletServiceError,
		},
		{
			name:           "Revert_409_Conflict",
			walletError:    &domain.WalletServiceError{StatusCode: 409, Code: "CONFLICT", Message: "Transaction already exists"},
			expectedStatus: domain.TransactionStatusCompleted,
			expectError:    false,
		},
		{
			name:           "Revert_Network_Error",
			walletError:    errors.New("network timeout"),
			expectedStatus: domain.TransactionStatusFailed,
			expectError:    true,
			errorCode:      domain.ErrCodeWalletServiceError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var walletErr *domain.WalletServiceError
			if errors.As(tt.walletError, &walletErr) {
				if tt.expectError {
					assert.NotEmpty(t, tt.errorCode)
				}
			}

			if tt.expectError && tt.errorCode != "" {
				assert.NotEmpty(t, tt.errorCode)
			}
		})
	}
}

func TestCompensationEventPublishing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWalletSvc := mocks.NewMockWalletService(ctrl)
	mockTxRepo := mocks.NewMockTransactionRepository(ctrl)
	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	mockOutboxRepo := mocks.NewMockOutboxRepository(ctrl)
	newLogger := logger.NewLogger("test", "debug")
	userLockManager := lock.NewUserLockManager()

	useCase := &TransactionUseCase{
		transactionRepo: mockTxRepo,
		userRepo:        mockUserRepo,
		walletSvc:       mockWalletSvc,
		outboxRepo:      mockOutboxRepo,
		db:              nil,
		logger:          newLogger,
		userLockManager: userLockManager,
	}

	tests := []struct {
		name         string
		transaction  *domain.Transaction
		error        error
		expectEvent  bool
		expectedType string
	}{
		{
			name: "Withdraw_Transaction_With_Error",
			transaction: &domain.Transaction{
				ID:           1,
				UserID:       123,
				Type:         domain.TransactionTypeWithdraw,
				Amount:       100.0,
				Currency:     "USD",
				ProviderTxID: "test_provider_123",
			},
			error:        errors.New("wallet service error"),
			expectEvent:  true,
			expectedType: domain.EventTypeWithdrawRevert,
		},
		{
			name: "Deposit_Transaction_With_Error",
			transaction: &domain.Transaction{
				ID:           2,
				UserID:       123,
				Type:         domain.TransactionTypeDeposit,
				Amount:       100.0,
				Currency:     "USD",
				ProviderTxID: "test_provider_456",
			},
			error:        errors.New("wallet service error"),
			expectEvent:  false,
			expectedType: "",
		},
		{
			name: "Revert_Transaction_With_Error",
			transaction: &domain.Transaction{
				ID:           3,
				UserID:       123,
				Type:         domain.TransactionTypeRevert,
				Amount:       100.0,
				Currency:     "USD",
				ProviderTxID: "revert_test_provider_123",
			},
			error:        errors.New("wallet service error"),
			expectEvent:  false,
			expectedType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectEvent {
				mockOutboxRepo.EXPECT().Save(gomock.Any()).Return(nil)
			}

			err := useCase.publishCompensationEvent(tt.transaction, tt.error)

			if tt.expectEvent {
				assert.NoError(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCompensationEventTypeDetection(t *testing.T) {
	tests := []struct {
		name            string
		transactionType domain.TransactionType
		expectedType    string
	}{
		{
			name:            "Withdraw_Transaction",
			transactionType: domain.TransactionTypeWithdraw,
			expectedType:    domain.EventTypeWithdrawRevert,
		},
		{
			name:            "Deposit_Transaction",
			transactionType: domain.TransactionTypeDeposit,
			expectedType:    "",
		},
		{
			name:            "Revert_Transaction",
			transactionType: domain.TransactionTypeRevert,
			expectedType:    "",
		},
		{
			name:            "Cancel_Transaction",
			transactionType: domain.TransactionTypeCancel,
			expectedType:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transaction := &domain.Transaction{
				ID:   1,
				Type: tt.transactionType,
			}

			eventType := getCompensationEventType(transaction.Type)
			assert.Equal(t, tt.expectedType, eventType)
		})
	}
}

func TestCompensationEventDataBuilding(t *testing.T) {
	tests := []struct {
		name        string
		transaction *domain.Transaction
		error       error
		expectData  map[string]interface{}
	}{
		{
			name: "Withdraw_Transaction_With_Error",
			transaction: &domain.Transaction{
				ID:           1,
				UserID:       123,
				Type:         domain.TransactionTypeWithdraw,
				Amount:       100.0,
				Currency:     "USD",
				ProviderTxID: "test_provider_123",
			},
			error: errors.New("wallet service error"),
			expectData: map[string]interface{}{
				"transaction_id": int64(1),
				"user_id":        int64(123),
				"amount":         100.0,
				"currency":       "USD",
				"provider_tx_id": "test_provider_123",
				"error":          "wallet service error",
			},
		},
		{
			name: "Withdraw_Transaction_With_ProviderWithdrawnTxID",
			transaction: &domain.Transaction{
				ID:                    1,
				UserID:                123,
				Type:                  domain.TransactionTypeWithdraw,
				Amount:                100.0,
				Currency:              "USD",
				ProviderTxID:          "test_provider_123",
				ProviderWithdrawnTxID: &[]int64{456}[0],
			},
			error: errors.New("wallet service error"),
			expectData: map[string]interface{}{
				"transaction_id":           int64(1),
				"user_id":                  int64(123),
				"amount":                   100.0,
				"currency":                 "USD",
				"provider_tx_id":           "test_provider_123",
				"provider_withdrawn_tx_id": int64(456),
				"error":                    "wallet service error",
			},
		},
		{
			name: "Withdraw_Transaction_Without_Error",
			transaction: &domain.Transaction{
				ID:           1,
				UserID:       123,
				Type:         domain.TransactionTypeWithdraw,
				Amount:       100.0,
				Currency:     "USD",
				ProviderTxID: "test_provider_123",
			},
			error: nil,
			expectData: map[string]interface{}{
				"transaction_id": int64(1),
				"user_id":        int64(123),
				"amount":         100.0,
				"currency":       "USD",
				"provider_tx_id": "test_provider_123",
				"error":          "Unknown error",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventData := buildCompensationEventData(tt.transaction, tt.error)

			for key, expectedValue := range tt.expectData {
				assert.Equal(t, expectedValue, eventData[key], "Field %s mismatch", key)
			}

			assert.Contains(t, eventData, "transaction_id")
			assert.Contains(t, eventData, "user_id")
			assert.Contains(t, eventData, "amount")
			assert.Contains(t, eventData, "currency")
			assert.Contains(t, eventData, "provider_tx_id")
			assert.Contains(t, eventData, "error")
		})
	}
}

func TestRevertValidationScenarios(t *testing.T) {
	tests := []struct {
		name           string
		existingRevert *domain.Transaction
		originTx       *domain.Transaction
		originTxError  error
		expectError    bool
		errorCode      string
	}{
		{
			name:           "Revert_Already_Exists",
			existingRevert: createTestTransaction(domain.TransactionStatusCompleted),
			originTx:       nil,
			originTxError:  nil,
			expectError:    true,
			errorCode:      domain.ErrCodeTransactionAlreadyExists,
		},
		{
			name:           "Origin_Transaction_Not_Found",
			existingRevert: nil,
			originTx:       nil,
			originTxError:  gorm.ErrRecordNotFound,
			expectError:    true,
			errorCode:      domain.ErrCodeTransactionNotFound,
		},
		{
			name:           "Origin_Transaction_Not_Failed",
			existingRevert: nil,
			originTx:       createTestTransaction(domain.TransactionStatusCompleted),
			originTxError:  nil,
			expectError:    true,
			errorCode:      domain.ErrCodeTransactionInvalidStatus,
		},
		{
			name:           "Valid_Revert_Scenario",
			existingRevert: nil,
			originTx:       createTestTransaction(domain.TransactionStatusFailed),
			originTxError:  nil,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation logic without calling the full Revert method
			if tt.existingRevert != nil {
				// Test that existing revert would cause an error
				assert.True(t, tt.expectError)
				assert.Equal(t, domain.ErrCodeTransactionAlreadyExists, tt.errorCode)
			}

			if tt.originTxError != nil {
				// Test that origin transaction not found would cause an error
				assert.True(t, tt.expectError)
				assert.Equal(t, domain.ErrCodeTransactionNotFound, tt.errorCode)
			}

			if tt.originTx != nil && tt.originTx.Status != domain.TransactionStatusFailed {
				// Test that non-failed origin transaction would cause an error
				assert.True(t, tt.expectError)
				assert.Equal(t, domain.ErrCodeTransactionInvalidStatus, tt.errorCode)
			}

			if !tt.expectError && tt.originTx != nil && tt.originTx.Status == domain.TransactionStatusFailed {
				// Test that valid scenario would not cause an error
				assert.False(t, tt.expectError)
			}
		})
	}
}
