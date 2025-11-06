package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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

// CommitInfo represents information about a git commit.
type CommitInfo struct {
	SHA     string
	Message string
	Diff    string
}

// GetCommitRange returns commits between two refs with their diffs.
func (r *Repository) GetCommitRange(ctx context.Context, from, to string) ([]CommitInfo, error) {
	// Get commit SHAs in the range.
	cmd := exec.CommandContext(ctx, "git", "rev-list", fmt.Sprintf("%s..%s", from, to))
	cmd.Dir = r.Path

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get commit range: %w", err)
	}

	if len(output) == 0 {
		return []CommitInfo{}, nil
	}

	shas := strings.Split(strings.TrimSpace(string(output)), "\n")
	var commits []CommitInfo

	// Get info for each commit.
	for i := len(shas) - 1; i >= 0; i-- { // Reverse to get chronological order.
		sha := shas[i]
		if sha == "" {
			continue
		}

		// Get commit message.
		msgCmd := exec.CommandContext(ctx, "git", "log", "-1", "--pretty=format:%B", sha)
		msgCmd.Dir = r.Path
		msgOutput, err := msgCmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get commit message for %s: %w", sha, err)
		}

		// Get commit diff.
		diffCmd := exec.CommandContext(ctx, "git", "diff", fmt.Sprintf("%s^", sha), sha)
		diffCmd.Dir = r.Path
		diffOutput, err := diffCmd.Output()
		if err != nil {
			// For the first commit, there might not be a parent.
			diffCmd = exec.CommandContext(ctx, "git", "diff", "--root", sha)
			diffCmd.Dir = r.Path
			diffOutput, err = diffCmd.Output()
			if err != nil {
				return nil, fmt.Errorf("failed to get commit diff for %s: %w", sha, err)
			}
		}

		commits = append(commits, CommitInfo{
			SHA:     sha,
			Message: strings.TrimSpace(string(msgOutput)),
			Diff:    string(diffOutput),
		})
	}

	return commits, nil
}

// GetUnpushedCommits returns commits that haven't been pushed to origin.
func (r *Repository) GetUnpushedCommits(ctx context.Context) ([]CommitInfo, error) {
	// Get current branch.
	branch, err := r.GetCurrentBranch(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	// Check if remote branch exists.
	checkCmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", fmt.Sprintf("origin/%s", branch))
	checkCmd.Dir = r.Path
	if err := checkCmd.Run(); err != nil {
		// Remote branch doesn't exist, get all commits since main/master.
		return r.GetCommitsFromBranchPoint(ctx)
	}

	// Get unpushed commits.
	return r.GetCommitRange(ctx, fmt.Sprintf("origin/%s", branch), "HEAD")
}

// GetBranchPoint finds where current branch diverged from main/master.
func (r *Repository) GetBranchPoint(ctx context.Context) (string, error) {
	// Try to find main branch first, then master.
	for _, baseBranch := range []string{"main", "master"} {
		// Check if base branch exists.
		checkCmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", fmt.Sprintf("origin/%s", baseBranch))
		checkCmd.Dir = r.Path
		if err := checkCmd.Run(); err != nil {
			continue
		}

		// Find merge base.
		cmd := exec.CommandContext(ctx, "git", "merge-base", fmt.Sprintf("origin/%s", baseBranch), "HEAD")
		cmd.Dir = r.Path
		output, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(output)), nil
		}
	}

	// If no main/master, use the root commit.
	cmd := exec.CommandContext(ctx, "git", "rev-list", "--max-parents=0", "HEAD")
	cmd.Dir = r.Path
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to find branch point: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetCurrentCommitSHA returns the SHA of the current HEAD.
func (r *Repository) GetCurrentCommitSHA(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = r.Path

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current commit SHA: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// HasUncommittedChanges checks if there are any uncommitted changes (staged or unstaged).
func (r *Repository) HasUncommittedChanges(ctx context.Context) (bool, error) {
	// Check for any changes (staged or unstaged)
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = r.Path

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check for uncommitted changes: %w", err)
	}

	// If output is not empty, there are uncommitted changes
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// Stash saves the current working directory and index state.
func (r *Repository) Stash(ctx context.Context, message string) (string, error) {
	if message == "" {
		message = "cmt absorb auto-stash"
	}

	cmd := exec.CommandContext(ctx, "git", "stash", "push", "-m", message, "--include-untracked")
	cmd.Dir = r.Path

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("git stash failed: %s", stderr.String())
		}
		return "", fmt.Errorf("git stash failed: %w", err)
	}

	// Get the stash SHA for reference
	stashCmd := exec.CommandContext(ctx, "git", "rev-parse", "stash@{0}")
	stashCmd.Dir = r.Path
	output, err := stashCmd.Output()
	if err != nil {
		// Stash was created but we couldn't get the SHA, not critical
		return "", nil
	}

	return strings.TrimSpace(string(output)), nil
}

// StashPop applies the latest stash and removes it from the stash list.
func (r *Repository) StashPop(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "stash", "pop")
	cmd.Dir = r.Path

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			// If there's a conflict, git will report it in stderr
			if strings.Contains(stderr.String(), "conflict") {
				return fmt.Errorf("git stash pop had conflicts: %s", stderr.String())
			}
			return fmt.Errorf("git stash pop failed: %s", stderr.String())
		}
		return fmt.Errorf("git stash pop failed: %w", err)
	}

	return nil
}
// GetCommitsFromBranchPoint returns commits from branch point to HEAD.
func (r *Repository) GetCommitsFromBranchPoint(ctx context.Context) ([]CommitInfo, error) {
	branchPoint, err := r.GetBranchPoint(ctx)
	if err != nil {
		return nil, err
	}

	return r.GetCommitRange(ctx, branchPoint, "HEAD")
}

// CreateFixupCommit creates a fixup commit for the target SHA.
func (r *Repository) CreateFixupCommit(ctx context.Context, targetSHA string, message string) error {
	// If message is provided, use it; otherwise use default fixup format.
	if message == "" {
		message = fmt.Sprintf("fixup! %s", targetSHA[:7])
	}

	cmd := exec.CommandContext(ctx, "git", "commit", "--fixup", targetSHA, "-m", message)
	cmd.Dir = r.Path

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("failed to create fixup commit: %s", stderr.String())
		}
		return fmt.Errorf("failed to create fixup commit: %w", err)
	}

	return nil
}

// AutosquashRebase performs an autosquash rebase onto the specified commit.
func (r *Repository) AutosquashRebase(ctx context.Context, onto string) error {
	cmd := exec.CommandContext(ctx, "git", "rebase", "--autosquash", "-i", "--autostash", onto)
	cmd.Dir = r.Path

	// Set environment variable to automatically accept the rebase todo list.
	cmd.Env = append(os.Environ(), "GIT_SEQUENCE_EDITOR=true")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("autosquash rebase failed: %s", stderr.String())
		}
		return fmt.Errorf("autosquash rebase failed: %w", err)
	}

	return nil
}

// CheckRebaseConflicts checks if rebasing would cause conflicts.
func (r *Repository) CheckRebaseConflicts(ctx context.Context, commits []string) (bool, []string, error) {
	// Create a temporary branch to test rebase.
	tempBranch := fmt.Sprintf("cmt-absorb-test-%d", os.Getpid())

	// Save current branch.
	currentBranch, err := r.GetCurrentBranch(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	// Create temp branch.
	createCmd := exec.CommandContext(ctx, "git", "checkout", "-b", tempBranch)
	createCmd.Dir = r.Path
	if err := createCmd.Run(); err != nil {
		return false, nil, fmt.Errorf("failed to create temp branch: %w", err)
	}

	// Ensure we clean up.
	defer func() {
		// Switch back to original branch.
		switchCmd := exec.Command("git", "checkout", currentBranch)
		switchCmd.Dir = r.Path
		switchCmd.Run()

		// Delete temp branch.
		deleteCmd := exec.Command("git", "branch", "-D", tempBranch)
		deleteCmd.Dir = r.Path
		deleteCmd.Run()
	}()

	// Try to perform the rebase.
	var conflictFiles []string
	hasConflicts := false

	for _, commit := range commits {
		rebaseCmd := exec.CommandContext(ctx, "git", "rebase", commit)
		rebaseCmd.Dir = r.Path

		if err := rebaseCmd.Run(); err != nil {
			hasConflicts = true

			// Get list of conflicted files.
			statusCmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "--diff-filter=U")
			statusCmd.Dir = r.Path
			output, _ := statusCmd.Output()

			if len(output) > 0 {
				files := strings.Split(strings.TrimSpace(string(output)), "\n")
				conflictFiles = append(conflictFiles, files...)
			}

			// Abort the rebase.
			abortCmd := exec.CommandContext(ctx, "git", "rebase", "--abort")
			abortCmd.Dir = r.Path
			abortCmd.Run()

			break
		}
	}

	return hasConflicts, conflictFiles, nil
}

// CreateBackupRef creates a backup ref in the custom namespace from current HEAD.
// Instead of creating a regular branch that pollutes `git branch`, this creates
// a ref in refs/cmt-backup/ that is still tracked by Git but doesn't clutter branches.
func (r *Repository) CreateBackupRef(ctx context.Context, name string) (string, error) {
	if name == "" {
		name = fmt.Sprintf("absorb-%d", time.Now().Unix())
	}

	// Use custom refs namespace to avoid polluting branch list
	refPath := fmt.Sprintf("refs/cmt-backup/%s", name)

	// Create the ref pointing to HEAD
	cmd := exec.CommandContext(ctx, "git", "update-ref", refPath, "HEAD")
	cmd.Dir = r.Path

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create backup ref: %w", err)
	}

	return refPath, nil
}

// ListBackupRefs lists all backup refs in the custom namespace.
func (r *Repository) ListBackupRefs(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "show-ref", "--heads", "refs/cmt-backup/")
	cmd.Dir = r.Path

	output, err := cmd.Output()
	if err != nil {
		// No refs found is not an error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to list backup refs: %w", err)
	}

	var refs []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Format: "<sha> refs/cmt-backup/absorb-123456"
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			refs = append(refs, parts[1])
		}
	}

	return refs, nil
}

// DeleteBackupRef deletes a backup ref from the custom namespace.
func (r *Repository) DeleteBackupRef(ctx context.Context, refPath string) error {
	cmd := exec.CommandContext(ctx, "git", "update-ref", "-d", refPath)
	cmd.Dir = r.Path

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete backup ref %s: %w", refPath, err)
	}

	return nil
}

// GetCommitDiff returns the diff for a specific commit.
func (r *Repository) GetCommitDiff(ctx context.Context, sha string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", fmt.Sprintf("%s^", sha), sha)
	cmd.Dir = r.Path

	output, err := cmd.Output()
	if err != nil {
		// For the first commit, there might not be a parent.
		cmd = exec.CommandContext(ctx, "git", "diff", "--root", sha)
		cmd.Dir = r.Path
		output, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get commit diff: %w", err)
		}
	}

	return string(output), nil
}

// GetCommitMessage returns the message for a specific commit.
func (r *Repository) GetCommitMessage(ctx context.Context, sha string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "log", "-1", "--pretty=format:%B", sha)
	cmd.Dir = r.Path

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit message: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}