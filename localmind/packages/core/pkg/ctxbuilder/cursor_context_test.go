package ctxbuilder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractScopeGoFunction(t *testing.T) {
	workspace := t.TempDir()
	cc := NewCursorContext(workspace)

	content := `package main

import "fmt"

func hello(name string) {
	greeting := "Hello, " + name
	fmt.Println(greeting)
}

func main() {
	hello("world")
}
`
	os.WriteFile(filepath.Join(workspace, "main.go"), []byte(content), 0644)

	// Cursor on line 6 (inside hello function)
	result := cc.ExtractScope("main.go", 6)

	if result == nil {
		t.Fatal("Expected a scope result")
	}
	if result.Name != "hello" {
		t.Errorf("Expected name 'hello', got %q", result.Name)
	}
	if result.Kind != "function" {
		t.Errorf("Expected kind 'function', got %q", result.Kind)
	}
	if !strings.Contains(result.Content, "greeting") {
		t.Error("Expected content to contain function body")
	}
}

func TestExtractScopeGoMethod(t *testing.T) {
	workspace := t.TempDir()
	cc := NewCursorContext(workspace)

	content := `package main

type Server struct {
	port int
}

func (s *Server) Start() error {
	return nil
}
`
	os.WriteFile(filepath.Join(workspace, "server.go"), []byte(content), 0644)

	// Cursor on line 8 (inside Start method)
	result := cc.ExtractScope("server.go", 8)

	if result == nil {
		t.Fatal("Expected a scope result")
	}
	if result.Name != "Start" {
		t.Errorf("Expected name 'Start', got %q", result.Name)
	}
	if result.Kind != "method" {
		t.Errorf("Expected kind 'method', got %q", result.Kind)
	}
}

func TestExtractScopeJSFunction(t *testing.T) {
	workspace := t.TempDir()
	cc := NewCursorContext(workspace)

	content := `function calculateSum(a, b) {
  const result = a + b;
  return result;
}
`
	os.WriteFile(filepath.Join(workspace, "calc.js"), []byte(content), 0644)

	result := cc.ExtractScope("calc.js", 2)

	if result == nil {
		t.Fatal("Expected a scope result")
	}
	if result.Name != "calculateSum" {
		t.Errorf("Expected name 'calculateSum', got %q", result.Name)
	}
	if result.Kind != "function" {
		t.Errorf("Expected kind 'function', got %q", result.Kind)
	}
}

func TestExtractScopePythonDef(t *testing.T) {
	workspace := t.TempDir()
	cc := NewCursorContext(workspace)

	// Python uses indentation, but our bracket parser won't work well.
	// This test documents behavior — Python support is best-effort.
	content := `def greet(name):
    print(f"Hello {name}")
    return True
`
	os.WriteFile(filepath.Join(workspace, "app.py"), []byte(content), 0644)

	result := cc.ExtractScope("app.py", 2)

	// Python doesn't use braces, so we expect nil (no scope found)
	if result != nil {
		t.Logf("Python scope detected (best-effort): %+v", result)
	}
}

func TestExtractScopeOutOfBounds(t *testing.T) {
	workspace := t.TempDir()
	cc := NewCursorContext(workspace)

	content := `package main
func main() {}
`
	os.WriteFile(filepath.Join(workspace, "main.go"), []byte(content), 0644)

	result := cc.ExtractScope("main.go", 999)
	if result != nil {
		t.Error("Expected nil for out-of-bounds line")
	}

	result = cc.ExtractScope("main.go", 0)
	if result != nil {
		t.Error("Expected nil for line 0")
	}
}

func TestExtractScopeFileNotFound(t *testing.T) {
	workspace := t.TempDir()
	cc := NewCursorContext(workspace)

	result := cc.ExtractScope("nonexistent.go", 1)
	if result != nil {
		t.Error("Expected nil for missing file")
	}
}

func TestBuildCursorContext(t *testing.T) {
	workspace := t.TempDir()
	cc := NewCursorContext(workspace)

	content := `package main

func process(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	return nil
}
`
	os.WriteFile(filepath.Join(workspace, "proc.go"), []byte(content), 0644)

	ctx := cc.BuildCursorContext("proc.go", 5)

	if ctx == "" {
		t.Fatal("Expected non-empty cursor context")
	}
	if !strings.Contains(ctx, "CURSOR CONTEXT") {
		t.Error("Expected context header")
	}
	if !strings.Contains(ctx, "process") {
		t.Error("Expected function name in context")
	}
}

func TestNestedScopes(t *testing.T) {
	workspace := t.TempDir()
	cc := NewCursorContext(workspace)

	content := `package main

func outer() {
	x := 1
	if x > 0 {
		inner := func() {
			y := 2
			_ = y
		}
		_ = inner
	}
}
`
	os.WriteFile(filepath.Join(workspace, "nested.go"), []byte(content), 0644)

	// Cursor on line 7 (inside the inner anonymous func)
	result := cc.ExtractScope("nested.go", 7)

	if result == nil {
		t.Fatal("Expected a scope result for nested position")
	}
	// Should find the innermost enclosing scope
	if !strings.Contains(result.Content, "y := 2") {
		t.Error("Expected innermost scope content")
	}
}
