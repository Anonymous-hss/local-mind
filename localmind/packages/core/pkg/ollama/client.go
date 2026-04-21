package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client is an HTTP client for the Ollama API
type Client struct {
	baseURL    string
	httpClient *http.Client
	mu         sync.RWMutex
	connected  bool
}

// ClientConfig holds client configuration
type ClientConfig struct {
	BaseURL        string
	Timeout        time.Duration
	KeepAlive      time.Duration
	MaxIdleConns   int
}

// DefaultClientConfig returns default configuration
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		BaseURL:      "http://localhost:11434",
		Timeout:      30 * time.Second,
		KeepAlive:    30 * time.Second,
		MaxIdleConns: 10,
	}
}

// NewClient creates a new Ollama client
func NewClient(config *ClientConfig) *Client {
	if config == nil {
		config = DefaultClientConfig()
	}

	transport := &http.Transport{
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxIdleConns,
		IdleConnTimeout:     config.KeepAlive,
		DisableKeepAlives:   false,
	}

	return &Client{
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   config.Timeout,
		},
	}
}

// IsConnected returns the current connection status
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// setConnected updates connection status
func (c *Client) setConnected(connected bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = connected
}

// Ping checks if Ollama is reachable
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/version", nil)
	if err != nil {
		c.setConnected(false)
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.setConnected(false)
		return fmt.Errorf("ollama not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.setConnected(false)
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	c.setConnected(true)
	return nil
}

// ListModels returns available models
func (c *Client) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.setConnected(false)
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer resp.Body.Close()

	c.setConnected(true)

	var result ListModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Models, nil
}

// HasModel checks if a specific model is available
func (c *Client) HasModel(ctx context.Context, modelName string) (bool, error) {
	models, err := c.ListModels(ctx)
	if err != nil {
		return false, err
	}

	for _, m := range models {
		if m.Name == modelName {
			return true, nil
		}
	}
	return false, nil
}

// StreamCallback is called for each token during streaming generation
type StreamCallback func(token string, done bool) error

// Generate performs a completion request with streaming
func (c *Client) Generate(ctx context.Context, req *GenerateRequest, callback StreamCallback) (*GenerateResponse, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.setConnected(false)
		return nil, fmt.Errorf("generate request failed: %w", err)
	}
	defer resp.Body.Close()

	c.setConnected(true)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Read streaming response
	var fullResponse string
	var lastResp GenerateResponse

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var streamResp GenerateResponse
		if err := json.Unmarshal(line, &streamResp); err != nil {
			continue
		}

		fullResponse += streamResp.Response
		lastResp = streamResp

		if callback != nil {
			if err := callback(streamResp.Response, streamResp.Done); err != nil {
				return nil, err
			}
		}

		if streamResp.Done {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading stream: %w", err)
	}

	lastResp.Response = fullResponse
	return &lastResp, nil
}

// GenerateSync performs a non-streaming completion request
func (c *Client) GenerateSync(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.setConnected(false)
		return nil, fmt.Errorf("generate request failed: %w", err)
	}
	defer resp.Body.Close()

	c.setConnected(true)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
