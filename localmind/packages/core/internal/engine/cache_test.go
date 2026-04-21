package engine

import (
	"testing"
	"time"
)

func TestCompletionCache_SetGet(t *testing.T) {
	cache := NewCompletionCache(10, 30*time.Second)

	cache.Set("func main() {", "go", "fmt.Println()", "test-model")

	entry, ok := cache.Get("func main() {", "go")
	if !ok {
		t.Fatal("Expected cache hit")
	}

	if entry.Completion != "fmt.Println()" {
		t.Errorf("Completion = %q, want %q", entry.Completion, "fmt.Println()")
	}
}

func TestCompletionCache_Miss(t *testing.T) {
	cache := NewCompletionCache(10, 30*time.Second)

	_, ok := cache.Get("nonexistent", "go")
	if ok {
		t.Error("Expected cache miss")
	}
}

func TestCompletionCache_TTL(t *testing.T) {
	cache := NewCompletionCache(10, 50*time.Millisecond)

	cache.Set("prefix", "go", "completion", "model")

	// Should hit immediately
	_, ok := cache.Get("prefix", "go")
	if !ok {
		t.Fatal("Expected cache hit before TTL")
	}

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	_, ok = cache.Get("prefix", "go")
	if ok {
		t.Error("Expected cache miss after TTL")
	}
}

func TestCompletionCache_LRU(t *testing.T) {
	cache := NewCompletionCache(3, 30*time.Second)

	// Fill cache
	cache.Set("a", "go", "1", "model")
	cache.Set("b", "go", "2", "model")
	cache.Set("c", "go", "3", "model")

	// Access 'a' to make it recently used
	cache.Get("a", "go")

	// Add new entry - should evict 'b' (oldest accessed)
	cache.Set("d", "go", "4", "model")

	// 'a' should still exist
	if _, ok := cache.Get("a", "go"); !ok {
		t.Error("'a' should still be in cache")
	}

	// 'd' should exist
	if _, ok := cache.Get("d", "go"); !ok {
		t.Error("'d' should be in cache")
	}

	if cache.Size() > 3 {
		t.Errorf("Cache size = %d, want <= 3", cache.Size())
	}
}

func TestCompletionCache_Language(t *testing.T) {
	cache := NewCompletionCache(10, 30*time.Second)

	cache.Set("func main", "go", "go completion", "model")
	cache.Set("func main", "python", "python completion", "model")

	goEntry, ok := cache.Get("func main", "go")
	if !ok {
		t.Fatal("Expected cache hit for go")
	}
	if goEntry.Completion != "go completion" {
		t.Errorf("Go completion = %q", goEntry.Completion)
	}

	pyEntry, ok := cache.Get("func main", "python")
	if !ok {
		t.Fatal("Expected cache hit for python")
	}
	if pyEntry.Completion != "python completion" {
		t.Errorf("Python completion = %q", pyEntry.Completion)
	}
}

func TestCompletionCache_Clear(t *testing.T) {
	cache := NewCompletionCache(10, 30*time.Second)

	cache.Set("a", "go", "1", "model")
	cache.Set("b", "go", "2", "model")

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Cache size after clear = %d, want 0", cache.Size())
	}
}
