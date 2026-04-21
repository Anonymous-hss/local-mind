package ctxbuilder

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestResolveImportsGo(t *testing.T) {
	workspace := t.TempDir()
	resolver := NewImportResolver(workspace)

	// main.go imports utils.go
	mainContent := `package main
import (
	"fmt"
	"./pkg/utils"
)
func main() {}`

	utilsDir := filepath.Join(workspace, "pkg", "utils")
	os.MkdirAll(utilsDir, 0755)

	os.WriteFile(filepath.Join(workspace, "main.go"), []byte(mainContent), 0644)
	os.WriteFile(filepath.Join(utilsDir, "utils.go"), []byte("package utils"), 0644)

	imports, err := resolver.ResolveImports([]string{"main.go"})
	if err != nil {
		t.Fatalf("ResolveImports failed: %v", err)
	}

	// The resolver might return absolute path or relative depending on implementation detail
	// Our implementation joins workspaceRoot, but returns relative paths if input was relative?
	// Actually resolvePath returns full path if fileExists check passes which uses join.
	// Wait, resolvePath logic:
	// 1. Join(dir, importPath) -> triggers OK check.
	// The returned path from ResolveImports should probably be consistent with input.
	// Let's check what resolvePath returns. It returns the candidate path which is join(dir, import).

	// Adjusting test expectation: result will be relative to workspace if input was?
	// No, filepath.Join(dir, import) where dir is from filepath.Dir("main.go") -> ".".
	// So "pkg/utils" -> expanded to "pkg/utils.go" -> returned.

	// Verify we got the util file
	if len(imports) == 0 {
		t.Fatal("Expected at least one import, got 0")
	}

	found := false
	for _, imp := range imports {
		if strings.HasSuffix(imp, "utils.go") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected utils.go in imports, got: %v", imports)
	}
}

func TestResolveImportsTS(t *testing.T) {
	workspace := t.TempDir()
	resolver := NewImportResolver(workspace)

	// app.ts imports helper
	appContent := `import { help } from './helper';
const x = require('./legacy');`

	os.WriteFile(filepath.Join(workspace, "app.ts"), []byte(appContent), 0644)
	os.WriteFile(filepath.Join(workspace, "helper.ts"), []byte("export const help = 1;"), 0644)
	os.WriteFile(filepath.Join(workspace, "legacy.js"), []byte("module.exports = {};"), 0644)

	imports, err := resolver.ResolveImports([]string{"app.ts"})
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(imports)
	if len(imports) != 2 {
		t.Errorf("Expected 2 imports, got %d: %v", len(imports), imports)
	}
}

func TestResolveImportsPython(t *testing.T) {
	workspace := t.TempDir()
	resolver := NewImportResolver(workspace)

	// main.py imports lib
	mainContent := `from lib import core
import network`

	os.WriteFile(filepath.Join(workspace, "main.py"), []byte(mainContent), 0644)
	os.WriteFile(filepath.Join(workspace, "lib.py"), []byte("# lib"), 0644)
	os.WriteFile(filepath.Join(workspace, "network.py"), []byte("# net"), 0644)

	imports, err := resolver.ResolveImports([]string{"main.py"})
	if err != nil {
		t.Fatal(err)
	}

	if len(imports) != 2 {
		t.Errorf("Expected 2 imports, got %d", len(imports))
	}
}

func TestCircularImports(t *testing.T) {
	workspace := t.TempDir()
	resolver := NewImportResolver(workspace)

	// a.js <-> b.js
	os.WriteFile(filepath.Join(workspace, "a.js"), []byte("import './b'"), 0644)
	os.WriteFile(filepath.Join(workspace, "b.js"), []byte("import './a'"), 0644)

	imports, err := resolver.ResolveImports([]string{"a.js"})
	if err != nil {
		t.Fatal(err)
	}

	// Should just return b.js for a.js input
	if len(imports) != 1 {
		t.Errorf("Expected 1 import, got %d", len(imports))
	}
}

func TestDeduplication(t *testing.T) {
	workspace := t.TempDir()
	resolver := NewImportResolver(workspace)

	// main.ts imports util.ts
	// helper.ts imports util.ts
	// input: [main.ts, helper.ts] -> should return util.ts once

	os.WriteFile(filepath.Join(workspace, "main.ts"), []byte("import './util'"), 0644)
	os.WriteFile(filepath.Join(workspace, "helper.ts"), []byte("import './util'"), 0644)
	os.WriteFile(filepath.Join(workspace, "util.ts"), []byte(""), 0644)

	imports, err := resolver.ResolveImports([]string{"main.ts", "helper.ts"})
	if err != nil {
		t.Fatal(err)
	}

	if len(imports) != 1 {
		t.Errorf("Expected 1 unique import, got %d", len(imports))
	}
}
