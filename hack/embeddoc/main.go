// embeddoc is a tool that extracts code snippets from source files and embeds them into documentation.
//
// Source files use markers like:
//
//	// <embed id="snippet-name">
//	code here...
//	// </embed>
//
// Documentation files use markers like:
//
//	<!-- <embed id="snippet-name" inject_from="code"> -->
//	(content will be replaced)
//	<!-- </embed> -->
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the embeddoc configuration file structure.
type Config struct {
	Walk struct {
		Includes        []string `yaml:"includes"`
		Excludes        []string `yaml:"excludes"`
		ExcludeFilePath []string `yaml:"exclude_file_path"`
	} `yaml:"walk"`
}

// Snippet represents an extracted code snippet.
type Snippet struct {
	ID      string
	Content string
	Source  string // source file path
}

var (
	// Source file markers (Go comments)
	sourceStartRe = regexp.MustCompile(`^\s*//\s*<embed\s+id="([^"]+)">\s*$`)
	sourceEndRe   = regexp.MustCompile(`^\s*//\s*</embed>\s*$`)

	// Documentation markers (HTML comments)
	docStartRe = regexp.MustCompile(`<!--\s*<embed\s+id="([^"]+)"\s+inject_from="code">\s*-->`)
	docEndRe   = regexp.MustCompile(`<!--\s*</embed>\s*-->`)
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration
	config, err := loadConfig("embeddoc.yaml")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Extract snippets from source files
	snippets := make(map[string]*Snippet)
	for _, include := range config.Walk.Includes {
		if err := extractSnippets(include, config.Walk.Excludes, snippets); err != nil {
			return fmt.Errorf("extracting snippets from %s: %w", include, err)
		}
	}

	if len(snippets) == 0 {
		fmt.Println("No snippets found")
		return nil
	}

	fmt.Printf("Found %d snippet(s):\n", len(snippets))
	for id, s := range snippets {
		fmt.Printf("  - %s (from %s)\n", id, s.Source)
	}

	// Inject snippets into documentation files
	var injected int
	for _, include := range config.Walk.Includes {
		n, err := injectSnippets(include, config.Walk.Excludes, snippets)
		if err != nil {
			return fmt.Errorf("injecting snippets in %s: %w", include, err)
		}
		injected += n
	}

	fmt.Printf("Injected snippets into %d location(s)\n", injected)
	return nil
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func shouldExclude(path string, excludes []string) bool {
	for _, exclude := range excludes {
		if strings.Contains(path, exclude) {
			return true
		}
	}
	return false
}

func extractSnippets(root string, excludes []string, snippets map[string]*Snippet) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if shouldExclude(path, excludes) {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process Go files for snippet extraction
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		if shouldExclude(path, excludes) {
			return nil
		}

		return extractSnippetsFromFile(path, snippets)
	})
}

func extractSnippetsFromFile(path string, snippets map[string]*Snippet) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var (
		currentID      string
		currentContent strings.Builder
		inSnippet      bool
	)

	for scanner.Scan() {
		line := scanner.Text()

		if !inSnippet {
			if matches := sourceStartRe.FindStringSubmatch(line); matches != nil {
				currentID = matches[1]
				inSnippet = true
				currentContent.Reset()
			}
		} else {
			if sourceEndRe.MatchString(line) {
				content := currentContent.String()
				// Trim trailing newline if present
				content = strings.TrimSuffix(content, "\n")

				if existing, ok := snippets[currentID]; ok {
					return fmt.Errorf("duplicate snippet ID %q: found in %s and %s", currentID, existing.Source, path)
				}

				snippets[currentID] = &Snippet{
					ID:      currentID,
					Content: content,
					Source:  path,
				}
				inSnippet = false
				currentID = ""
			} else {
				currentContent.WriteString(line)
				currentContent.WriteString("\n")
			}
		}
	}

	if inSnippet {
		return fmt.Errorf("unclosed snippet %q in %s", currentID, path)
	}

	return scanner.Err()
}

func injectSnippets(root string, excludes []string, snippets map[string]*Snippet) (int, error) {
	var count int

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if shouldExclude(path, excludes) {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process Markdown files for injection
		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		if shouldExclude(path, excludes) {
			return nil
		}

		n, err := injectSnippetsInFile(path, snippets)
		if err != nil {
			return err
		}
		count += n
		return nil
	})

	return count, err
}

func injectSnippetsInFile(path string, snippets map[string]*Snippet) (int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(content), "\n")
	var (
		result    strings.Builder
		inBlock   bool
		currentID string
		count     int
	)

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if !inBlock {
			result.WriteString(line)
			result.WriteString("\n")

			if matches := docStartRe.FindStringSubmatch(line); matches != nil {
				currentID = matches[1]
				inBlock = true

				// Look up snippet and inject
				snippet, ok := snippets[currentID]
				if !ok {
					return 0, fmt.Errorf("snippet %q not found (referenced in %s)", currentID, path)
				}

				result.WriteString(snippet.Content)
				result.WriteString("\n")
				count++
			}
		} else {
			// Skip lines until we find the end marker
			if docEndRe.MatchString(line) {
				result.WriteString(line)
				result.WriteString("\n")
				inBlock = false
				currentID = ""
			}
			// Skip any existing content between markers
		}
	}

	if inBlock {
		return 0, fmt.Errorf("unclosed embed block %q in %s", currentID, path)
	}

	// Remove trailing newline that we added
	output := result.String()
	output = strings.TrimSuffix(output, "\n")

	// Only write if content changed
	if output != string(content) {
		if err := os.WriteFile(path, []byte(output), 0644); err != nil {
			return 0, err
		}
		fmt.Printf("Updated %s\n", path)
	}

	return count, nil
}
