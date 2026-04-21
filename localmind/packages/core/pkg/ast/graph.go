package ast

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
)

// GraphBuilder builds dependency graphs from parsed files
type GraphBuilder struct {
	parser *Parser
	graph  *DependencyGraph
}

// NewGraphBuilder creates a new graph builder
func NewGraphBuilder(parser *Parser) *GraphBuilder {
	return &GraphBuilder{
		parser: parser,
		graph: &DependencyGraph{
			Nodes: make(map[string]*FileNode),
			Edges: make([]Edge, 0),
		},
	}
}

// AddFile adds a file to the dependency graph
func (b *GraphBuilder) AddFile(ctx context.Context, filepath string, content []byte) error {
	result, err := b.parser.Parse(ctx, filepath, content)
	if err != nil {
		return err
	}

	node := &FileNode{
		Path:     filepath,
		Package:  result.Package,
		Language: result.Language,
		Imports:  result.Imports,
		Exports:  result.Exports,
		Symbols:  result.Symbols,
	}

	b.graph.Nodes[filepath] = node
	return nil
}

// Build builds the complete dependency graph
// Call this after all files have been added
func (b *GraphBuilder) Build() *DependencyGraph {
	// Build edges from imports
	for fromPath, node := range b.graph.Nodes {
		for _, imp := range node.Imports {
			// Try to resolve import to a file in our graph
			toPath := b.resolveImport(fromPath, imp)
			if toPath != "" && toPath != fromPath {
				b.graph.Edges = append(b.graph.Edges, Edge{
					From: fromPath,
					To:   toPath,
					Kind: "import",
				})
			}
		}
	}

	// Detect circular dependencies
	b.graph.Circular = b.detectCircular()

	return b.graph
}

// resolveImport attempts to resolve an import path to a file in the graph
func (b *GraphBuilder) resolveImport(fromPath string, imp Import) string {
	// For relative imports, resolve against the source file
	if imp.IsLocal {
		dir := filepath.Dir(fromPath)
		resolved := filepath.Join(dir, imp.Path)

		// Try various extensions
		extensions := []string{"", ".go", ".ts", ".tsx", ".js", ".jsx", ".py"}
		for _, ext := range extensions {
			candidate := resolved + ext
			if _, exists := b.graph.Nodes[candidate]; exists {
				return candidate
			}
		}

		// Try index files
		indexFiles := []string{
			filepath.Join(resolved, "index.ts"),
			filepath.Join(resolved, "index.js"),
			filepath.Join(resolved, "index.tsx"),
			filepath.Join(resolved, "__init__.py"),
		}
		for _, idx := range indexFiles {
			if _, exists := b.graph.Nodes[idx]; exists {
				return idx
			}
		}
	}

	// For Go imports, match by package
	for path, node := range b.graph.Nodes {
		if node.Package != "" && strings.HasSuffix(imp.Path, "/"+node.Package) {
			return path
		}
	}

	return ""
}

// detectCircular detects circular dependencies using DFS
func (b *GraphBuilder) detectCircular() [][]string {
	var cycles [][]string

	// Build adjacency list
	adj := make(map[string][]string)
	for _, edge := range b.graph.Edges {
		adj[edge.From] = append(adj[edge.From], edge.To)
	}

	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	path := make([]string, 0)

	var dfs func(node string) bool
	dfs = func(node string) bool {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		for _, neighbor := range adj[node] {
			if !visited[neighbor] {
				if dfs(neighbor) {
					return true
				}
			} else if recStack[neighbor] {
				// Found cycle - extract it
				cycleStart := -1
				for i, n := range path {
					if n == neighbor {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cycle := make([]string, len(path)-cycleStart)
					copy(cycle, path[cycleStart:])
					cycles = append(cycles, cycle)
				}
			}
		}

		path = path[:len(path)-1]
		recStack[node] = false
		return false
	}

	for node := range b.graph.Nodes {
		if !visited[node] {
			dfs(node)
		}
	}

	return cycles
}

// GetDependencies returns all dependencies of a file
func (b *GraphBuilder) GetDependencies(filepath string) []string {
	var deps []string
	for _, edge := range b.graph.Edges {
		if edge.From == filepath {
			deps = append(deps, edge.To)
		}
	}
	return deps
}

// GetDependents returns all files that depend on this file
func (b *GraphBuilder) GetDependents(filepath string) []string {
	var deps []string
	for _, edge := range b.graph.Edges {
		if edge.To == filepath {
			deps = append(deps, edge.From)
		}
	}
	return deps
}

// GetDepthMap returns the dependency depth of each file
// Files with no dependencies have depth 0
func (b *GraphBuilder) GetDepthMap() map[string]int {
	depths := make(map[string]int)

	// Build reverse adjacency list
	inDegree := make(map[string]int)
	adj := make(map[string][]string)

	for _, node := range b.graph.Nodes {
		inDegree[node.Path] = 0
	}

	for _, edge := range b.graph.Edges {
		adj[edge.From] = append(adj[edge.From], edge.To)
		inDegree[edge.To]++
	}

	// Topological sort with levels
	queue := make([]string, 0)
	for path, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, path)
			depths[path] = 0
		}
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, next := range adj[current] {
			inDegree[next]--
			if depths[next] < depths[current]+1 {
				depths[next] = depths[current] + 1
			}
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	return depths
}

// TopologicalSort returns files in dependency order
func (b *GraphBuilder) TopologicalSort() []string {
	depths := b.GetDepthMap()

	files := make([]string, 0, len(depths))
	for path := range depths {
		files = append(files, path)
	}

	sort.Slice(files, func(i, j int) bool {
		return depths[files[i]] > depths[files[j]]
	})

	return files
}
