package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	maxBatchSize     = 20
	maxResponseBytes = 32 << 20
	maxAttempts      = 3
)

type Usage struct {
	TotalTokens int64
}

type Embedder interface {
	Embed(context.Context, []string, string, int) ([][]float32, Usage, error)
}

type Client struct {
	apiKey     string
	endpoint   string
	httpClient *http.Client
}

func NewClient(apiKey, baseURL string, httpClient *http.Client) (*Client, error) {
	apiKey = strings.TrimSpace(apiKey)
	baseURL = strings.TrimSpace(baseURL)
	if apiKey == "" {
		return nil, errors.New("Bailian API key is required")
	}
	if baseURL == "" {
		return nil, errors.New("Bailian base URL is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid Bailian base URL %q", baseURL)
	}
	if parsed.Scheme != "https" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost" {
		return nil, errors.New("Bailian base URL must use HTTPS")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 2 * time.Minute}
	}
	return &Client{
		apiKey:     apiKey,
		endpoint:   strings.TrimRight(baseURL, "/") + "/embeddings",
		httpClient: httpClient,
	}, nil
}

type embeddingRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	Dimensions     int      `json:"dimensions"`
	EncodingFormat string   `json:"encoding_format"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Usage struct {
		TotalTokens int64 `json:"total_tokens"`
	} `json:"usage"`
}

type errorResponse struct {
	Error struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
}

func (client *Client) Embed(ctx context.Context, inputs []string, model string, dimensions int) ([][]float32, Usage, error) {
	if len(inputs) == 0 || len(inputs) > maxBatchSize {
		return nil, Usage{}, fmt.Errorf("embedding input count must be between 1 and %d", maxBatchSize)
	}
	if strings.TrimSpace(model) == "" {
		return nil, Usage{}, errors.New("embedding model is required")
	}
	if dimensions <= 0 {
		return nil, Usage{}, errors.New("embedding dimensions must be positive")
	}
	for index, input := range inputs {
		if strings.TrimSpace(input) == "" {
			return nil, Usage{}, fmt.Errorf("embedding input %d is empty", index)
		}
	}

	body, err := json.Marshal(embeddingRequest{
		Model:          model,
		Input:          inputs,
		Dimensions:     dimensions,
		EncodingFormat: "float",
	})
	if err != nil {
		return nil, Usage{}, fmt.Errorf("encode embedding request: %w", err)
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		vectors, usage, retryAfter, retry, requestErr := client.embedOnce(ctx, body, len(inputs), dimensions)
		if requestErr == nil {
			return vectors, usage, nil
		}
		if !retry || attempt == maxAttempts-1 {
			return nil, Usage{}, requestErr
		}
		if retryAfter <= 0 {
			retryAfter = time.Duration(250*(1<<attempt)) * time.Millisecond
		}
		timer := time.NewTimer(retryAfter)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, Usage{}, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, Usage{}, errors.New("embedding request exhausted retries")
}

func (client *Client) embedOnce(ctx context.Context, body []byte, inputCount, dimensions int) ([][]float32, Usage, time.Duration, bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, Usage{}, 0, false, fmt.Errorf("create embedding request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+client.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, Usage{}, 0, false, ctx.Err()
		}
		return nil, Usage{}, 0, true, fmt.Errorf("call embedding API: %w", err)
	}
	defer response.Body.Close()

	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, Usage{}, 0, true, fmt.Errorf("read embedding response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return nil, Usage{}, 0, false, errors.New("embedding response exceeds size limit")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		retry := response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError
		return nil, Usage{}, parseRetryAfter(response.Header.Get("Retry-After")), retry, embeddingAPIError(response.StatusCode, data)
	}

	var payload embeddingResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, Usage{}, 0, false, fmt.Errorf("decode embedding response: %w", err)
	}
	if len(payload.Data) != inputCount {
		return nil, Usage{}, 0, false, fmt.Errorf("embedding response count %d does not match input count %d", len(payload.Data), inputCount)
	}

	vectors := make([][]float32, inputCount)
	for _, item := range payload.Data {
		if item.Index < 0 || item.Index >= inputCount {
			return nil, Usage{}, 0, false, fmt.Errorf("embedding response index %d is out of range", item.Index)
		}
		if vectors[item.Index] != nil {
			return nil, Usage{}, 0, false, fmt.Errorf("embedding response index %d is duplicated", item.Index)
		}
		if len(item.Embedding) != dimensions {
			return nil, Usage{}, 0, false, fmt.Errorf("embedding response dimension %d does not match requested %d", len(item.Embedding), dimensions)
		}
		vectors[item.Index] = item.Embedding
	}
	return vectors, Usage{TotalTokens: payload.Usage.TotalTokens}, 0, false, nil
}

func embeddingAPIError(status int, data []byte) error {
	var payload errorResponse
	if json.Unmarshal(data, &payload) == nil && strings.TrimSpace(payload.Error.Message) != "" {
		if payload.Error.Code != "" {
			return fmt.Errorf("embedding API status %d (%s): %s", status, payload.Error.Code, payload.Error.Message)
		}
		return fmt.Errorf("embedding API status %d: %s", status, payload.Error.Message)
	}
	message := strings.TrimSpace(string(data))
	if len(message) > 512 {
		message = message[:512]
	}
	if message == "" {
		message = http.StatusText(status)
	}
	return fmt.Errorf("embedding API status %d: %s", status, message)
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
