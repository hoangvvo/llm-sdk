package clientutils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hoangvvo/llm-sdk/sdk-go/internal/sse"
)

// JSONRequestConfig holds configuration for JSON requests
type JSONRequestConfig struct {
	URL     string
	Headers map[string]string
	Body    any
}

// SSERequestConfig holds configuration for SSE requests
type SSERequestConfig struct {
	URL     string
	Headers map[string]string
	Body    any
}

// DoJSON performs a JSON POST request and unmarshals the response
func DoJSON[T any](ctx context.Context, client *http.Client, config JSONRequestConfig) (*T, error) {
	// Marshal request body
	reqBody, err := json.Marshal(config.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", config.URL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")

	// Set custom headers
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Unmarshal response
	var result T
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// SSEStream represents a server-sent event stream
type SSEStream struct {
	Response *http.Response
	Scanner  *sse.Scanner
}

// Close closes the SSE stream
func (s *SSEStream) Close() error {
	return s.Response.Body.Close()
}

// DoSSE performs a streaming SSE POST request and returns a stream
func DoSSE(ctx context.Context, client *http.Client, config SSERequestConfig) (*SSEStream, error) {
	// Marshal request body
	reqBody, err := json.Marshal(config.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", config.URL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	// Set custom headers
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	return &SSEStream{
		Response: resp,
		Scanner:  sse.NewScanner(resp.Body),
	}, nil
}
