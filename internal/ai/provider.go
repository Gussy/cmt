package ai

import (
	"context"
	"fmt"

	"github.com/gussy/cmt/internal/git"
)

// MessageFormat represents the format of the commit message.
type MessageFormat int

const (
	// FormatStandard is the default commit message format.
	FormatStandard MessageFormat = iota
	// FormatOneLine generates a single-line commit message (50 chars max).
	FormatOneLine
	// FormatVerbose generates a detailed commit message with explanation.
	FormatVerbose
)

// CommitRequest contains the information needed to generate a commit message.
type CommitRequest struct {
	// Diff is the git diff to describe.
	Diff string
	// StagedFiles is the list of files being committed.
	StagedFiles []string
	// Format specifies the desired message format.
	Format MessageFormat
	// Hint is optional additional context from the user.
	Hint string
	// Scope is the optional scope for conventional commits.
	Scope string
	// Model is the AI model to use (provider-specific).
	Model string
	// Temperature controls randomness (0.0 to 1.0).
	Temperature float64
	// MaxTokens limits the response length.
	MaxTokens int
}

// CommitResponse contains the generated commit message and metadata.
type CommitResponse struct {
	// Message is the generated commit message.
	Message string
	// Title is the commit title (first line) for multi-line messages.
	Title string
	// Body is the commit body for multi-line messages.
	Body string
	// TokensUsed is the number of tokens consumed.
	TokensUsed int
	// Model is the actual model used.
	Model string
}

// Provider defines the interface for AI providers.
type Provider interface {
	// Name returns the provider name.
	Name() string

	// IsAvailable checks if the provider is configured and ready to use.
	IsAvailable(ctx context.Context) (bool, error)

	// GenerateCommitMessage generates a commit message based on the request.
	GenerateCommitMessage(ctx context.Context, req *CommitRequest) (*CommitResponse, error)

	// RegenerateWithFeedback regenerates a commit message with user feedback.
	RegenerateWithFeedback(ctx context.Context, req *CommitRequest, previousMessage string, feedback string) (*CommitResponse, error)

	// AnalyzeHunkAssignment analyzes which hunks should be absorbed into which commits.
	AnalyzeHunkAssignment(ctx context.Context, req *AbsorbRequest) (*AbsorbResponse, error)

	// GetDefaultModel returns the default model for this provider.
	GetDefaultModel() string

	// GetAvailableModels returns a list of available models.
	GetAvailableModels() []string
}

// ProviderConfig contains configuration for a provider.
type ProviderConfig struct {
	// APIKey is the API key (not used for Claude CLI).
	APIKey string
	// BaseURL is the base API URL (for custom endpoints).
	BaseURL string
	// DefaultModel is the default model to use.
	DefaultModel string
	// Timeout is the request timeout in seconds.
	Timeout int
}

// ProviderError represents an error from a provider.
type ProviderError struct {
	Provider string
	Message  string
	Err      error
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s provider error: %s: %v", e.Provider, e.Message, e.Err)
	}
	return fmt.Sprintf("%s provider error: %s", e.Provider, e.Message)
}

// Unwrap returns the underlying error.
func (e *ProviderError) Unwrap() error {
	return e.Err
}

// NewProviderError creates a new provider error.
func NewProviderError(provider, message string, err error) error {
	return &ProviderError{
		Provider: provider,
		Message:  message,
		Err:      err,
	}
}

// AbsorbRequest contains the information needed for absorb analysis.
type AbsorbRequest struct {
	// Hunks are the diff hunks to analyze.
	Hunks []git.Hunk
	// Commits are the candidate commits with their diffs.
	Commits []git.CommitInfo
	// Strategy for handling ambiguous assignments.
	Strategy string // "interactive" or "best-match".
	// ConfidenceThreshold is the minimum confidence for auto-assignment.
	ConfidenceThreshold float64
	// Model is the AI model to use.
	Model string
	// Temperature controls randomness.
	Temperature float64
	// MaxTokens limits the response length.
	MaxTokens int
}

// AbsorbResponse contains the hunk assignments from AI analysis.
type AbsorbResponse struct {
	// Assignments maps each hunk to a commit.
	Assignments []HunkAssignment
	// UnmatchedHunks are hunks that couldn't be assigned.
	UnmatchedHunks []git.Hunk
	// TokensUsed is the number of tokens consumed.
	TokensUsed int
	// Model is the actual model used.
	Model string
}

// HunkAssignment represents the AI's assignment of a hunk to a commit.
type HunkAssignment struct {
	// Hunk is the hunk being assigned.
	Hunk git.Hunk
	// CommitSHA is the target commit SHA.
	CommitSHA string
	// CommitMessage is the first line of the commit message.
	CommitMessage string
	// Confidence is the AI's confidence in this assignment (0.0 to 1.0).
	Confidence float64
	// Reasoning is the AI's explanation for this assignment.
	Reasoning string
	// Alternatives are other possible assignments with lower confidence.
	Alternatives []AlternativeAssignment
}

// AlternativeAssignment represents an alternative commit for a hunk.
type AlternativeAssignment struct {
	CommitSHA     string
	CommitMessage string
	Confidence    float64
	Reasoning     string
}