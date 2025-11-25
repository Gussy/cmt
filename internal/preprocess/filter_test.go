package preprocess

import (
	"strings"
	"testing"
)

func TestExtractFilePath(t *testing.T) {
	tests := []struct {
		name     string
		diffLine string
		expected string
	}{
		{
			name:     "standard diff header",
			diffLine: "diff --git a/internal/config/config.go b/internal/config/config.go",
			expected: "internal/config/config.go",
		},
		{
			name:     "with space in path",
			diffLine: "diff --git a/my folder/file.txt b/my folder/file.txt",
			expected: "my folder/file.txt",
		},
		{
			name:     "new file",
			diffLine: "diff --git a/newfile.go b/newfile.go",
			expected: "newfile.go",
		},
		{
			name:     "deleted file",
			diffLine: "diff --git a/deleted.txt b/deleted.txt",
			expected: "deleted.txt",
		},
		{
			name:     "empty line",
			diffLine: "",
			expected: "",
		},
		{
			name:     "malformed header",
			diffLine: "diff --git something wrong",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractFilePath(tc.diffLine)
			if result != tc.expected {
				t.Errorf("extractFilePath(%q) = %q, expected %q", tc.diffLine, result, tc.expected)
			}
		})
	}
}

func TestShouldSkipFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		opts     Options
		expected bool
	}{
		// Binary files
		{
			name:     "image file png",
			path:     "logo.png",
			opts:     Options{FilterBinary: true},
			expected: true,
		},
		{
			name:     "image file jpg",
			path:     "photo.jpg",
			opts:     Options{FilterBinary: true},
			expected: true,
		},
		{
			name:     "pdf file",
			path:     "document.pdf",
			opts:     Options{FilterBinary: true},
			expected: true,
		},
		{
			name:     "executable",
			path:     "app.exe",
			opts:     Options{FilterBinary: true},
			expected: true,
		},
		{
			name:     "binary disabled",
			path:     "image.png",
			opts:     Options{FilterBinary: false},
			expected: false,
		},
		// Minified files
		{
			name:     "minified js",
			path:     "bundle.min.js",
			opts:     Options{FilterMinified: true},
			expected: true,
		},
		{
			name:     "minified css",
			path:     "styles.min.css",
			opts:     Options{FilterMinified: true},
			expected: true,
		},
		{
			name:     "regular js",
			path:     "app.js",
			opts:     Options{FilterMinified: true},
			expected: false,
		},
		{
			name:     "minified disabled",
			path:     "bundle.min.js",
			opts:     Options{FilterMinified: false},
			expected: false,
		},
		// Generated files
		{
			name:     "package-lock.json",
			path:     "package-lock.json",
			opts:     Options{FilterGenerated: true},
			expected: true,
		},
		{
			name:     "yarn.lock",
			path:     "yarn.lock",
			opts:     Options{FilterGenerated: true},
			expected: true,
		},
		{
			name:     "go.sum",
			path:     "go.sum",
			opts:     Options{FilterGenerated: true},
			expected: true,
		},
		{
			name:     ".DS_Store",
			path:     ".DS_Store",
			opts:     Options{FilterGenerated: true},
			expected: true,
		},
		{
			name:     "generated disabled",
			path:     "package-lock.json",
			opts:     Options{FilterGenerated: false},
			expected: false,
		},
		// Regular files
		{
			name:     "go file",
			path:     "main.go",
			opts:     Options{FilterBinary: true, FilterMinified: true, FilterGenerated: true},
			expected: false,
		},
		{
			name:     "markdown file",
			path:     "README.md",
			opts:     Options{FilterBinary: true, FilterMinified: true, FilterGenerated: true},
			expected: false,
		},
		// Path with directories
		{
			name:     "nested binary",
			path:     "assets/images/logo.png",
			opts:     Options{FilterBinary: true},
			expected: true,
		},
		{
			name:     "nested lock file",
			path:     "frontend/package-lock.json",
			opts:     Options{FilterGenerated: true},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := shouldSkipFile(tc.path, tc.opts)
			if result != tc.expected {
				t.Errorf("shouldSkipFile(%q, opts) = %v, expected %v", tc.path, result, tc.expected)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name:     "whitespace only",
			text:     "   ",
			expected: 0,
		},
		{
			name:     "short text",
			text:     "Hi",
			expected: 1,
		},
		{
			name:     "4 chars",
			text:     "Test",
			expected: 1,
		},
		{
			name:     "8 chars",
			text:     "TestText",
			expected: 2,
		},
		{
			name:     "typical line",
			text:     "func main() { fmt.Println(\"Hello, World!\") }",
			expected: 11, // 45 chars / 4 = 11.25 -> 11
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := estimateTokens(tc.text)
			if result != tc.expected {
				t.Errorf("estimateTokens(%q) = %d, expected %d", tc.text, result, tc.expected)
			}
		})
	}
}

func TestProcess(t *testing.T) {
	tests := []struct {
		name     string
		diff     string
		opts     Options
		contains []string
		excludes []string
	}{
		{
			name: "filter binary file",
			diff: `diff --git a/logo.png b/logo.png
Binary files a/logo.png and b/logo.png differ
diff --git a/main.go b/main.go
+func hello() {
+    fmt.Println("Hello")
+}`,
			opts: Options{
				FilterBinary: true,
				MaxTokens:    1000,
			},
			contains: []string{"main.go", "hello()"},
			excludes: []string{"logo.png"},
		},
		{
			name: "filter minified file",
			diff: `diff --git a/bundle.min.js b/bundle.min.js
+var a=function(){return 1};
diff --git a/app.js b/app.js
+function app() {
+    return true;
+}`,
			opts: Options{
				FilterMinified: true,
				MaxTokens:      1000,
			},
			contains: []string{"app.js", "function app()"},
			excludes: []string{"bundle.min.js"},
		},
		{
			name: "filter lock file",
			diff: `diff --git a/package-lock.json b/package-lock.json
+{
+  "lockfileVersion": 2
+}
diff --git a/package.json b/package.json
+{
+  "name": "myapp"
+}`,
			opts: Options{
				FilterGenerated: true,
				MaxTokens:       1000,
			},
			contains: []string{"package.json", "myapp"},
			excludes: []string{"package-lock.json", "lockfileVersion"},
		},
		{
			name: "token truncation",
			diff: generateLongDiff(),
			opts: Options{
				MaxTokens: 50, // Very small limit
			},
			contains: []string{"... (diff truncated"},
		},
		{
			name: "no filtering",
			diff: `diff --git a/logo.png b/logo.png
Binary files a/logo.png and b/logo.png differ
diff --git a/main.go b/main.go
+func main() {}`,
			opts: Options{
				FilterBinary:    false,
				FilterMinified:  false,
				FilterGenerated: false,
				MaxTokens:       1000,
			},
			contains: []string{"logo.png", "main.go", "Binary file"},
		},
		{
			name: "multiple filters",
			diff: `diff --git a/image.jpg b/image.jpg
Binary files differ
diff --git a/styles.min.css b/styles.min.css
+.class{margin:0}
diff --git a/yarn.lock b/yarn.lock
+dependencies:
diff --git a/app.go b/app.go
+package main`,
			opts: Options{
				FilterBinary:    true,
				FilterMinified:  true,
				FilterGenerated: true,
				MaxTokens:       1000,
			},
			contains: []string{"app.go", "package main"},
			excludes: []string{"image.jpg", "styles.min.css", "yarn.lock"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Process(tc.diff, tc.opts)

			// Check that expected strings are contained
			for _, expected := range tc.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected result to contain %q, but it doesn't.\nResult:\n%s", expected, result)
				}
			}

			// Check that excluded strings are not contained
			for _, excluded := range tc.excludes {
				if strings.Contains(result, excluded) {
					t.Errorf("Expected result NOT to contain %q, but it does.\nResult:\n%s", excluded, result)
				}
			}
		})
	}
}

func TestProcessWithStats(t *testing.T) {
	diff := `diff --git a/logo.png b/logo.png
Binary files differ
diff --git a/bundle.min.js b/bundle.min.js
+var a=1;
diff --git a/go.sum b/go.sum
+github.com/example v1.0.0
diff --git a/main.go b/main.go
+func main() {}`

	opts := Options{
		FilterBinary:    true,
		FilterMinified:  true,
		FilterGenerated: true,
		MaxTokens:       1000,
	}

	result, stats := ProcessWithStats(diff, opts)

	// Check stats
	if stats.TotalFiles != 4 {
		t.Errorf("Expected TotalFiles = 4, got %d", stats.TotalFiles)
	}
	if stats.FilteredFiles != 3 {
		t.Errorf("Expected FilteredFiles = 3, got %d", stats.FilteredFiles)
	}
	if stats.BinaryFiles != 1 {
		t.Errorf("Expected BinaryFiles = 1, got %d", stats.BinaryFiles)
	}
	if stats.MinifiedFiles != 1 {
		t.Errorf("Expected MinifiedFiles = 1, got %d", stats.MinifiedFiles)
	}
	if stats.GeneratedFiles != 1 {
		t.Errorf("Expected GeneratedFiles = 1, got %d", stats.GeneratedFiles)
	}
	if stats.Truncated {
		t.Error("Expected Truncated = false")
	}

	// Check that only main.go is in the result
	if !strings.Contains(result, "main.go") {
		t.Error("Expected result to contain main.go")
	}
	if strings.Contains(result, "logo.png") || strings.Contains(result, "bundle.min.js") || strings.Contains(result, "go.sum") {
		t.Error("Expected result NOT to contain filtered files")
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.MaxTokens != 16384 {
		t.Errorf("Expected MaxTokens = 16384, got %d", opts.MaxTokens)
	}
	if !opts.FilterBinary {
		t.Error("Expected FilterBinary = true")
	}
	if !opts.FilterMinified {
		t.Error("Expected FilterMinified = true")
	}
	if !opts.FilterGenerated {
		t.Error("Expected FilterGenerated = true")
	}
}

func TestProcessEmptyDiff(t *testing.T) {
	result := Process("", DefaultOptions())
	if result != "" {
		t.Errorf("Expected empty result for empty diff, got %q", result)
	}
}

func TestProcessWithZeroOptions(t *testing.T) {
	// Test that zero Options struct uses sensible defaults
	diff := `diff --git a/main.go b/main.go
+func main() {}`

	result := Process(diff, Options{})
	if !strings.Contains(result, "main.go") {
		t.Error("Expected result to contain main.go even with zero Options")
	}
}

// Helper function to generate a long diff for testing truncation
func generateLongDiff() string {
	var sb strings.Builder
	sb.WriteString("diff --git a/file.txt b/file.txt\n")
	for i := 0; i < 1000; i++ {
		sb.WriteString("+This is a very long line that will contribute to token count\n")
	}
	return sb.String()
}
