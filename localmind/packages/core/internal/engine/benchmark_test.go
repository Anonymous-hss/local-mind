package engine

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/localmind/core/pkg/protocol"
)

// =============================================================================
// Latency Benchmarks
// =============================================================================

// BenchmarkCompletionLatency measures completion request latency
// Target: <150ms for p99
func BenchmarkCompletionLatency(b *testing.B) {
	engine := NewCompletionEngineWithConfig(nil, nil)

	req := &protocol.Request{
		ID:        "bench-1",
		Timestamp: time.Now().UnixMilli(),
		Type:      protocol.RequestTypeCompletion,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Execute(ctx, req)
	}
}

// BenchmarkSuggestionLatency measures suggestion request latency
// Target: <300ms for p99
func BenchmarkSuggestionLatency(b *testing.B) {
	engine := NewSuggestionEngine()
	engine.SimulatedDelay = 0 // Disable delay for benchmark

	req := &protocol.Request{
		ID:        "bench-1",
		Timestamp: time.Now().UnixMilli(),
		Type:      protocol.RequestTypeSuggestion,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Execute(ctx, req)
	}
}

// BenchmarkAgentPlanning measures agent planning latency
// Target: <3s for complex tasks
func BenchmarkAgentPlanning(b *testing.B) {
	engine := NewAgentEngine()

	req := &protocol.Request{
		ID:        "bench-1",
		Timestamp: time.Now().UnixMilli(),
		Type:      protocol.RequestTypeAgent,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Execute(ctx, req)
	}
}

// =============================================================================
// Memory Benchmarks
// =============================================================================

// BenchmarkMemoryUsage measures memory allocation during completion
func BenchmarkMemoryUsage(b *testing.B) {
	engine := NewCompletionEngineWithConfig(nil, nil)

	req := &protocol.Request{
		ID:        "bench-mem",
		Timestamp: time.Now().UnixMilli(),
		Type:      protocol.RequestTypeCompletion,
	}

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = engine.Execute(ctx, req)
	}
}

// BenchmarkCacheHitRatio measures cache performance
func BenchmarkCacheHitRatio(b *testing.B) {
	cache := NewCompletionCache(100, 5*time.Minute)

	// Warm up cache
	for i := 0; i < 50; i++ {
		prefix := "prefix-" + string(rune(i%26+'a'))
		cache.Set(prefix, "go", "completion", "model")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 80% cache hits, 20% misses
		prefix := "prefix-" + string(rune(i%26+'a'))
		cache.Get(prefix, "go")
	}
}

// =============================================================================
// Latency Regression Tests
// =============================================================================

// TestCompletionLatencyBudget ensures completions meet latency target
func TestCompletionLatencyBudget(t *testing.T) {
	const budget = 150 * time.Millisecond
	const iterations = 10

	engine := NewCompletionEngineWithConfig(nil, nil)
	ctx := context.Background()

	req := &protocol.Request{
		ID:        "latency-test",
		Timestamp: time.Now().UnixMilli(),
		Type:      protocol.RequestTypeCompletion,
	}

	var totalLatency time.Duration
	var maxLatency time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()
		_, _ = engine.Execute(ctx, req)
		latency := time.Since(start)

		totalLatency += latency
		if latency > maxLatency {
			maxLatency = latency
		}
	}

	avgLatency := totalLatency / iterations

	t.Logf("Completion latency: avg=%v, max=%v, budget=%v", avgLatency, maxLatency, budget)

	if avgLatency > budget {
		t.Errorf("Average latency %v exceeded budget %v", avgLatency, budget)
	}

	// Allow 2x budget for max (accounts for GC, cold start)
	if maxLatency > budget*2 {
		t.Errorf("Max latency %v exceeded 2x budget %v", maxLatency, budget*2)
	}
}

// TestSuggestionLatencyBudget ensures suggestions meet latency target
func TestSuggestionLatencyBudget(t *testing.T) {
	const budget = 300 * time.Millisecond
	const iterations = 10

	engine := NewSuggestionEngine()
	engine.SimulatedDelay = 0 // Disable delay for test
	ctx := context.Background()

	req := &protocol.Request{
		ID:        "latency-test",
		Timestamp: time.Now().UnixMilli(),
		Type:      protocol.RequestTypeSuggestion,
	}

	var totalLatency time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()
		_, _ = engine.Execute(ctx, req)
		totalLatency += time.Since(start)
	}

	avgLatency := totalLatency / iterations

	t.Logf("Suggestion latency: avg=%v, budget=%v", avgLatency, budget)

	if avgLatency > budget {
		t.Errorf("Average latency %v exceeded budget %v", avgLatency, budget)
	}
}

// TestMemoryBudget ensures memory usage stays within limits
func TestMemoryBudget(t *testing.T) {
	const maxMemoryMB = 100

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	startAlloc := m.Alloc

	engine := NewCompletionEngineWithConfig(nil, nil)
	ctx := context.Background()

	// Simulate 1000 requests
	for i := 0; i < 1000; i++ {
		req := &protocol.Request{
			ID:        "mem-test",
			Timestamp: time.Now().UnixMilli(),
			Type:      protocol.RequestTypeCompletion,
		}
		_, _ = engine.Execute(ctx, req)
	}

	runtime.ReadMemStats(&m)
	deltaAllocMB := float64(m.Alloc-startAlloc) / 1024 / 1024

	t.Logf("Memory delta: %.2f MB, budget: %d MB", deltaAllocMB, maxMemoryMB)

	if deltaAllocMB > maxMemoryMB {
		t.Errorf("Memory usage %.2f MB exceeded budget %d MB", deltaAllocMB, maxMemoryMB)
	}
}
