// Package main implements a test runner that parses Go test2json output
// and produces a structured JSON report for automated analysis.
//
// Usage:
//
//	go run ./cmd/testrunner [-integration] [-e2e] [-timeout 60s] [-v]
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// ─── test2json event types ───────────────────────────────────────────────────

// TestEvent represents a single event from `go test -json`.
type TestEvent struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Elapsed float64   `json:"Elapsed"`
	Output  string    `json:"Output"`
}

// ─── report types ────────────────────────────────────────────────────────────

type TestResult struct {
	Name     string  `json:"name"`
	Passed   bool    `json:"passed"`
	Skipped  bool    `json:"skipped,omitempty"`
	Duration string  `json:"duration"`
	Output   string  `json:"output,omitempty"` // only populated on failure
	Elapsed  float64 `json:"elapsed_seconds"`
}

type PackageResult struct {
	Name     string       `json:"name"`
	Passed   bool         `json:"passed"`
	Duration string       `json:"duration"`
	Elapsed  float64      `json:"elapsed_seconds"`
	Tests    []TestResult `json:"tests"`
	Output   string       `json:"output,omitempty"` // build errors, etc.
}

type Summary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

type Report struct {
	Timestamp   string          `json:"timestamp"`
	Duration    string          `json:"duration"`
	Summary     Summary         `json:"summary"`
	Packages    []PackageResult `json:"packages"`
	BuildErrors []string        `json:"build_errors,omitempty"`
}

// ─── internal state ──────────────────────────────────────────────────────────

type testState struct {
	output  []string
	elapsed float64
	action  string // last action: "pass", "fail", "skip"
}

type pkgState struct {
	tests   map[string]*testState
	output  []string
	elapsed float64
	action  string
}

func main() {
	// Parse flags
	integration := false
	e2e := false
	timeout := "60s"
	verbose := false

	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-integration":
			integration = true
		case "-e2e":
			e2e = true
		case "-timeout":
			if i+1 < len(os.Args) {
				timeout = os.Args[i+1]
				i++
			}
		case "-v":
			verbose = true
		}
	}

	start := time.Now()

	// Build the list of packages to test
	targets := []string{"./internal/..."}
	if integration {
		targets = append(targets, "./test/integration/...")
	}
	if e2e {
		targets = append(targets, "./test/e2e/...")
	}

	report := Report{
		Timestamp: start.Format(time.RFC3339),
	}

	// Track all package states
	packages := map[string]*pkgState{}

	for _, target := range targets {
		args := []string{"test", "-json", "-tags", "nocgo", "-count=1", "-timeout", timeout}
		if verbose {
			args = append(args, "-v")
		}
		args = append(args, target)

		if verbose {
			fmt.Fprintf(os.Stderr, "[testrunner] running: go %s\n", strings.Join(args, " "))
		}

		cmd := exec.Command("go", args...)
		cmd.Dir = "." // run from the core directory
		cmd.Stderr = os.Stderr

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			report.BuildErrors = append(report.BuildErrors, fmt.Sprintf("failed to create stdout pipe: %v", err))
			continue
		}

		if err := cmd.Start(); err != nil {
			report.BuildErrors = append(report.BuildErrors, fmt.Sprintf("failed to start go test: %v", err))
			continue
		}

		scanner := bufio.NewScanner(stdout)
		// Increase scanner buffer for large output lines
		scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()

			var event TestEvent
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				// Non-JSON output (e.g., build errors)
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					report.BuildErrors = append(report.BuildErrors, trimmed)
				}
				continue
			}

			// Get or create package state
			pkg, ok := packages[event.Package]
			if !ok {
				pkg = &pkgState{tests: map[string]*testState{}}
				packages[event.Package] = pkg
			}

			if event.Test == "" {
				// Package-level event
				switch event.Action {
				case "output":
					pkg.output = append(pkg.output, event.Output)
				case "pass", "fail", "skip":
					pkg.action = event.Action
					pkg.elapsed = event.Elapsed
				}
			} else {
				// Test-level event
				ts, ok := pkg.tests[event.Test]
				if !ok {
					ts = &testState{}
					pkg.tests[event.Test] = ts
				}

				switch event.Action {
				case "output":
					ts.output = append(ts.output, event.Output)
				case "pass", "fail", "skip":
					ts.action = event.Action
					ts.elapsed = event.Elapsed
				}
			}
		}

		if err := cmd.Wait(); err != nil {
			// go test exits non-zero on test failure — that's fine,
			// the results are already captured in the events
			if verbose {
				fmt.Fprintf(os.Stderr, "[testrunner] go test exited: %v\n", err)
			}
		}
	}

	// Build report from state
	for pkgName, pkg := range packages {
		pr := PackageResult{
			Name:     pkgName,
			Passed:   pkg.action == "pass",
			Elapsed:  pkg.elapsed,
			Duration: formatDuration(pkg.elapsed),
		}

		// If package failed to build, capture the package output
		if pkg.action == "fail" && len(pkg.tests) == 0 {
			pr.Output = joinOutput(pkg.output)
		}

		// Only include non-subtests (subtests contain "/")
		for testName, ts := range pkg.tests {
			if strings.Contains(testName, "/") {
				continue // skip subtests, they're part of parent
			}

			tr := TestResult{
				Name:     testName,
				Passed:   ts.action == "pass",
				Skipped:  ts.action == "skip",
				Elapsed:  ts.elapsed,
				Duration: formatDuration(ts.elapsed),
			}

			// On failure, include the test's output for diagnosis
			if ts.action == "fail" {
				tr.Output = joinOutput(ts.output)
			}

			pr.Tests = append(pr.Tests, tr)

			report.Summary.Total++
			switch ts.action {
			case "pass":
				report.Summary.Passed++
			case "fail":
				report.Summary.Failed++
			case "skip":
				report.Summary.Skipped++
			}
		}

		// Sort tests by name for stable output
		sort.Slice(pr.Tests, func(i, j int) bool {
			return pr.Tests[i].Name < pr.Tests[j].Name
		})

		report.Packages = append(report.Packages, pr)
	}

	// Sort packages by name
	sort.Slice(report.Packages, func(i, j int) bool {
		return report.Packages[i].Name < report.Packages[j].Name
	})

	report.Duration = time.Since(start).Round(time.Millisecond).String()

	// Output JSON report
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode report: %v\n", err)
		os.Exit(2)
	}

	if report.Summary.Failed > 0 || len(report.BuildErrors) > 0 {
		os.Exit(1)
	}
}

func formatDuration(seconds float64) string {
	if seconds < 0.001 {
		return "<1ms"
	}
	if seconds < 1 {
		return fmt.Sprintf("%.0fms", seconds*1000)
	}
	return fmt.Sprintf("%.2fs", seconds)
}

func joinOutput(lines []string) string {
	var sb strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed != "" {
			sb.WriteString(trimmed)
			sb.WriteByte('\n')
		}
	}
	result := sb.String()
	// Truncate very long output
	if len(result) > 2000 {
		return result[:2000] + "\n... [truncated]"
	}
	return result
}
