package wallet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/saradorri/gameintegrator/internal/domain"
)

type walletServiceImpl struct {
	baseURL string
	apiKey  string
	client  *retryablehttp.Client
}

func NewWalletService(baseURL, apiKey string) domain.WalletService {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 3 // TODO: 3 times makes response time too high but for reducing failures set to 5
	retryClient.Logger = log.New(os.Stdout, "retryable_http: ", log.LstdFlags)
	retryClient.Backoff = retryablehttp.DefaultBackoff // 100ms → 200ms → 400ms → 800ms → 1.6s (with jitter)

	return &walletServiceImpl{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  retryClient,
	}
}

func (w *walletServiceImpl) GetBalance(userID int64) (domain.WalletBalanceResponse, error) {
	url := fmt.Sprintf("%s/api/v1/balance/%d", w.baseURL, userID)
	var resp domain.WalletBalanceResponse
	err := w.sendRequest("GET", url, nil, http.StatusOK, &resp)
	return resp, err
}

func (w *walletServiceImpl) Deposit(reqData domain.WalletTransactionRequest) (domain.WalletTransactionResponse, error) {
	url := fmt.Sprintf("%s/api/v1/deposit", w.baseURL)
	var resp domain.WalletTransactionResponse
	err := w.sendRequest("POST", url, reqData, http.StatusOK, &resp)
	return resp, err
}

func (w *walletServiceImpl) Withdraw(reqData domain.WalletTransactionRequest) (domain.WalletTransactionResponse, error) {
	url := fmt.Sprintf("%s/api/v1/withdraw", w.baseURL)
	var resp domain.WalletTransactionResponse
	err := w.sendRequest("POST", url, reqData, http.StatusOK, &resp)
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

	req, err := retryablehttp.NewRequest(method, url, body)
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
			return &domain.WalletServiceError{
				StatusCode: resp.StatusCode,
				Code:       errResp.Code,
				Message:    errResp.Msg,
			}
		}
		return &domain.WalletServiceError{
			StatusCode: resp.StatusCode,
			Code:       "UNKNOWN_ERROR",
			Message:    string(respBody),
		}
	}

	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}
