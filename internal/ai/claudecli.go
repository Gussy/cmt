package ai

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ClaudeCLI implements the Provider interface using the Claude Code CLI.
type ClaudeCLI struct {
	config     *ProviderConfig
	claudePath string
}

// NewClaudeCLI creates a new Claude CLI provider.
func NewClaudeCLI(config *ProviderConfig) (*ClaudeCLI, error) {
	if config == nil {
		config = &ProviderConfig{
			DefaultModel: "haiku-4.5",
			Timeout:      60,
		}
	}

	// Find claude executable
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return nil, NewProviderError("claude-cli", "claude command not found in PATH", err)
	}

	return &ClaudeCLI{
		config:     config,
		claudePath: claudePath,
	}, nil
}

// Name returns the provider name.
func (c *ClaudeCLI) Name() string {
	return "claude-cli"
}

// IsAvailable checks if Claude CLI is installed and accessible.
func (c *ClaudeCLI) IsAvailable(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, c.claudePath, "--version")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Provide more details about why claude is not available
		errMsg := fmt.Sprintf("claude --version failed: %v", err)
		if stderr.Len() > 0 {
			errMsg = fmt.Sprintf("%s (stderr: %s)", errMsg, stderr.String())
		}
		if stdout.Len() > 0 {
			errMsg = fmt.Sprintf("%s (stdout: %s)", errMsg, stdout.String())
		}
		return false, fmt.Errorf("%s", errMsg)
	}
	return true, nil
}

// GenerateCommitMessage generates a commit message using Claude CLI.
func (c *ClaudeCLI) GenerateCommitMessage(ctx context.Context, req *CommitRequest) (*CommitResponse, error) {
	if req.Diff == "" {
		return nil, NewProviderError(c.Name(), "no diff provided", nil)
	}

	// Build the prompt
	prompt := c.buildPrompt(req)

	// Execute claude command
	response, err := c.executeClaudeCommand(ctx, prompt, req.Model)
	if err != nil {
		return nil, err
	}

	// Parse and clean the response
	message := c.cleanResponse(response)

	// Split into title and body for multi-line messages
	title, body := c.splitMessage(message)

	return &CommitResponse{
		Message: message,
		Title:   title,
		Body:    body,
		Model:   c.getModelName(req.Model),
	}, nil
}

// RegenerateWithFeedback regenerates a commit message with user feedback.
func (c *ClaudeCLI) RegenerateWithFeedback(ctx context.Context, req *CommitRequest, previousMessage string, feedback string) (*CommitResponse, error) {
	// Build prompt with feedback
	prompt := c.buildPromptWithFeedback(req, previousMessage, feedback)

	// Execute claude command
	response, err := c.executeClaudeCommand(ctx, prompt, req.Model)
	if err != nil {
		return nil, err
	}

	// Parse and clean the response
	message := c.cleanResponse(response)

	// Split into title and body for multi-line messages
	title, body := c.splitMessage(message)

	return &CommitResponse{
		Message: message,
		Title:   title,
		Body:    body,
		Model:   c.getModelName(req.Model),
	}, nil
}

// GetDefaultModel returns the default model for Claude CLI.
func (c *ClaudeCLI) GetDefaultModel() string {
	if c.config.DefaultModel != "" {
		return c.config.DefaultModel
	}
	return "haiku-4.5"
}

// GetAvailableModels returns available Claude models.
func (c *ClaudeCLI) GetAvailableModels() []string {
	return []string{
		"haiku-4.5",
		"sonnet-4.5",
		"opus-4.1",
	}
}

// executeClaudeCommand executes the claude CLI command with the given prompt.
func (c *ClaudeCLI) executeClaudeCommand(ctx context.Context, prompt string, model string) (string, error) {
	if model == "" {
		model = c.GetDefaultModel()
	}

	// Prepare the command
	args := []string{}

	// Add model flag if specified
	if model != "" && model != "default" {
		args = append(args, "--model", c.mapModelName(model))
	}

	// Create command with timeout
	timeout := time.Duration(c.config.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.claudePath, args...)
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()
	if err != nil {
		stderrStr := stderr.String()
		stdoutStr := stdout.String()

		// Build detailed error message
		errMsg := fmt.Sprintf("claude command failed (exit: %v)", err)
		if stderrStr != "" {
			errMsg = fmt.Sprintf("%s\nstderr: %s", errMsg, stderrStr)
		}
		if stdoutStr != "" {
			errMsg = fmt.Sprintf("%s\nstdout: %s", errMsg, stdoutStr)
		}

		// Log command details for debugging
		debugMsg := fmt.Sprintf("Command: %s %s\nPrompt length: %d chars",
			c.claudePath, strings.Join(args, " "), len(prompt))

		return "", NewProviderError(c.Name(), fmt.Sprintf("%s\nDebug: %s", errMsg, debugMsg), err)
	}

	output := stdout.String()
	if output == "" {
		return "", NewProviderError(c.Name(), "empty response from claude", nil)
	}

	return output, nil
}

// buildPrompt builds the prompt for commit message generation.
func (c *ClaudeCLI) buildPrompt(req *CommitRequest) string {
	var prompt strings.Builder

	// Base instruction
	switch req.Format {
	case FormatOneLine:
		prompt.WriteString("Generate a concise, single-line git commit message (max 50 characters) for the following changes.\n")
		prompt.WriteString("The message should be clear and descriptive but very brief.\n")
	case FormatVerbose:
		prompt.WriteString("Generate a detailed git commit message for the following changes.\n")
		prompt.WriteString("Include a short title line (max 50 chars), followed by a blank line, ")
		prompt.WriteString("then a detailed explanation of what changed and why.\n")
	default:
		prompt.WriteString("Generate a clear and concise git commit message for the following changes.\n")
		prompt.WriteString("Follow conventional commit format if applicable.\n")
	}

	// Add scope if provided
	if req.Scope != "" {
		prompt.WriteString(fmt.Sprintf("Use scope '%s' in the commit message (e.g., 'feat(%s): description').\n", req.Scope, req.Scope))
	}

	// Add user hint if provided
	if req.Hint != "" {
		prompt.WriteString(fmt.Sprintf("\nAdditional context: %s\n", req.Hint))
	}

	// Add file list
	if len(req.StagedFiles) > 0 {
		prompt.WriteString("\nFiles being committed:\n")
		for _, file := range req.StagedFiles {
			prompt.WriteString(fmt.Sprintf("- %s\n", file))
		}
	}

	// Add the diff
	prompt.WriteString("\nGit diff:\n```diff\n")
	prompt.WriteString(req.Diff)
	prompt.WriteString("\n```\n\n")

	// Final instruction
	prompt.WriteString("Generate only the commit message, without any additional explanation or formatting.")

	return prompt.String()
}

// buildPromptWithFeedback builds a prompt that includes user feedback.
func (c *ClaudeCLI) buildPromptWithFeedback(req *CommitRequest, previousMessage string, feedback string) string {
	var prompt strings.Builder

	// Start with context about regeneration
	prompt.WriteString("The user requested changes to a git commit message.\n\n")
	prompt.WriteString("Previous message:\n```\n")
	prompt.WriteString(previousMessage)
	prompt.WriteString("\n```\n\n")
	prompt.WriteString("User feedback:\n")
	prompt.WriteString(feedback)
	prompt.WriteString("\n\n")

	// Add the rest of the normal prompt
	basePrompt := c.buildPrompt(req)
	prompt.WriteString(basePrompt)

	return prompt.String()
}

// cleanResponse cleans up the Claude response.
func (c *ClaudeCLI) cleanResponse(response string) string {
	// Remove leading/trailing whitespace
	response = strings.TrimSpace(response)

	// Remove code block markers if present
	if strings.HasPrefix(response, "```") {
		lines := strings.Split(response, "\n")
		var cleaned []string
		inCodeBlock := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inCodeBlock = !inCodeBlock
				continue
			}
			if !inCodeBlock {
				cleaned = append(cleaned, line)
			}
		}
		response = strings.Join(cleaned, "\n")
	}

	// Remove quotes if the entire message is quoted
	if strings.HasPrefix(response, "\"") && strings.HasSuffix(response, "\"") {
		response = strings.Trim(response, "\"")
	}

	return strings.TrimSpace(response)
}

// splitMessage splits a commit message into title and body.
func (c *ClaudeCLI) splitMessage(message string) (string, string) {
	lines := strings.Split(message, "\n")
	if len(lines) == 0 {
		return "", ""
	}

	title := lines[0]

	// Find the body (skip blank lines after title)
	var bodyLines []string
	foundBody := false
	for i := 1; i < len(lines); i++ {
		if !foundBody && strings.TrimSpace(lines[i]) == "" {
			continue
		}
		foundBody = true
		bodyLines = append(bodyLines, lines[i])
	}

	body := strings.TrimSpace(strings.Join(bodyLines, "\n"))

	return title, body
}

// mapModelName maps user-friendly model names to Claude CLI model names.
func (c *ClaudeCLI) mapModelName(model string) string {
	// Remove version numbers and map to claude CLI format
	model = strings.ToLower(model)

	// Handle common variations
	switch {
	case strings.Contains(model, "haiku"):
		// Use the correct Claude Haiku 4.5 identifier
		return "claude-haiku-4-5"
	case strings.Contains(model, "sonnet"):
		// Use the correct Claude Sonnet 4.5 identifier
		return "claude-sonnet-4-5"
	case strings.Contains(model, "opus"):
		// Use the correct Claude Opus 4.1 identifier
		return "claude-opus-4-1"
	default:
		return model
	}
}

// getModelName returns the user-friendly model name.
func (c *ClaudeCLI) getModelName(model string) string {
	if model == "" {
		model = c.GetDefaultModel()
	}
	return model
}
