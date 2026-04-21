package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_Ping(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/version" {
			json.NewEncoder(w).Encode(VersionResponse{Version: "0.1.0"})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	err := client.Ping(context.Background())
	if err != nil {
		t.Errorf("Ping() error = %v", err)
	}

	if !client.IsConnected() {
		t.Error("Client should be connected after successful ping")
	}
}

func TestClient_PingFailed(t *testing.T) {
	client := NewClient(&ClientConfig{
		BaseURL: "http://localhost:99999", // Invalid port
		Timeout: 100 * time.Millisecond,
	})

	err := client.Ping(context.Background())
	if err == nil {
		t.Error("Ping() should fail for unreachable server")
	}

	if client.IsConnected() {
		t.Error("Client should not be connected after failed ping")
	}
}

func TestClient_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			json.NewEncoder(w).Encode(ListModelsResponse{
				Models: []ModelInfo{
					{Name: "qwen2.5-coder:1.5b", Size: 1000000000},
					{Name: "llama2:7b", Size: 4000000000},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL})

	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}

	if len(models) != 2 {
		t.Errorf("ListModels() returned %d models, want 2", len(models))
	}
}

func TestClient_HasModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ListModelsResponse{
			Models: []ModelInfo{
				{Name: "qwen2.5-coder:1.5b"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL})

	has, err := client.HasModel(context.Background(), "qwen2.5-coder:1.5b")
	if err != nil {
		t.Fatalf("HasModel() error = %v", err)
	}
	if !has {
		t.Error("HasModel() should return true for existing model")
	}

	has, err = client.HasModel(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("HasModel() error = %v", err)
	}
	if has {
		t.Error("HasModel() should return false for nonexistent model")
	}
}

func TestClient_GenerateStreaming(t *testing.T) {
	responses := []GenerateResponse{
		{Response: "Hello", Done: false},
		{Response: " World", Done: false},
		{Response: "!", Done: true},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/generate" {
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "Streaming not supported", http.StatusInternalServerError)
				return
			}

			for _, resp := range responses {
				json.NewEncoder(w).Encode(resp)
				flusher.Flush()
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL})

	var tokens []string
	resp, err := client.Generate(context.Background(), &GenerateRequest{
		Model:  "test",
		Prompt: "hello",
	}, func(token string, done bool) error {
		tokens = append(tokens, token)
		return nil
	})

	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if resp.Response != "Hello World!" {
		t.Errorf("Generate() response = %q, want %q", resp.Response, "Hello World!")
	}

	if len(tokens) != 3 {
		t.Errorf("Received %d tokens, want 3", len(tokens))
	}
}

func TestClient_GenerateCancellation(t *testing.T) {
	// Server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		json.NewEncoder(w).Encode(GenerateResponse{Response: "too late", Done: true})
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Generate(ctx, &GenerateRequest{
		Model:  "test",
		Prompt: "hello",
	}, nil)

	if err == nil {
		t.Error("Generate() should return error on cancellation")
	}
}
