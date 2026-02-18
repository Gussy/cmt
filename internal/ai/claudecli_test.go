package ai

import "testing"

func TestStripAttributionTrailers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no trailers",
			input:    "feat: add user authentication",
			expected: "feat: add user authentication",
		},
		{
			name:     "co-authored-by claude",
			input:    "feat: add login\n\nCo-Authored-By: Claude <noreply@anthropic.com>",
			expected: "feat: add login\n",
		},
		{
			name:     "co-authored-by claude opus",
			input:    "fix: resolve bug\n\nCo-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>",
			expected: "fix: resolve bug\n",
		},
		{
			name:     "case insensitive",
			input:    "chore: update deps\n\nco-authored-by: claude <noreply@anthropic.com>",
			expected: "chore: update deps\n",
		},
		{
			name:     "generated-by anthropic",
			input:    "docs: update readme\n\nGenerated-by: Anthropic Claude",
			expected: "docs: update readme\n",
		},
		{
			name:     "signed-off-by claude",
			input:    "refactor: simplify logic\n\nSigned-off-by: Claude AI Assistant <noreply@anthropic.com>",
			expected: "refactor: simplify logic\n",
		},
		{
			name:     "preserves human co-author",
			input:    "feat: add feature\n\nCo-Authored-By: Jane Doe <jane@example.com>",
			expected: "feat: add feature\n\nCo-Authored-By: Jane Doe <jane@example.com>",
		},
		{
			name:     "preserves human signed-off",
			input:    "fix: bug\n\nSigned-off-by: John Smith <john@example.com>",
			expected: "fix: bug\n\nSigned-off-by: John Smith <john@example.com>",
		},
		{
			name:     "strips only AI trailer from mixed",
			input:    "feat: thing\n\nCo-Authored-By: Jane <jane@example.com>\nCo-Authored-By: Claude <noreply@anthropic.com>",
			expected: "feat: thing\n\nCo-Authored-By: Jane <jane@example.com>",
		},
		{
			name:     "multiline body with trailer",
			input:    "feat: add auth\n\nThis adds OAuth2 support.\n\nCo-Authored-By: Claude <noreply@anthropic.com>",
			expected: "feat: add auth\n\nThis adds OAuth2 support.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripAttributionTrailers(tt.input)
			if got != tt.expected {
				t.Errorf("stripAttributionTrailers() =\n%q\nwant:\n%q", got, tt.expected)
			}
		})
	}
}

func TestIsAttributionLine(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		{"co-authored-by: claude <noreply@anthropic.com>", true},
		{"co-authored-by: claude opus 4.6 <noreply@anthropic.com>", true},
		{"generated-by: anthropic claude", true},
		{"signed-off-by: claude ai assistant <noreply@anthropic.com>", true},
		{"co-authored-by: jane doe <jane@example.com>", false},
		{"signed-off-by: john smith <john@example.com>", false},
		{"this is a normal line", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := isAttributionLine(tt.line)
			if got != tt.expected {
				t.Errorf("isAttributionLine(%q) = %v, want %v", tt.line, got, tt.expected)
			}
		})
	}
}
