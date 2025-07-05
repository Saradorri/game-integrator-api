package wallet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/saradorri/gameintegrator/internal/domain"
	"io"
	"net/http"
	"time"
)

type walletServiceImpl struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewWalletService(baseURL, apiKey string) domain.WalletService {
	return &walletServiceImpl{
		baseURL: baseURL,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (w *walletServiceImpl) GetBalance(userID int64) (domain.WalletBalanceResponse, error) {
	url := fmt.Sprintf("%s/api/v1/balance/%s", w.baseURL, userID)
	var resp domain.WalletBalanceResponse
	err := w.sendRequest("GET", url, nil, http.StatusOK, &resp)
	return resp, err
}

func (w *walletServiceImpl) Deposit(reqData domain.WalletTransactionRequest) (domain.WalletTransactionResponse, error) {
	url := fmt.Sprintf("%s/api/v1/deposit", w.baseURL)
	var resp domain.WalletTransactionResponse
	err := w.sendRequest("POST", url, reqData, http.StatusCreated, &resp)
	return resp, err
}

func (w *walletServiceImpl) Withdraw(reqData domain.WalletTransactionRequest) (domain.WalletTransactionResponse, error) {
	url := fmt.Sprintf("%s/api/v1/withdraw", w.baseURL)
	var resp domain.WalletTransactionResponse
	err := w.sendRequest("POST", url, reqData, http.StatusCreated, &resp)
	return resp, err
}

// method to send HTTP requests and handle responses
func (w *walletServiceImpl) sendRequest(method, url string, bodyData any, expectedStatus int, out any) error {
	var body io.Reader

	if bodyData != nil {
		jsonBytes, err := json.Marshal(bodyData)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewBuffer(jsonBytes)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", w.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != expectedStatus {
		var errResp domain.WalletErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return fmt.Errorf("wallet service error: %s - %s", errResp.Code, errResp.Msg)
		}
		return fmt.Errorf("wallet service error: unexpected status %d - %s", resp.StatusCode, string(respBody))
	}

	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}
