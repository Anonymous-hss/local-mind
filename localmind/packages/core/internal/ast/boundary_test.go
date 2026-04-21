package ast

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBoundaryDetector_DetectLayer(t *testing.T) {
	bd := NewBoundaryDetector()

	tests := []struct {
		name     string
		input    string
		expected LayerType
	}{
		{"API Layer", "handlers", LayerAPI},
		{"Controller Layer", "controllers", LayerController},
		{"Service Layer", "services", LayerService},
		{"Repository Layer", "repository", LayerRepository},
		{"Model Layer", "models", LayerModel},
		{"Utils Layer", "utils", LayerUtil},
		{"Test Layer", "tests", LayerTest},
		{"Config Layer", "config", LayerConfig},
		{"Unknown Layer", "random", LayerUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bd.DetectLayer(tt.input)
			if got != tt.expected {
				t.Errorf("DetectLayer(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestBoundaryDetector_DetectFramework(t *testing.T) {
	// Create temporary directory structure
	tmpDir, err := os.MkdirTemp("", "framework_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	bd := NewBoundaryDetector()

	// Test Go
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0644)
	if fw := bd.detectFramework(tmpDir); fw != "go" {
		t.Errorf("Expected go, got %s", fw)
	}
	os.Remove(filepath.Join(tmpDir, "go.mod"))

	// Test Node/React
	packageJson := `{"dependencies": {"react": "^18.0.0"}}`
	os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(packageJson), 0644)
	if fw := bd.detectFramework(tmpDir); fw != "react" {
		t.Errorf("Expected react, got %s", fw)
	}
}
