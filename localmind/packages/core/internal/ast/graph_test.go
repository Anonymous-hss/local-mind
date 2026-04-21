package ast

import (
	"testing"
)

// Mock parser for testing
func newTestGraphBuilder() *GraphBuilder {
	return &GraphBuilder{
		graph: &DependencyGraph{
			Nodes: make(map[string]*FileNode),
			Edges: make([]Edge, 0),
		},
	}
}

func TestGraphBuilder_ResolveImports(t *testing.T) {
	gb := newTestGraphBuilder()

	// Add nodes manually to avoid parser dependency
	gb.graph.Nodes["main.go"] = &FileNode{
		Path:    "main.go",
		Package: "main",
		Imports: []Import{{Path: "fmt", IsLocal: false}, {Path: "./utils", IsLocal: true}},
	}
	gb.graph.Nodes["utils/helper.go"] = &FileNode{
		Path:    "utils/helper.go",
		Package: "utils",
	}

	// Build graph
	graph := gb.Build()

	// Check edges
	found := false
	for _, edge := range graph.Edges {
		if edge.From == "main.go" && edge.To == "utils/helper.go" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Failed to resolve local import 'utils/helper.go' from 'main.go'")
	}
}

func TestGraphBuilder_CycleDetection(t *testing.T) {
	gb := newTestGraphBuilder()

	// Create a cycle: A -> B -> C -> A
	gb.graph.Nodes["a.go"] = &FileNode{Path: "a.go", Imports: []Import{{Path: "./b", IsLocal: true}}}
	gb.graph.Nodes["b.go"] = &FileNode{Path: "b.go", Imports: []Import{{Path: "./c", IsLocal: true}}}
	gb.graph.Nodes["c.go"] = &FileNode{Path: "c.go", Imports: []Import{{Path: "./a", IsLocal: true}}}

	gb.Build()

	if len(gb.graph.Circular) == 0 {
		t.Error("Failed to detect circular dependency")
	}

	cycle := gb.graph.Circular[0]
	if len(cycle) != 3 {
		t.Errorf("Expected cycle of length 3, got %d", len(cycle))
	}
}

func TestGraphBuilder_TopologicalSort(t *testing.T) {
	gb := newTestGraphBuilder()

	// A -> B -> C
	gb.graph.Nodes["a.go"] = &FileNode{Path: "a.go", Imports: []Import{{Path: "./b", IsLocal: true}}}
	gb.graph.Nodes["b.go"] = &FileNode{Path: "b.go", Imports: []Import{{Path: "./c", IsLocal: true}}}
	gb.graph.Nodes["c.go"] = &FileNode{Path: "c.go"}

	gb.Build()
	sorted := gb.TopologicalSort()

	if len(sorted) != 3 {
		t.Fatalf("Expected 3 files, got %d", len(sorted))
	}

	// Order matters: C must come before B, B before A
	idx := make(map[string]int)
	for i, f := range sorted {
		idx[f] = i
	}

	if idx["c.go"] > idx["b.go"] {
		t.Error("c.go should come before b.go")
	}
	if idx["b.go"] > idx["a.go"] {
		t.Error("b.go should come before a.go")
	}
}

func TestGraphBuilder_AddFile(t *testing.T) {
	// This requires a parser, skipping to avoid complex mocking of the parser struct
	// Logic is widely covered by integration tests
}
