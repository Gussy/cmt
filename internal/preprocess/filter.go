package preprocess

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Options configures the preprocessing behavior.
type Options struct {
	// MaxTokens is the maximum number of tokens to include in the diff.
	// Default is 16384 if not specified.
	MaxTokens int

	// FilterBinary determines whether to filter out binary files.
	// Default is true.
	FilterBinary bool

	// FilterMinified determines whether to filter out minified files.
	// Default is true.
	FilterMinified bool

	// FilterGenerated determines whether to filter out generated/lock files.
	// Default is true.
	FilterGenerated bool
}

// Default returns default preprocessing options.
func DefaultOptions() Options {
	return Options{
		MaxTokens:       16384,
		FilterBinary:    true,
		FilterMinified:  true,
		FilterGenerated: true,
	}
}

// binaryExtensions are file extensions typically associated with binary files.
var binaryExtensions = map[string]bool{
	// Images
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".bmp":  true,
	".ico":  true,
	".svg":  true,
	".webp": true,
	// Documents
	".pdf":  true,
	".doc":  true,
	".docx": true,
	".xls":  true,
	".xlsx": true,
	".ppt":  true,
	".pptx": true,
	// Archives
	".zip": true,
	".tar": true,
	".gz":  true,
	".bz2": true,
	".rar": true,
	".7z":  true,
	// Executables
	".exe":   true,
	".dll":   true,
	".so":    true,
	".dylib": true,
	".app":   true,
	".deb":   true,
	".rpm":   true,
	".dmg":   true,
	".pkg":   true,
	// Media
	".mp3":  true,
	".mp4":  true,
	".avi":  true,
	".mov":  true,
	".wmv":  true,
	".flv":  true,
	".wav":  true,
	".flac": true,
	// Fonts
	".ttf":   true,
	".otf":   true,
	".woff":  true,
	".woff2": true,
	".eot":   true,
	// Database
	".db":     true,
	".sqlite": true,
}

// generatedFiles are filenames that are typically generated or lock files.
var generatedFiles = map[string]bool{
	// Lock files
	"package-lock.json": true,
	"yarn.lock":         true,
	"pnpm-lock.yaml":    true,
	"go.sum":            true,
	"Gemfile.lock":      true,
	"Cargo.lock":        true,
	"poetry.lock":       true,
	"composer.lock":     true,
	"Podfile.lock":      true,
	// System files
	".DS_Store":   true,
	"Thumbs.db":   true,
	"desktop.ini": true,
}

// Process preprocesses a git diff according to the provided options.
// It filters out binary files, minified files, and generated files,
// and truncates the diff if it exceeds the token limit.
func Process(diff string, opts Options) string {
	// Use defaults if options are zero
	if opts.MaxTokens == 0 {
		opts.MaxTokens = 16384
	}

	lines := strings.Split(diff, "\n")
	var result []string
	var currentFile string
	var skipCurrentFile bool
	tokensUsed := 0
	truncated := false

	for _, line := range lines {
		// Check if we've exceeded token limit
		lineTokens := estimateTokens(line)
		if tokensUsed+lineTokens > opts.MaxTokens {
			truncated = true
			break
		}

		// Check for file header
		if strings.HasPrefix(line, "diff --git") {
			currentFile = extractFilePath(line)
			skipCurrentFile = shouldSkipFile(currentFile, opts)

			// Always include the header so the AI knows about all changed files.
			result = append(result, line)
			tokensUsed += lineTokens

			if skipCurrentFile {
				// Add a note about why the content was filtered.
				note := fmt.Sprintf("(%s)", fileFilterReason(currentFile, opts))
				result = append(result, note)
				tokensUsed += estimateTokens(note)
			}
			continue
		}

		// Skip content lines for filtered files, but include file
		// metadata lines (deleted/new file mode, rename info) so
		// the AI knows the nature of the change.
		if skipCurrentFile {
			if isFileMetadataLine(line) {
				result = append(result, line)
				tokensUsed += lineTokens
			}
			continue
		}

		// Check for binary file indicator
		if strings.Contains(line, "Binary files") && strings.Contains(line, "differ") {
			if opts.FilterBinary {
				// Replace with a simple indicator
				result = append(result, "Binary file (content omitted)")
				tokensUsed += estimateTokens("Binary file (content omitted)")
				skipCurrentFile = true
				continue
			}
		}

		// Add the line to result
		result = append(result, line)
		tokensUsed += lineTokens
	}

	// Add truncation indicator if needed
	if truncated {
		result = append(result, "", "... (diff truncated due to token limit)")
	}

	return strings.Join(result, "\n")
}

// extractFilePath extracts the file path from a diff header line.
// Example: "diff --git a/path/to/file.go b/path/to/file.go" -> "path/to/file.go"
func extractFilePath(diffLine string) string {
	// Look for 'a/' prefix first
	if idx := strings.Index(diffLine, " a/"); idx != -1 {
		start := idx + 3 // Skip " a/"
		// Find the end - look for " b/"
		if endIdx := strings.Index(diffLine[start:], " b/"); endIdx != -1 {
			return diffLine[start : start+endIdx]
		}
		// If no " b/" found, take everything after "a/"
		remaining := diffLine[start:]
		// Split by space if there's one
		if spaceIdx := strings.Index(remaining, " "); spaceIdx != -1 {
			return remaining[:spaceIdx]
		}
		return remaining
	}

	// Fallback: look for 'b/' prefix
	if idx := strings.Index(diffLine, " b/"); idx != -1 {
		start := idx + 3 // Skip " b/"
		remaining := diffLine[start:]
		// Take until end or next space that's not part of the path
		if spaceIdx := strings.Index(remaining, " "); spaceIdx != -1 {
			return remaining[:spaceIdx]
		}
		return remaining
	}

	return ""
}

// shouldSkipFile determines if a file should be skipped based on its path and options.
func shouldSkipFile(path string, opts Options) bool {
	if path == "" {
		return false
	}

	// Get the base filename
	filename := filepath.Base(path)

	// Check for generated/lock files
	if opts.FilterGenerated && generatedFiles[filename] {
		return true
	}

	// Check for minified files
	if opts.FilterMinified {
		if strings.Contains(filename, ".min.js") || strings.Contains(filename, ".min.css") {
			return true
		}
	}

	// Check for binary files by extension
	if opts.FilterBinary {
		ext := strings.ToLower(filepath.Ext(path))
		if binaryExtensions[ext] {
			return true
		}
	}

	return false
}

// fileFilterReason returns a human-readable reason for why a file was filtered.
func fileFilterReason(path string, opts Options) string {
	filename := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(path))

	if opts.FilterGenerated && generatedFiles[filename] {
		return "generated/lock file content filtered"
	}
	if opts.FilterMinified && (strings.Contains(filename, ".min.js") || strings.Contains(filename, ".min.css")) {
		return "minified file content filtered"
	}
	if opts.FilterBinary && binaryExtensions[ext] {
		return "binary file content filtered"
	}
	return "file content filtered"
}

// isFileMetadataLine returns true for git diff metadata lines that describe
// the nature of a file change (deletion, creation, rename, mode change)
// rather than the actual content diff.
func isFileMetadataLine(line string) bool {
	return strings.HasPrefix(line, "deleted file mode") ||
		strings.HasPrefix(line, "new file mode") ||
		strings.HasPrefix(line, "old mode") ||
		strings.HasPrefix(line, "new mode") ||
		strings.HasPrefix(line, "similarity index") ||
		strings.HasPrefix(line, "rename from") ||
		strings.HasPrefix(line, "rename to") ||
		strings.HasPrefix(line, "copy from") ||
		strings.HasPrefix(line, "copy to")
}

// estimateTokens provides a rough estimate of token count for a string.
// Uses the approximation of ~4 characters per token.
func estimateTokens(text string) int {
	// Remove leading/trailing whitespace for more accurate count
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	// Rough approximation: 4 characters per token
	// This is a simplified estimate; actual tokenization is more complex
	tokens := len(text) / 4
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}

// FilterStats provides statistics about what was filtered.
type FilterStats struct {
	TotalFiles     int
	FilteredFiles  int
	BinaryFiles    int
	MinifiedFiles  int
	GeneratedFiles int
	TokensUsed     int
	Truncated      bool
}

// ProcessWithStats preprocesses a git diff and returns statistics about what was filtered.
func ProcessWithStats(diff string, opts Options) (string, *FilterStats) {
	// Use defaults if options are zero
	if opts.MaxTokens == 0 {
		opts.MaxTokens = 16384
	}

	stats := &FilterStats{}
	lines := strings.Split(diff, "\n")
	var result []string
	var currentFile string
	var skipCurrentFile bool
	tokensUsed := 0

	for _, line := range lines {
		// Check if we've exceeded token limit
		lineTokens := estimateTokens(line)
		if tokensUsed+lineTokens > opts.MaxTokens {
			stats.Truncated = true
			break
		}

		// Check for file header
		if strings.HasPrefix(line, "diff --git") {
			currentFile = extractFilePath(line)
			stats.TotalFiles++

			// Check why we might skip this file
			skipCurrentFile = false
			filename := filepath.Base(currentFile)
			ext := strings.ToLower(filepath.Ext(currentFile))

			if opts.FilterGenerated && generatedFiles[filename] {
				skipCurrentFile = true
				stats.GeneratedFiles++
				stats.FilteredFiles++
			} else if opts.FilterMinified && (strings.Contains(filename, ".min.js") || strings.Contains(filename, ".min.css")) {
				skipCurrentFile = true
				stats.MinifiedFiles++
				stats.FilteredFiles++
			} else if opts.FilterBinary && binaryExtensions[ext] {
				skipCurrentFile = true
				stats.BinaryFiles++
				stats.FilteredFiles++
			}

			// Always include the header so the AI knows about all changed files.
			result = append(result, line)
			tokensUsed += lineTokens

			if skipCurrentFile {
				// Add a note about why the content was filtered.
				note := fmt.Sprintf("(%s)", fileFilterReason(currentFile, opts))
				result = append(result, note)
				tokensUsed += estimateTokens(note)
			}
			continue
		}

		// Skip content lines for filtered files, but include file
		// metadata lines (deleted/new file mode, rename info) so
		// the AI knows the nature of the change.
		if skipCurrentFile {
			if isFileMetadataLine(line) {
				result = append(result, line)
				tokensUsed += lineTokens
			}
			continue
		}

		// Check for binary file indicator
		if strings.Contains(line, "Binary files") && strings.Contains(line, "differ") {
			if opts.FilterBinary && !skipCurrentFile {
				// This is a binary file we didn't catch by extension
				stats.BinaryFiles++
				stats.FilteredFiles++
				result = append(result, "Binary file (content omitted)")
				tokensUsed += estimateTokens("Binary file (content omitted)")
				skipCurrentFile = true
				continue
			}
		}

		// Add the line to result
		result = append(result, line)
		tokensUsed += lineTokens
	}

	stats.TokensUsed = tokensUsed

	// Add truncation indicator if needed
	if stats.Truncated {
		result = append(result, "", fmt.Sprintf("... (diff truncated at %d tokens, limit: %d)", tokensUsed, opts.MaxTokens))
	}

	return strings.Join(result, "\n"), stats
}
