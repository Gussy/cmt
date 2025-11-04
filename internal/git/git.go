package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Repository represents a git repository.
type Repository struct {
	Path string
}

// FileStatus represents the status of a file in git.
type FileStatus struct {
	Path     string
	Status   string // M=modified, A=added, D=deleted, R=renamed, C=copied, U=untracked
	IsStaged bool
}

// NewRepository creates a new Repository instance.
func NewRepository(path string) (*Repository, error) {
	if path == "" {
		// Use current working directory
		var err error
		path, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	repo := &Repository{Path: path}

	// Check if it's a git repository
	if !repo.IsGitRepository() {
		return nil, fmt.Errorf("not a git repository: %s", path)
	}

	return repo, nil
}

// IsGitRepository checks if the path is inside a git repository.
func (r *Repository) IsGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = r.Path
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Debug: Print the error for now.
		fmt.Printf("Git check failed in dir %s: %v, output: %s\n", r.Path, err, output)
	}
	return err == nil
}

// GetRootPath returns the root path of the git repository.
func (r *Repository) GetRootPath() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = r.Path
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get repository root: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetDiff returns the diff of staged changes.
func (r *Repository) GetDiff(ctx context.Context, staged bool) (string, error) {
	args := []string{"diff"}

	if staged {
		args = append(args, "--cached")
	}

	// Add options for better diff output
	args = append(args,
		"--no-color",    // No color codes
		"--no-ext-diff", // Don't use external diff tools
		"--unified=3",   // 3 lines of context
	)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.Path

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git diff failed: %s", exitErr.Stderr)
		}
		return "", fmt.Errorf("git diff failed: %w", err)
	}

	return string(output), nil
}

// GetStatus returns the status of files in the repository.
func (r *Repository) GetStatus(ctx context.Context) ([]FileStatus, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain", "-uall")
	cmd.Dir = r.Path

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}

	var files []FileStatus
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Parse status line (format: "XY filename")
		if len(line) < 3 {
			continue
		}

		stagedStatus := line[0]
		unstagedStatus := line[1]
		filename := strings.TrimSpace(line[3:])

		// Handle renamed files (format: "R  old -> new")
		if strings.Contains(filename, " -> ") {
			parts := strings.Split(filename, " -> ")
			if len(parts) == 2 {
				filename = parts[1]
			}
		}

		// Determine if file is staged
		isStaged := stagedStatus != ' ' && stagedStatus != '?'

		// Determine status
		var status string
		if stagedStatus != ' ' && stagedStatus != '?' {
			status = string(stagedStatus)
		} else if unstagedStatus != ' ' {
			status = string(unstagedStatus)
		}

		if status != "" {
			files = append(files, FileStatus{
				Path:     filename,
				Status:   status,
				IsStaged: isStaged,
			})
		}
	}

	return files, nil
}

// GetStagedFiles returns a list of staged file paths.
func (r *Repository) GetStagedFiles(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--name-only")
	cmd.Dir = r.Path

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get staged files: %w", err)
	}

	if len(output) == 0 {
		return []string{}, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []string
	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// StageAll stages all changes in the repository.
func (r *Repository) StageAll(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "add", "-A")
	cmd.Dir = r.Path

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stage all files: %w", err)
	}

	return nil
}

// StageFiles stages specific files.
func (r *Repository) StageFiles(ctx context.Context, files []string) error {
	if len(files) == 0 {
		return nil
	}

	args := append([]string{"add"}, files...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.Path

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stage files: %w", err)
	}

	return nil
}

// UnstageFiles unstages specific files.
func (r *Repository) UnstageFiles(ctx context.Context, files []string) error {
	if len(files) == 0 {
		return nil
	}

	args := append([]string{"reset", "HEAD", "--"}, files...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.Path

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to unstage files: %w", err)
	}

	return nil
}

// Commit creates a commit with the given message.
func (r *Repository) Commit(ctx context.Context, message string) error {
	if message == "" {
		return fmt.Errorf("commit message cannot be empty")
	}

	cmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	cmd.Dir = r.Path

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("git commit failed: %s", stderr.String())
		}
		return fmt.Errorf("git commit failed: %w", err)
	}

	return nil
}

// Push pushes commits to the remote repository.
func (r *Repository) Push(ctx context.Context) error {
	// Get current branch
	branch, err := r.GetCurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "push", "origin", branch)
	cmd.Dir = r.Path

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("git push failed: %s", stderr.String())
		}
		return fmt.Errorf("git push failed: %w", err)
	}

	return nil
}

// GetCurrentBranch returns the current branch name.
func (r *Repository) GetCurrentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = r.Path

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// HasStagedChanges checks if there are any staged changes.
func (r *Repository) HasStagedChanges(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--quiet")
	cmd.Dir = r.Path

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 means there are changes
			if exitErr.ExitCode() == 1 {
				return true, nil
			}
		}
		return false, fmt.Errorf("failed to check staged changes: %w", err)
	}

	// Exit code 0 means no changes
	return false, nil
}

// GetLastCommitMessage returns the last commit message.
func (r *Repository) GetLastCommitMessage(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "log", "-1", "--pretty=format:%B")
	cmd.Dir = r.Path

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get last commit message: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetFileContent returns the content of a file at a specific revision.
func (r *Repository) GetFileContent(ctx context.Context, path string, revision string) (string, error) {
	if revision == "" {
		revision = "HEAD"
	}

	cmd := exec.CommandContext(ctx, "git", "show", fmt.Sprintf("%s:%s", revision, path))
	cmd.Dir = r.Path

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get file content: %w", err)
	}

	return string(output), nil
}

// IsFileTracked checks if a file is tracked by git.
func (r *Repository) IsFileTracked(ctx context.Context, path string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--error-unmatch", path)
	cmd.Dir = r.Path

	err := cmd.Run()
	return err == nil, nil
}

// GetRemoteURL returns the URL of the origin remote.
func (r *Repository) GetRemoteURL(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
	cmd.Dir = r.Path

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get remote URL: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// CheckHooksExist checks if git hooks exist in the repository.
func (r *Repository) CheckHooksExist() (map[string]bool, error) {
	rootPath, err := r.GetRootPath()
	if err != nil {
		return nil, err
	}

	hooksPath := filepath.Join(rootPath, ".git", "hooks")
	hooks := make(map[string]bool)

	hookNames := []string{"pre-commit", "commit-msg", "post-commit"}
	for _, hookName := range hookNames {
		hookPath := filepath.Join(hooksPath, hookName)
		if info, err := os.Stat(hookPath); err == nil && !info.IsDir() {
			hooks[hookName] = true
		} else {
			hooks[hookName] = false
		}
	}

	return hooks, nil
}