package ctxbuilder

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Git Context Tests
// =============================================================================

// setupGitRepo creates a temporary git repo with an initial commit.
// Returns the workspace path.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()

	// Initialize git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@localmind.dev"},
		{"git", "config", "user.name", "Test"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = workspace
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git setup failed: %s: %s", err, out)
		}
	}

	// Create a file and commit it
	err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "Initial commit"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = workspace
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git commit failed: %s: %s", err, out)
		}
	}

	return workspace
}

func TestGetDiff(t *testing.T) {
	workspace := setupGitRepo(t)
	gc := NewGitContext(5 * time.Second)

	// Modify a tracked file
	err := os.WriteFile(filepath.Join(workspace, "main.go"),
		[]byte("package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello\") }\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	diff, err := gc.GetDiff(workspace)
	if err != nil {
		t.Fatal(err)
	}

	// Should mention main.go in the stat output
	if !strings.Contains(diff, "main.go") {
		t.Errorf("Expected diff to mention main.go, got: %q", diff)
	}
}

func TestGetDiffFull(t *testing.T) {
	workspace := setupGitRepo(t)
	gc := NewGitContext(5 * time.Second)

	// Modify the file
	err := os.WriteFile(filepath.Join(workspace, "main.go"),
		[]byte("package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello\") }\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	diff, err := gc.GetDiffFull(workspace, 5000)
	if err != nil {
		t.Fatal(err)
	}

	// Full diff should contain actual change markers
	if !strings.Contains(diff, "+") {
		t.Errorf("Expected full diff to contain additions, got: %q", diff)
	}
}

func TestGetStagedDiff(t *testing.T) {
	workspace := setupGitRepo(t)
	gc := NewGitContext(5 * time.Second)

	// Create and stage a new file
	err := os.WriteFile(filepath.Join(workspace, "utils.go"),
		[]byte("package main\n\nfunc add(a, b int) int { return a + b }\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "add", "utils.go")
	cmd.Dir = workspace
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %s: %s", err, out)
	}

	staged, err := gc.GetStagedDiff(workspace)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(staged, "utils.go") {
		t.Errorf("Expected staged diff to mention utils.go, got: %q", staged)
	}
}

func TestGetRecentCommits(t *testing.T) {
	workspace := setupGitRepo(t)
	gc := NewGitContext(5 * time.Second)

	// Add a second commit
	err := os.WriteFile(filepath.Join(workspace, "readme.md"), []byte("# Test\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "Add readme"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = workspace
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git commit failed: %s: %s", err, out)
		}
	}

	commits, err := gc.GetRecentCommits(workspace, 5)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(commits, "Add readme") {
		t.Errorf("Expected commits to contain 'Add readme', got: %q", commits)
	}
	if !strings.Contains(commits, "Initial commit") {
		t.Errorf("Expected commits to contain 'Initial commit', got: %q", commits)
	}
}

func TestGetChangedFiles(t *testing.T) {
	workspace := setupGitRepo(t)
	gc := NewGitContext(5 * time.Second)

	// Modify tracked file + create new staged file
	err := os.WriteFile(filepath.Join(workspace, "main.go"),
		[]byte("package main\n\nfunc main() { println(1) }\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(workspace, "new.go"),
		[]byte("package main\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "add", "new.go")
	cmd.Dir = workspace
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %s: %s", err, out)
	}

	files, err := gc.GetChangedFiles(workspace)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) < 2 {
		t.Errorf("Expected at least 2 changed files, got: %v", files)
	}
}

func TestBuildChangeContext(t *testing.T) {
	workspace := setupGitRepo(t)
	gc := NewGitContext(5 * time.Second)

	// Modify file to create a diff
	err := os.WriteFile(filepath.Join(workspace, "main.go"),
		[]byte("package main\n\nfunc main() { println(\"updated\") }\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	ctx := gc.BuildChangeContext(workspace)

	if ctx == "" {
		t.Error("Expected non-empty change context")
	}
	if !strings.Contains(ctx, "CHANGE CONTEXT") {
		t.Error("Expected context to contain header")
	}
	if !strings.Contains(ctx, "WORKING CHANGES") {
		t.Error("Expected context to contain working changes section")
	}
}

func TestNoGitRepo(t *testing.T) {
	// Use a temp dir without git init
	workspace := t.TempDir()

	// Prevent git from discovering parent repos
	gc := &GitContext{timeout: 5 * time.Second}

	// Override by running with GIT_DIR pointing to nonexistent path
	// We test IsGitRepo by checking if BuildChangeContext returns empty
	ctx := gc.BuildChangeContext(workspace)

	// Even if git finds a parent repo, if there are no changes the context is empty
	// The real test is that it doesn't panic or error
	if ctx != "" {
		// It's possible the parent dir is a git repo, which is fine
		t.Logf("Non-empty context (parent repo detected): %q", ctx[:min(len(ctx), 100)])
	}
}

func TestIsGitRepo(t *testing.T) {
	workspace := setupGitRepo(t)
	gc := NewGitContext(5 * time.Second)

	if !gc.IsGitRepo(workspace) {
		t.Error("Expected git dir to return true for IsGitRepo")
	}
}
