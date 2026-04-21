package ast

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// BoundaryDetector detects code boundaries and architectural layers
type BoundaryDetector struct {
	// Layer patterns (regex -> layer type)
	layerPatterns map[*regexp.Regexp]LayerType
}

// NewBoundaryDetector creates a new boundary detector
func NewBoundaryDetector() *BoundaryDetector {
	bd := &BoundaryDetector{
		layerPatterns: make(map[*regexp.Regexp]LayerType),
	}

	// Common layer patterns
	patterns := map[string]LayerType{
		// API layer
		`(?i)^api$`:           LayerAPI,
		`(?i)^handlers?$`:     LayerAPI,
		`(?i)^routes?$`:       LayerAPI,
		`(?i)^endpoints?$`:    LayerAPI,
		`(?i)^rest$`:          LayerAPI,
		`(?i)^graphql$`:       LayerAPI,
		`(?i)^grpc$`:          LayerAPI,

		// Controller layer
		`(?i)^controllers?$`:  LayerController,
		`(?i)^ctrl$`:          LayerController,

		// Service layer
		`(?i)^services?$`:     LayerService,
		`(?i)^usecases?$`:     LayerService,
		`(?i)^business$`:      LayerService,
		`(?i)^domain$`:        LayerService,
		`(?i)^core$`:          LayerService,

		// Repository layer
		`(?i)^repo(sitor(y|ies))?$`:  LayerRepository,
		`(?i)^dal$`:                   LayerRepository,
		`(?i)^data$`:                  LayerRepository,
		`(?i)^storage$`:               LayerRepository,
		`(?i)^database$`:              LayerRepository,
		`(?i)^db$`:                    LayerRepository,
		`(?i)^store$`:                 LayerRepository,

		// Model layer
		`(?i)^models?$`:       LayerModel,
		`(?i)^entities$`:      LayerModel,
		`(?i)^types?$`:        LayerModel,
		`(?i)^schema$`:        LayerModel,
		`(?i)^dto$`:           LayerModel,

		// Utility layer
		`(?i)^utils?$`:        LayerUtil,
		`(?i)^helpers?$`:      LayerUtil,
		`(?i)^lib$`:           LayerUtil,
		`(?i)^common$`:        LayerUtil,
		`(?i)^shared$`:        LayerUtil,
		`(?i)^pkg$`:           LayerUtil,
		`(?i)^internal$`:      LayerUtil,

		// Test layer
		`(?i)^tests?$`:        LayerTest,
		`(?i)^__tests__$`:     LayerTest,
		`(?i)^spec$`:          LayerTest,
		`(?i)^e2e$`:           LayerTest,

		// Config layer
		`(?i)^configs?$`:      LayerConfig,
		`(?i)^settings?$`:     LayerConfig,
		`(?i)^env$`:           LayerConfig,
	}

	for pattern, layer := range patterns {
		bd.layerPatterns[regexp.MustCompile(pattern)] = layer
	}

	return bd
}

// DetectLayer detects the architectural layer from a folder name
func (bd *BoundaryDetector) DetectLayer(folderName string) LayerType {
	for pattern, layer := range bd.layerPatterns {
		if pattern.MatchString(folderName) {
			return layer
		}
	}
	return LayerUnknown
}

// AnalyzeDirectory analyzes a directory and returns boundaries
func (bd *BoundaryDetector) AnalyzeDirectory(rootPath string) ([]Boundary, error) {
	var boundaries []Boundary

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if !info.IsDir() {
			return nil
		}

		// Skip hidden directories and common non-code dirs
		name := info.Name()
		if strings.HasPrefix(name, ".") || 
			name == "node_modules" || 
			name == "vendor" ||
			name == "__pycache__" ||
			name == ".git" {
			return filepath.SkipDir
		}

		relPath, _ := filepath.Rel(rootPath, path)
		if relPath == "." {
			return nil
		}

		layer := bd.DetectLayer(name)
		
		boundary := Boundary{
			Path:  path,
			Name:  name,
			Type:  bd.detectBoundaryType(path, rootPath),
			Layer: layer,
			Files: bd.listFiles(path),
		}

		if boundary.Type != "" || layer != LayerUnknown {
			boundaries = append(boundaries, boundary)
		}

		return nil
	})

	return boundaries, err
}

// detectBoundaryType determines if a directory is a package/module
func (bd *BoundaryDetector) detectBoundaryType(path, rootPath string) BoundaryType {
	// Check for Go package (has .go files)
	goFiles, _ := filepath.Glob(filepath.Join(path, "*.go"))
	if len(goFiles) > 0 {
		return BoundaryTypePackage
	}

	// Check for TypeScript/JS module (has package.json or index file)
	if _, err := os.Stat(filepath.Join(path, "package.json")); err == nil {
		return BoundaryTypeModule
	}
	if _, err := os.Stat(filepath.Join(path, "index.ts")); err == nil {
		return BoundaryTypeModule
	}
	if _, err := os.Stat(filepath.Join(path, "index.js")); err == nil {
		return BoundaryTypeModule
	}

	// Check for Python package (has __init__.py)
	if _, err := os.Stat(filepath.Join(path, "__init__.py")); err == nil {
		return BoundaryTypePackage
	}

	return ""
}

// listFiles lists code files in a directory (non-recursive)
func (bd *BoundaryDetector) listFiles(path string) []string {
	var files []string

	entries, err := os.ReadDir(path)
	if err != nil {
		return files
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		switch ext {
		case ".go", ".ts", ".tsx", ".js", ".jsx", ".py":
			files = append(files, filepath.Join(path, entry.Name()))
		}
	}

	return files
}

// InferProjectStructure analyzes a project and returns high-level structure info
func (bd *BoundaryDetector) InferProjectStructure(rootPath string) (*ProjectStructure, error) {
	boundaries, err := bd.AnalyzeDirectory(rootPath)
	if err != nil {
		return nil, err
	}

	ps := &ProjectStructure{
		RootPath:   rootPath,
		Boundaries: boundaries,
		Layers:     make(map[LayerType][]string),
	}

	// Group boundaries by layer
	for _, b := range boundaries {
		if b.Layer != LayerUnknown {
			ps.Layers[b.Layer] = append(ps.Layers[b.Layer], b.Path)
		}
	}

	// Detect primary language
	ps.PrimaryLanguage = bd.detectPrimaryLanguage(rootPath)

	// Detect framework conventions
	ps.Framework = bd.detectFramework(rootPath)

	return ps, nil
}

// ProjectStructure represents the analyzed structure of a project
type ProjectStructure struct {
	RootPath        string
	PrimaryLanguage Language
	Framework       string
	Boundaries      []Boundary
	Layers          map[LayerType][]string
}

// detectPrimaryLanguage detects the main language of a project
func (bd *BoundaryDetector) detectPrimaryLanguage(rootPath string) Language {
	counts := make(map[Language]int)

	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		
		// Skip common non-source directories
		if strings.Contains(path, "node_modules") ||
			strings.Contains(path, "vendor") ||
			strings.Contains(path, ".git") {
			return nil
		}

		lang := LanguageFromFilename(info.Name())
		if lang != LanguageUnknown {
			counts[lang]++
		}
		return nil
	})

	var maxLang Language
	var maxCount int
	for lang, count := range counts {
		if count > maxCount {
			maxCount = count
			maxLang = lang
		}
	}

	return maxLang
}

// detectFramework attempts to detect the project framework
func (bd *BoundaryDetector) detectFramework(rootPath string) string {
	// Check for common framework indicators
	frameworkFiles := map[string]string{
		"go.mod":         "go",
		"package.json":   "node",
		"requirements.txt": "python",
		"Cargo.toml":     "rust",
		"pom.xml":        "java/maven",
		"build.gradle":   "java/gradle",
	}

	for file, framework := range frameworkFiles {
		if _, err := os.Stat(filepath.Join(rootPath, file)); err == nil {
			// Further refinement for node
			if framework == "node" {
				if pkg, err := os.ReadFile(filepath.Join(rootPath, file)); err == nil {
					content := string(pkg)
					if strings.Contains(content, "\"react\"") {
						return "react"
					}
					if strings.Contains(content, "\"next\"") {
						return "nextjs"
					}
					if strings.Contains(content, "\"vue\"") {
						return "vue"
					}
					if strings.Contains(content, "\"@angular/core\"") {
						return "angular"
					}
					if strings.Contains(content, "\"express\"") {
						return "express"
					}
					if strings.Contains(content, "\"fastify\"") {
						return "fastify"
					}
				}
			}
			return framework
		}
	}

	return "unknown"
}
