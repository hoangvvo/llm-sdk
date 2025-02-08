package clientutils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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

// DoSSE performs a streaming SSE POST request and returns a stream
func DoSSE[T any](ctx context.Context, client *http.Client, config SSERequestConfig) (*SSEStream[T], error) {
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

	return &SSEStream[T]{
		response: resp,
		scanner:  sse.NewScanner(resp.Body),
	}, nil
}

// SSEStream represents a server-sent event stream
type SSEStream[T any] struct {
	response *http.Response
	scanner  *sse.Scanner
}

// Close closes the SSE stream
func (s *SSEStream[T]) Close() error {
	return s.response.Body.Close()
}

func (s *SSEStream[T]) Next() bool {
	return s.scanner.Scan()
}

func (s *SSEStream[T]) Current() (*T, error) {
	line := s.scanner.Text()
	if line == "" || strings.HasPrefix(line, ":") {
		return nil, nil
	}
	if data, ok := sse.IsDataLine(line); ok {
		if data == "[DONE]" {
			return nil, nil
		}
		var result T
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal SSE data: %w", err)
		}
		return &result, nil
	}
	return nil, nil
}

func (s *SSEStream[T]) Err() error {
	return s.scanner.Err()
}
