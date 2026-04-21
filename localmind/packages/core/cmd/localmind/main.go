// Package main is the entry point for the LocalMind core engine.
//
// The core engine:
// - Listens on STDIO for IPC messages from VS Code extension
// - Routes tasks to appropriate engines (completion, suggestion, agent)
// - Manages latency budgets and cancellation
// - Interfaces with Ollama for model inference (Phase 2+)
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/localmind/core/pkg/orchestrator"
)

// Version is injected at build time by the release workflow:
//
//	go build -ldflags="-X main.Version=v0.1.0"
var Version = "dev"

func main() {
	log.Printf("LocalMind core starting (version=%s)", Version)

	// Create orchestrator with default config (stdin/stdout)
	orch := orchestrator.New(nil)

	// Set up context with signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v", sig)
		cancel()
	}()

	// Run orchestrator (blocks until shutdown)
	if err := orch.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Orchestrator error: %v", err)
	}
}
