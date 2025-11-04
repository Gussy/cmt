package security

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gussy/cmt/internal/ui"
)

// Scanner detects potential secrets in code.
type Scanner struct {
	patterns map[string]*regexp.Regexp
}

// NewScanner creates a new security scanner with all secret patterns.
func NewScanner() *Scanner {
	patterns := map[string]*regexp.Regexp{
		// AWS
		"AWS Access Key": regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		"AWS Secret Key": regexp.MustCompile(`aws(.{0,20})?['\"][0-9a-zA-Z/+=]{40}['\"]`),

		// GitHub
		"GitHub Token": regexp.MustCompile(`gh[ps]_[a-zA-Z0-9]{36}`),

		// Private Keys
		"Private Key":     regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`),
		"SSH DSA Key":     regexp.MustCompile(`-----BEGIN DSA PRIVATE KEY-----`),

		// API Keys
		"Google API Key":  regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`),
		"Slack Token":     regexp.MustCompile(`xox[baprs]-([0-9a-zA-Z]{10,48})`),
		"Stripe API Key":  regexp.MustCompile(`sk_live_[0-9a-zA-Z]{24}`),
		"NPM Token":       regexp.MustCompile(`npm_[a-zA-Z0-9]{36}`),

		// Generic Patterns
		"Generic API Key": regexp.MustCompile(`[aA][pP][iI]_?[kK][eE][yY].*['\"]([0-9a-zA-Z]{32,45})['\"]`),
		"Generic Secret":  regexp.MustCompile(`[sS][eE][cC][rR][eE][tT].*['\"]([0-9a-zA-Z]{32,45})['\"]`),

		// Authentication
		"Password in URL": regexp.MustCompile(`[a-zA-Z]{3,10}://[^/\\s:@]{1,}:[^/\\s:@]{1,}@.{1,}`),
		"JWT Token":       regexp.MustCompile(`eyJ[A-Za-z0-9-_=]+\.[A-Za-z0-9-_=]+\.?[A-Za-z0-9-_.+/=]*`),
		"Bearer Token":    regexp.MustCompile(`[bB][eE][aA][rR][eE][rR][\s]+[a-zA-Z0-9\-_.]+`),
		"Basic Auth":      regexp.MustCompile(`[bB][aA][sS][iI][cC][\s]+[a-zA-Z0-9\-_.=]+`),
	}

	return &Scanner{
		patterns: patterns,
	}
}

// Scan analyzes the git diff for potential secrets.
func (s *Scanner) Scan(diff string) ([]ui.Secret, error) {
	if diff == "" {
		return nil, nil
	}

	var secrets []ui.Secret
	lines := strings.Split(diff, "\n")

	var currentFile string
	lineNumber := 0

	for _, line := range lines {
		// Extract file path from diff headers.
		if strings.HasPrefix(line, "diff --git") {
			// Format: diff --git a/path/to/file b/path/to/file
			parts := strings.Split(line, " ")
			if len(parts) >= 3 {
				// Remove a/ or b/ prefix.
				currentFile = strings.TrimPrefix(parts[2], "a/")
			}
			lineNumber = 0
			continue
		}

		// Track line numbers in the file.
		if strings.HasPrefix(line, "@@") {
			// Parse hunk header to get starting line number.
			// Format: @@ -1,2 +3,4 @@
			lineNumber = s.parseHunkHeader(line)
			continue
		}

		// Only scan added lines.
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			lineNumber++

			// Skip if it's a test or example file.
			if s.isTestOrExample(line) {
				continue
			}

			// Check each pattern.
			for secretType, pattern := range s.patterns {
				matches := pattern.FindAllString(line, -1)
				for _, match := range matches {
					// Check for false positives.
					if s.isFalsePositive(match, secretType) {
						continue
					}

					// Add the secret.
					secrets = append(secrets, ui.Secret{
						Type:     secretType,
						FilePath: currentFile,
						Line:     lineNumber,
						Match:    s.redact(match),
					})
				}
			}
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			// Skip removed lines but track line number.
			continue
		} else if !strings.HasPrefix(line, "\\") {
			// Context line.
			lineNumber++
		}
	}

	return secrets, nil
}

// parseHunkHeader extracts the starting line number from a hunk header.
func (s *Scanner) parseHunkHeader(header string) int {
	// Format: @@ -old_start,old_count +new_start,new_count @@
	// We want new_start.
	parts := strings.Split(header, " ")
	for _, part := range parts {
		if strings.HasPrefix(part, "+") {
			// Remove the + and extract the number before the comma.
			numStr := strings.TrimPrefix(part, "+")
			if commaIdx := strings.Index(numStr, ","); commaIdx > 0 {
				numStr = numStr[:commaIdx]
			}

			// Parse the number (ignoring error for simplicity).
			var num int
			fmt.Sscanf(numStr, "%d", &num)
			return num - 1 // Subtract 1 as we'll increment for each line.
		}
	}
	return 0
}

// isTestOrExample checks if the line is from a test or example file.
func (s *Scanner) isTestOrExample(line string) bool {
	testKeywords := []string{
		"test", "Test", "TEST",
		"example", "Example", "EXAMPLE",
		"sample", "Sample", "SAMPLE",
		"demo", "Demo", "DEMO",
		"mock", "Mock", "MOCK",
		"fake", "Fake", "FAKE",
		"dummy", "Dummy", "DUMMY",
	}

	for _, keyword := range testKeywords {
		if strings.Contains(line, keyword) {
			return true
		}
	}

	return false
}

// isFalsePositive checks if a match is likely a false positive.
func (s *Scanner) isFalsePositive(match string, secretType string) bool {
	// Check for placeholder patterns.
	placeholders := []string{
		"xxxxxxxxxx", "XXXXXXXXXX",
		"0000000000",
		"1234567890",
		"your-api-key", "YOUR_API_KEY", "YOUR-API-KEY",
		"<api-key>", "<API-KEY>",
		"${", "{{", // Template/env var syntax
	}

	for _, placeholder := range placeholders {
		if strings.Contains(match, placeholder) {
			return true
		}
	}

	// Check if it starts with environment variable syntax.
	if strings.HasPrefix(match, "$") || strings.HasPrefix(match, "{{") {
		return true
	}

	// For generic patterns, be more strict.
	if strings.Contains(secretType, "Generic") {
		// Check if the value part is too repetitive.
		if s.isRepetitive(match) {
			return true
		}
	}

	return false
}

// isRepetitive checks if a string is too repetitive to be a real secret.
func (s *Scanner) isRepetitive(str string) bool {
	if len(str) < 10 {
		return false
	}

	// Count unique characters.
	uniqueChars := make(map[rune]bool)
	for _, ch := range str {
		uniqueChars[ch] = true
	}

	// If less than 5 unique characters in a long string, it's likely fake.
	if len(uniqueChars) < 5 {
		return true
	}

	// Check for repeating patterns.
	if len(str) >= 8 {
		// Check if first 4 chars repeat.
		pattern := str[:4]
		repeatCount := 0
		for i := 0; i < len(str)-3; i += 4 {
			if i+4 <= len(str) && str[i:i+4] == pattern {
				repeatCount++
			}
		}
		if repeatCount > 2 {
			return true
		}
	}

	return false
}

// redact creates a safe display version of a secret.
func (s *Scanner) redact(secret string) string {
	if len(secret) <= 8 {
		return "***"
	}

	// Show first 4 and last 4 characters.
	return secret[:4] + "..." + secret[len(secret)-4:]
}