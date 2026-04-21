package ctxbuilder

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// =============================================================================
// Git Context — Extracts change context from git for the planner
// =============================================================================

// GitContext provides git-based change context for the agent planner.
// It shells out to the git CLI and extracts diffs, staged changes, and
// recent commit messages to inform the planning prompt.
type GitContext struct {
	timeout time.Duration
}

// NewGitContext creates a git context extractor with the given timeout.
func NewGitContext(timeout time.Duration) *GitContext {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &GitContext{timeout: timeout}
}

// GetDiff returns the working directory diff summary (unstaged changes).
func (gc *GitContext) GetDiff(workspace string) (string, error) {
	return gc.runGit(workspace, "diff", "--stat", "--no-color")
}

// GetDiffFull returns the full working directory diff (with content).
// Limited to maxChars to prevent blowing the token budget.
func (gc *GitContext) GetDiffFull(workspace string, maxChars int) (string, error) {
	output, err := gc.runGit(workspace, "diff", "--no-color", "-U3")
	if err != nil {
		return "", err
	}
	if len(output) > maxChars {
		output = output[:maxChars] + "\n... (diff truncated)"
	}
	return output, nil
}

// GetStagedDiff returns the staged (cached) diff summary.
func (gc *GitContext) GetStagedDiff(workspace string) (string, error) {
	return gc.runGit(workspace, "diff", "--cached", "--stat", "--no-color")
}

// GetRecentCommits returns the last N commit messages (one-line format).
func (gc *GitContext) GetRecentCommits(workspace string, n int) (string, error) {
	return gc.runGit(workspace, "log", "--oneline", fmt.Sprintf("-%d", n), "--no-color")
}

// GetChangedFiles returns a list of files with uncommitted changes.
func (gc *GitContext) GetChangedFiles(workspace string) ([]string, error) {
	output, err := gc.runGit(workspace, "diff", "--name-only", "--no-color")
	if err != nil {
		return nil, err
	}

	staged, err := gc.runGit(workspace, "diff", "--cached", "--name-only", "--no-color")
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var files []string

	for _, line := range strings.Split(output+"\n"+staged, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !seen[line] {
			seen[line] = true
			files = append(files, line)
		}
	}

	return files, nil
}

// BuildChangeContext creates a formatted prompt section from all git context.
// Returns an empty string if the workspace is not a git repo or has no changes.
func (gc *GitContext) BuildChangeContext(workspace string) string {
	var sections []string

	// Staged changes
	if staged, err := gc.GetStagedDiff(workspace); err == nil && strings.TrimSpace(staged) != "" {
		sections = append(sections, fmt.Sprintf("STAGED CHANGES:\n%s", staged))
	}

	// Working directory diff (summary + content snippet)
	if diff, err := gc.GetDiffFull(workspace, 2000); err == nil && strings.TrimSpace(diff) != "" {
		sections = append(sections, fmt.Sprintf("WORKING CHANGES:\n%s", diff))
	}

	// Recent commits for momentum context
	if commits, err := gc.GetRecentCommits(workspace, 5); err == nil && strings.TrimSpace(commits) != "" {
		sections = append(sections, fmt.Sprintf("RECENT COMMITS:\n%s", commits))
	}

	if len(sections) == 0 {
		return ""
	}

	return fmt.Sprintf("=== CHANGE CONTEXT ===\n%s\n=== END CHANGE CONTEXT ===", strings.Join(sections, "\n\n"))
}

// IsGitRepo checks if the workspace is inside a git repository.
func (gc *GitContext) IsGitRepo(workspace string) bool {
	_, err := gc.runGit(workspace, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

// runGit executes a git command in the workspace directory and returns stdout.
// Returns ("", nil) if git is not available or the directory is not a repo.
func (gc *GitContext) runGit(workspace string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gc.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workspace

	output, err := cmd.Output()
	if err != nil {
		// Git not found or not a repo is not an error for our purposes
		if _, ok := err.(*exec.ExitError); ok {
			return "", nil
		}
		return "", nil
	}

	return strings.TrimSpace(string(output)), nil
}
