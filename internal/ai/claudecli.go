package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gussy/cmt/internal/git"
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

// AnalyzeHunkAssignment analyzes which hunks should be absorbed into which commits.
func (c *ClaudeCLI) AnalyzeHunkAssignment(ctx context.Context, req *AbsorbRequest) (*AbsorbResponse, error) {
	if len(req.Hunks) == 0 {
		return nil, NewProviderError(c.Name(), "no hunks provided", nil)
	}

	if len(req.Commits) == 0 {
		// No commits to absorb into, all hunks are unmatched.
		return &AbsorbResponse{
			UnmatchedHunks: req.Hunks,
			Model:          c.getModelName(req.Model),
		}, nil
	}

	// Build the absorb prompt.
	prompt := c.buildAbsorbPrompt(req)

	// Execute claude command.
	response, err := c.executeClaudeCommand(ctx, prompt, req.Model)
	if err != nil {
		return nil, err
	}

	// Parse the JSON response.
	absorbResp, err := c.parseAbsorbResponse(response, req)
	if err != nil {
		return nil, NewProviderError(c.Name(), fmt.Sprintf("failed to parse absorb response: %v", err), err)
	}

	absorbResp.Model = c.getModelName(req.Model)
	return absorbResp, nil
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

	// Strip AI attribution trailers (Co-Authored-By, Signed-off-by, Generated-by, etc.).
	response = stripAttributionTrailers(response)

	return strings.TrimSpace(response)
}

// stripAttributionTrailers removes AI attribution lines from commit messages.
func stripAttributionTrailers(message string) string {
	lines := strings.Split(message, "\n")
	var cleaned []string
	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		if isAttributionLine(lower) {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}

// isAttributionLine checks if a line is an AI attribution trailer.
func isAttributionLine(lower string) bool {
	// Match git trailers referencing AI/Claude/Anthropic.
	trailerPrefixes := []string{
		"co-authored-by:",
		"signed-off-by:",
		"generated-by:",
		"authored-by:",
	}
	aiIndicators := []string{
		"claude",
		"anthropic",
		"noreply@anthropic.com",
		"ai assistant",
		"generated with",
	}

	for _, prefix := range trailerPrefixes {
		if strings.HasPrefix(lower, prefix) {
			for _, indicator := range aiIndicators {
				if strings.Contains(lower, indicator) {
					return true
				}
			}
		}
	}
	return false
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

// buildAbsorbPrompt builds the prompt for hunk assignment analysis.
func (c *ClaudeCLI) buildAbsorbPrompt(req *AbsorbRequest) string {
	var prompt strings.Builder

	prompt.WriteString("You are analyzing git diff hunks to determine which previous commits they should be absorbed into.\n")
	prompt.WriteString("Each hunk should be matched with the most semantically related commit based on:\n")
	prompt.WriteString("1. File paths and names\n")
	prompt.WriteString("2. Code context and functionality\n")
	prompt.WriteString("3. Commit message relevance\n")
	prompt.WriteString("4. Related changes in the same area\n\n")

	if req.Strategy == "best-match" {
		prompt.WriteString(fmt.Sprintf("Confidence threshold: %.2f (assign only if confidence is above this)\n", req.ConfidenceThreshold))
		prompt.WriteString("Strategy: Choose the single best matching commit for each hunk.\n\n")
	} else {
		prompt.WriteString("Strategy: Provide alternatives when multiple commits could match.\n\n")
	}

	// Add commits information.
	prompt.WriteString("Available commits (from oldest to newest):\n")
	prompt.WriteString("=====================================\n")
	for i, commit := range req.Commits {
		// Get first line of commit message.
		lines := strings.Split(commit.Message, "\n")
		firstLine := lines[0]
		if len(firstLine) > 72 {
			firstLine = firstLine[:69] + "..."
		}

		prompt.WriteString(fmt.Sprintf("\nCommit %d: %s\n", i+1, commit.SHA[:8]))
		prompt.WriteString(fmt.Sprintf("Message: %s\n", firstLine))

		// Add a summary of the commit diff.
		if len(commit.Diff) > 0 {
			prompt.WriteString("Changed files:\n")
			for _, line := range strings.Split(commit.Diff, "\n") {
				if strings.HasPrefix(line, "diff --git") {
					parts := strings.Split(line, " ")
					if len(parts) >= 4 {
						file := strings.TrimPrefix(parts[3], "b/")
						prompt.WriteString(fmt.Sprintf("  - %s\n", file))
					}
				}
			}
		}
	}

	// Add hunks to analyze.
	prompt.WriteString("\n\nHunks to analyze:\n")
	prompt.WriteString("================\n")
	for i, hunk := range req.Hunks {
		prompt.WriteString(fmt.Sprintf("\nHunk %d:\n", i+1))
		prompt.WriteString(fmt.Sprintf("File: %s\n", hunk.FilePath))
		if hunk.IsNew {
			prompt.WriteString("Status: NEW FILE\n")
		} else if hunk.IsDeleted {
			prompt.WriteString("Status: DELETED FILE\n")
		} else if hunk.IsRenamed {
			prompt.WriteString(fmt.Sprintf("Status: RENAMED from %s\n", hunk.OldFilePath))
		}
		prompt.WriteString(fmt.Sprintf("Lines: %s\n", hunk.Header))
		prompt.WriteString("Content:\n```diff\n")
		prompt.WriteString(hunk.Content)
		prompt.WriteString("```\n")
	}

	// Request structured output.
	prompt.WriteString("\n\nProvide your analysis as a JSON object with this structure:\n")
	prompt.WriteString("```json\n")
	prompt.WriteString("{\n")
	prompt.WriteString("  \"assignments\": [\n")
	prompt.WriteString("    {\n")
	prompt.WriteString("      \"hunk_index\": 0,  // 0-based index of the hunk\n")
	prompt.WriteString("      \"commit_sha\": \"abc123...\",  // Full SHA of the target commit\n")
	prompt.WriteString("      \"confidence\": 0.95,  // Confidence score 0.0 to 1.0\n")
	prompt.WriteString("      \"reasoning\": \"This hunk modifies the same function...\",\n")
	prompt.WriteString("      \"alternatives\": [  // Optional, only if strategy is 'interactive'\n")
	prompt.WriteString("        {\n")
	prompt.WriteString("          \"commit_sha\": \"def456...\",\n")
	prompt.WriteString("          \"confidence\": 0.7,\n")
	prompt.WriteString("          \"reasoning\": \"Could also relate to...\"\n")
	prompt.WriteString("        }\n")
	prompt.WriteString("      ]\n")
	prompt.WriteString("    }\n")
	prompt.WriteString("  ],\n")
	prompt.WriteString("  \"unmatched_hunks\": [0, 2]  // Indices of hunks that don't match any commit\n")
	prompt.WriteString("}\n")
	prompt.WriteString("```\n\n")
	prompt.WriteString("Return ONLY the JSON object, no additional explanation.")

	return prompt.String()
}

// absorbJSONResponse is the structure for parsing the AI's JSON response.
type absorbJSONResponse struct {
	Assignments []struct {
		HunkIndex    int     `json:"hunk_index"`
		CommitSHA    string  `json:"commit_sha"`
		Confidence   float64 `json:"confidence"`
		Reasoning    string  `json:"reasoning"`
		Alternatives []struct {
			CommitSHA  string  `json:"commit_sha"`
			Confidence float64 `json:"confidence"`
			Reasoning  string  `json:"reasoning"`
		} `json:"alternatives,omitempty"`
	} `json:"assignments"`
	UnmatchedHunks []int `json:"unmatched_hunks"`
}

// parseAbsorbResponse parses the JSON response from the AI.
func (c *ClaudeCLI) parseAbsorbResponse(response string, req *AbsorbRequest) (*AbsorbResponse, error) {
	// Clean the response to extract JSON.
	response = strings.TrimSpace(response)

	// Remove code block markers if present.
	if strings.Contains(response, "```json") {
		start := strings.Index(response, "{")
		end := strings.LastIndex(response, "}")
		if start >= 0 && end > start {
			response = response[start : end+1]
		}
	}

	// Parse JSON.
	var jsonResp absorbJSONResponse
	if err := json.Unmarshal([]byte(response), &jsonResp); err != nil {
		// Try to extract JSON from the response.
		lines := strings.Split(response, "\n")
		var jsonStr strings.Builder
		inJSON := false
		for _, line := range lines {
			if strings.Contains(line, "{") {
				inJSON = true
			}
			if inJSON {
				jsonStr.WriteString(line + "\n")
			}
			if strings.Contains(line, "}") && inJSON {
				break
			}
		}
		if jsonStr.Len() > 0 {
			if err := json.Unmarshal([]byte(jsonStr.String()), &jsonResp); err != nil {
				return nil, fmt.Errorf("failed to parse JSON: %w", err)
			}
		} else {
			return nil, fmt.Errorf("no valid JSON found in response")
		}
	}

	// Convert to AbsorbResponse.
	resp := &AbsorbResponse{
		Assignments:    []HunkAssignment{},
		UnmatchedHunks: []git.Hunk{},
	}

	// Track which hunks were assigned.
	assignedHunks := make(map[int]bool)

	// Process assignments.
	for _, assignment := range jsonResp.Assignments {
		if assignment.HunkIndex < 0 || assignment.HunkIndex >= len(req.Hunks) {
			continue
		}

		hunk := req.Hunks[assignment.HunkIndex]
		assignedHunks[assignment.HunkIndex] = true

		// Find commit message for this SHA.
		var commitMessage string
		for _, commit := range req.Commits {
			if strings.HasPrefix(commit.SHA, assignment.CommitSHA[:8]) {
				lines := strings.Split(commit.Message, "\n")
				commitMessage = lines[0]
				assignment.CommitSHA = commit.SHA // Use full SHA.
				break
			}
		}

		hunkAssignment := HunkAssignment{
			Hunk:          hunk,
			CommitSHA:     assignment.CommitSHA,
			CommitMessage: commitMessage,
			Confidence:    assignment.Confidence,
			Reasoning:     assignment.Reasoning,
		}

		// Process alternatives.
		for _, alt := range assignment.Alternatives {
			var altMessage string
			for _, commit := range req.Commits {
				if strings.HasPrefix(commit.SHA, alt.CommitSHA[:8]) {
					lines := strings.Split(commit.Message, "\n")
					altMessage = lines[0]
					alt.CommitSHA = commit.SHA
					break
				}
			}

			hunkAssignment.Alternatives = append(hunkAssignment.Alternatives, AlternativeAssignment{
				CommitSHA:     alt.CommitSHA,
				CommitMessage: altMessage,
				Confidence:    alt.Confidence,
				Reasoning:     alt.Reasoning,
			})
		}

		// Apply confidence threshold if using best-match strategy.
		if req.Strategy == "best-match" && assignment.Confidence < req.ConfidenceThreshold {
			resp.UnmatchedHunks = append(resp.UnmatchedHunks, hunk)
		} else {
			resp.Assignments = append(resp.Assignments, hunkAssignment)
		}
	}

	// Process unmatched hunks.
	for _, idx := range jsonResp.UnmatchedHunks {
		if idx >= 0 && idx < len(req.Hunks) && !assignedHunks[idx] {
			resp.UnmatchedHunks = append(resp.UnmatchedHunks, req.Hunks[idx])
		}
	}

	// Check for any hunks that weren't mentioned.
	for i, hunk := range req.Hunks {
		if !assignedHunks[i] {
			found := false
			for _, idx := range jsonResp.UnmatchedHunks {
				if idx == i {
					found = true
					break
				}
			}
			if !found {
				// Hunk wasn't assigned or marked as unmatched.
				resp.UnmatchedHunks = append(resp.UnmatchedHunks, hunk)
			}
		}
	}

	return resp, nil
}
