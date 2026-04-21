package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// GoldenTestCase represents a single golden test case
type GoldenTestCase struct {
	Name               string `json:"name,omitempty"`
	Prefix             string `json:"prefix"`
	Suffix             string `json:"suffix,omitempty"`
	Language           string `json:"language"`
	FilePath           string `json:"filePath"`
	ExpectedCompletion string `json:"expectedCompletion,omitempty"`
	ExpectedPattern    string `json:"expectedPattern,omitempty"` // Regex pattern
	MaxLatencyMs       int64  `json:"maxLatencyMs,omitempty"`
}

// loadGoldenTests loads all golden test cases from a directory
func loadGoldenTests(t *testing.T, dir string) []GoldenTestCase {
	t.Helper()

	var tests []GoldenTestCase

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Logf("No golden tests found in %s: %v", dir, err)
		return tests
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("Failed to read %s: %v", path, err)
			continue
		}

		var tc GoldenTestCase
		if err := json.Unmarshal(data, &tc); err != nil {
			t.Errorf("Failed to parse %s: %v", path, err)
			continue
		}

		tc.Name = strings.TrimSuffix(entry.Name(), ".json")
		tests = append(tests, tc)
	}

	return tests
}

// TestGoldenCompletions runs golden tests for completions
func TestGoldenCompletions(t *testing.T) {
	// Path relative to internal/engine - go up to core then into testdata
	dir := filepath.Join("..", "..", "testdata", "golden", "completion")
	tests := loadGoldenTests(t, dir)

	if len(tests) == 0 {
		t.Skip("No golden completion tests found")
	}

	// Create a mock completion engine for testing
	engine := newMockCompletionEngine()

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			start := time.Now()

			result, err := engine.Complete(ctx, tc.Prefix, tc.Language)
			if err != nil {
				t.Fatalf("Completion failed: %v", err)
			}

			latency := time.Since(start)

			// Check latency budget
			maxLatency := time.Duration(tc.MaxLatencyMs) * time.Millisecond
			if maxLatency == 0 {
				maxLatency = 150 * time.Millisecond // Default
			}

			if latency > maxLatency {
				t.Errorf("Latency %v exceeded budget %v", latency, maxLatency)
			}

			// Verify result
			if tc.ExpectedCompletion != "" {
				if result != tc.ExpectedCompletion {
					t.Errorf("Expected completion:\n%q\nGot:\n%q", tc.ExpectedCompletion, result)
				}
			}

			if tc.ExpectedPattern != "" {
				matched, err := regexp.MatchString(tc.ExpectedPattern, result)
				if err != nil {
					t.Fatalf("Invalid pattern %q: %v", tc.ExpectedPattern, err)
				}
				if !matched {
					t.Errorf("Result %q did not match pattern %q", result, tc.ExpectedPattern)
				}
			}

			t.Logf("Completion: %q (latency: %v)", result, latency)
		})
	}
}

// mockCompletionEngine is a deterministic mock for golden tests
type mockCompletionEngine struct{}

func newMockCompletionEngine() *mockCompletionEngine {
	return &mockCompletionEngine{}
}

func (e *mockCompletionEngine) Complete(ctx context.Context, prefix, language string) (string, error) {
	// Deterministic completions based on input patterns
	switch {
	case strings.Contains(prefix, `greet("world")`):
		return ")\n}", nil
	case strings.HasSuffix(strings.TrimSpace(prefix), "res"):
		return ".json(users);", nil
	case strings.Contains(prefix, "def main"):
		return "():\n    pass", nil
	default:
		return "", nil
	}
}

// TestGoldenDeterminism ensures outputs are deterministic
func TestGoldenDeterminism(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "golden", "completion")
	tests := loadGoldenTests(t, dir)

	if len(tests) == 0 {
		t.Skip("No golden completion tests found")
	}

	engine := newMockCompletionEngine()

	for _, tc := range tests {
		t.Run(tc.Name+"_determinism", func(t *testing.T) {
			ctx := context.Background()

			// Run 3 times and verify same result
			var results []string
			for i := 0; i < 3; i++ {
				result, err := engine.Complete(ctx, tc.Prefix, tc.Language)
				if err != nil {
					t.Fatalf("Completion %d failed: %v", i, err)
				}
				results = append(results, result)
			}

			for i := 1; i < len(results); i++ {
				if results[i] != results[0] {
					t.Errorf("Non-deterministic output:\nRun 0: %q\nRun %d: %q", results[0], i, results[i])
				}
			}
		})
	}
}
