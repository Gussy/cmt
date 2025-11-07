package prompt

import (
	"fmt"
	"strings"
)

// Template represents a prompt template for generating commit messages.
type Template struct {
	Name        string
	Description string
	Format      string
	Examples    []string
}

// Templates contains predefined prompt templates.
var Templates = map[string]*Template{
	"conventional": {
		Name:        "conventional",
		Description: "Conventional Commits specification",
		Format: `Follow the Conventional Commits specification:
- Format: <type>(<scope>): <description>
- Types: feat, fix, docs, style, refactor, test, chore, perf, ci, build, revert
- Scope is optional but recommended
- Description should be imperative mood (e.g., "add" not "adds" or "added")
- Keep the first line under 50 characters`,
		Examples: []string{
			"feat(auth): add OAuth2 authentication",
			"fix(api): resolve race condition in user endpoints",
			"docs(readme): update installation instructions",
		},
	},
	"gitmoji": {
		Name:        "gitmoji",
		Description: "Gitmoji commit messages with emojis",
		Format: `Use Gitmoji format with appropriate emoji:
- Start with an emoji that represents the change
- Follow with a short description
- Common emojis:
  ✨ :sparkles: New feature
  🐛 :bug: Bug fix
  📝 :memo: Documentation
  🎨 :art: Code structure/format
  ♻️ :recycle: Refactoring
  ✅ :white_check_mark: Tests
  🔧 :wrench: Configuration`,
		Examples: []string{
			"✨ Add user authentication system",
			"🐛 Fix memory leak in data processor",
			"📝 Update API documentation",
		},
	},
	"semantic": {
		Name:        "semantic",
		Description: "Semantic commit messages",
		Format: `Create semantic commit messages:
- Use prefixes: [ADD], [FIX], [UPDATE], [REMOVE], [REFACTOR], [DOCS], [TEST]
- Follow with a clear description
- Be specific about what changed`,
		Examples: []string{
			"[ADD] User profile management features",
			"[FIX] Correct calculation in billing module",
			"[UPDATE] Dependencies to latest versions",
		},
	},
}

// Builder helps construct prompts for commit message generation.
type Builder struct {
	format       string
	scope        string
	hint         string
	template     *Template
	stagedFiles  []string
	diff         string
	isOneLine    bool
	isVerbose    bool
}

// NewBuilder creates a new prompt builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// WithFormat sets the commit message format.
func (b *Builder) WithFormat(format string) *Builder {
	b.format = format
	return b
}

// WithScope sets the scope for conventional commits.
func (b *Builder) WithScope(scope string) *Builder {
	b.scope = scope
	return b
}

// WithHint adds user-provided context.
func (b *Builder) WithHint(hint string) *Builder {
	b.hint = hint
	return b
}

// WithTemplate applies a predefined template.
func (b *Builder) WithTemplate(name string) *Builder {
	if template, ok := Templates[name]; ok {
		b.template = template
	}
	return b
}

// WithStagedFiles adds the list of staged files.
func (b *Builder) WithStagedFiles(files []string) *Builder {
	b.stagedFiles = files
	return b
}

// WithDiff adds the git diff.
func (b *Builder) WithDiff(diff string) *Builder {
	b.diff = diff
	return b
}

// OneLine sets the prompt to generate a single-line message.
func (b *Builder) OneLine() *Builder {
	b.isOneLine = true
	b.isVerbose = false
	return b
}

// Verbose sets the prompt to generate a detailed message.
func (b *Builder) Verbose() *Builder {
	b.isVerbose = true
	b.isOneLine = false
	return b
}

// Build constructs the final prompt.
func (b *Builder) Build() string {
	var prompt strings.Builder

	// Add base instruction based on format preference
	if b.isOneLine {
		prompt.WriteString("Generate a concise, single-line git commit message (maximum 50 characters).\n")
		prompt.WriteString("Be extremely brief but clear.\n\n")
	} else if b.isVerbose {
		prompt.WriteString("Generate a detailed git commit message with:\n")
		prompt.WriteString("1. A short title line (max 50 characters)\n")
		prompt.WriteString("2. A blank line\n")
		prompt.WriteString("3. A detailed explanation including:\n")
		prompt.WriteString("   - What changed\n")
		prompt.WriteString("   - Why it changed\n")
		prompt.WriteString("   - Any important implementation details\n\n")
	} else {
		prompt.WriteString("Generate a clear and concise git commit message.\n")
		prompt.WriteString("Keep the first line under 50 characters if possible.\n\n")
	}

	// Add template format if specified
	if b.template != nil {
		prompt.WriteString(fmt.Sprintf("Use the %s format:\n", b.template.Name))
		prompt.WriteString(b.template.Format)
		prompt.WriteString("\n\nExamples:\n")
		for _, example := range b.template.Examples {
			prompt.WriteString(fmt.Sprintf("- %s\n", example))
		}
		prompt.WriteString("\n")
	} else if b.format == "conventional" {
		// Default to conventional commits if no template but format specified
		prompt.WriteString("Follow Conventional Commits format: <type>(<scope>): <description>\n")
		prompt.WriteString("Types: feat, fix, docs, style, refactor, test, chore\n\n")
	}

	// Add scope instruction if provided
	if b.scope != "" {
		prompt.WriteString(fmt.Sprintf("Use '%s' as the scope for this commit.\n\n", b.scope))
	}

	// Add user hint if provided
	if b.hint != "" {
		prompt.WriteString("Additional context from user:\n")
		prompt.WriteString(b.hint)
		prompt.WriteString("\n\n")
	}

	// Add staged files if provided
	if len(b.stagedFiles) > 0 {
		prompt.WriteString("Files being committed:\n")
		for _, file := range b.stagedFiles {
			prompt.WriteString(fmt.Sprintf("- %s\n", file))
		}
		prompt.WriteString("\n")
	}

	// Add the diff
	if b.diff != "" {
		prompt.WriteString("Changes:\n```diff\n")
		prompt.WriteString(b.diff)
		prompt.WriteString("\n```\n\n")
	}

	// Add final instruction
	prompt.WriteString("Generate only the commit message without any additional explanation, ")
	prompt.WriteString("markdown formatting, or code blocks. ")
	prompt.WriteString("The response should be the exact commit message to use.\n\n")
	prompt.WriteString("IMPORTANT: Do not include any attribution, signatures, or metadata indicating AI generation ")
	prompt.WriteString("(e.g., 'generated by AI', 'co-authored by Claude', 'committed by AI', etc.). ")
	prompt.WriteString("Provide only the commit message itself.")

	return prompt.String()
}

// BuildRegenerationPrompt creates a prompt for regenerating with feedback.
func BuildRegenerationPrompt(originalPrompt, previousMessage, feedback string) string {
	var prompt strings.Builder

	prompt.WriteString("The user wants to modify the following commit message:\n\n")
	prompt.WriteString("Previous message:\n```\n")
	prompt.WriteString(previousMessage)
	prompt.WriteString("\n```\n\n")

	prompt.WriteString("User feedback:\n")
	prompt.WriteString(feedback)
	prompt.WriteString("\n\n")

	prompt.WriteString("Please generate a new commit message addressing the user's feedback.\n")
	prompt.WriteString("Keep the same general format and style unless the feedback suggests otherwise.\n\n")
	prompt.WriteString("IMPORTANT: Do not include any attribution, signatures, or metadata indicating AI generation ")
	prompt.WriteString("(e.g., 'generated by AI', 'co-authored by Claude', 'committed by AI', etc.). ")
	prompt.WriteString("Provide only the commit message itself.\n\n")

	prompt.WriteString("Original context:\n")
	prompt.WriteString(originalPrompt)

	return prompt.String()
}

// ExtractConventionalType attempts to extract the type from a conventional commit message.
func ExtractConventionalType(message string) string {
	// Look for conventional commit pattern
	parts := strings.SplitN(message, ":", 2)
	if len(parts) < 2 {
		return ""
	}

	// Extract type (and possibly scope)
	typeAndScope := strings.TrimSpace(parts[0])

	// Check if there's a scope
	if strings.Contains(typeAndScope, "(") {
		typePart := strings.SplitN(typeAndScope, "(", 2)[0]
		return strings.TrimSpace(typePart)
	}

	return typeAndScope
}

// FormatWithScope adds or updates the scope in a commit message.
func FormatWithScope(message, scope string) string {
	if scope == "" {
		return message
	}

	// Check if it's a conventional commit
	if strings.Contains(message, ":") {
		parts := strings.SplitN(message, ":", 2)
		if len(parts) == 2 {
			typeAndScope := parts[0]
			description := parts[1]

			// Remove existing scope if present
			if strings.Contains(typeAndScope, "(") && strings.Contains(typeAndScope, ")") {
				typeOnly := strings.SplitN(typeAndScope, "(", 2)[0]
				return fmt.Sprintf("%s(%s):%s", typeOnly, scope, description)
			}

			// Add scope
			return fmt.Sprintf("%s(%s):%s", typeAndScope, scope, description)
		}
	}

	// Not a conventional commit, return as is
	return message
}