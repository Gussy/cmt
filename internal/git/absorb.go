package git

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Hunk represents a single diff hunk.
type Hunk struct {
	// File information.
	FilePath    string
	OldFilePath string // For renames.
	IsNew       bool
	IsDeleted   bool
	IsRenamed   bool

	// Hunk content.
	Header  string   // The @@ line.
	Content string   // The full hunk including header.
	Lines   []string // Individual lines of the hunk.

	// Context for matching.
	ContextBefore []string // Lines before the change.
	ContextAfter  []string // Lines after the change.
	AddedLines    []string // Lines that were added.
	RemovedLines  []string // Lines that were removed.

	// Line numbers.
	OldStartLine int
	OldLineCount int
	NewStartLine int
	NewLineCount int
}

// HunkAssignment represents the assignment of a hunk to a commit.
type HunkAssignment struct {
	Hunk       *Hunk
	CommitSHA  string
	Confidence float64 // 0.0 to 1.0.
}

// AbsorbState represents the state for undo operations.
type AbsorbState struct {
	OriginalHEAD  string
	BackupRef     string   // Full ref path (e.g., refs/cmt-backup/absorb-123456)
	Operations    []string // List of operations performed.
	Timestamp     int64
	CurrentBranch string
	StashSHA      string // SHA of stash if uncommitted changes were saved.
}

// SplitDiffIntoHunks parses a diff string into individual hunks.
func SplitDiffIntoHunks(diff string) ([]Hunk, error) {
	var hunks []Hunk
	scanner := bufio.NewScanner(strings.NewReader(diff))

	var currentFile string
	var oldFile string
	var isNew, isDeleted, isRenamed bool
	var currentHunk *Hunk
	var inHunk bool

	for scanner.Scan() {
		line := scanner.Text()

		// File header: diff --git a/file b/file.
		if strings.HasPrefix(line, "diff --git") {
			// Save previous hunk if exists.
			if currentHunk != nil && inHunk {
				hunks = append(hunks, *currentHunk)
				currentHunk = nil
				inHunk = false
			}

			// Parse file paths.
			parts := strings.Split(line, " ")
			if len(parts) >= 4 {
				// Remove a/ and b/ prefixes.
				aFile := strings.TrimPrefix(parts[2], "a/")
				bFile := strings.TrimPrefix(parts[3], "b/")
				currentFile = bFile
				oldFile = aFile
			}

			isNew = false
			isDeleted = false
			isRenamed = false
		}

		// File status headers.
		if strings.HasPrefix(line, "new file mode") {
			isNew = true
		} else if strings.HasPrefix(line, "deleted file mode") {
			isDeleted = true
		} else if strings.HasPrefix(line, "rename from") {
			isRenamed = true
			oldFile = strings.TrimPrefix(line, "rename from ")
		} else if strings.HasPrefix(line, "rename to") {
			currentFile = strings.TrimPrefix(line, "rename to ")
		}

		// Hunk header: @@ -old,count +new,count @@.
		if strings.HasPrefix(line, "@@") && strings.Contains(line, "@@") {
			// Save previous hunk if exists.
			if currentHunk != nil && inHunk {
				hunks = append(hunks, *currentHunk)
			}

			// Parse hunk header.
			currentHunk = &Hunk{
				FilePath:    currentFile,
				OldFilePath: oldFile,
				IsNew:       isNew,
				IsDeleted:   isDeleted,
				IsRenamed:   isRenamed,
				Header:      line,
				Content:     line + "\n",
			}

			// Parse line numbers.
			if err := parseHunkHeader(line, currentHunk); err != nil {
				return nil, fmt.Errorf("failed to parse hunk header: %w", err)
			}

			inHunk = true
		} else if inHunk && currentHunk != nil {
			// Add line to current hunk.
			currentHunk.Content += line + "\n"
			currentHunk.Lines = append(currentHunk.Lines, line)

			// Categorize line.
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				currentHunk.AddedLines = append(currentHunk.AddedLines, strings.TrimPrefix(line, "+"))
			} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
				currentHunk.RemovedLines = append(currentHunk.RemovedLines, strings.TrimPrefix(line, "-"))
			} else if strings.HasPrefix(line, " ") {
				// Context line.
				if len(currentHunk.AddedLines) == 0 && len(currentHunk.RemovedLines) == 0 {
					currentHunk.ContextBefore = append(currentHunk.ContextBefore, strings.TrimPrefix(line, " "))
				} else {
					currentHunk.ContextAfter = append(currentHunk.ContextAfter, strings.TrimPrefix(line, " "))
				}
			}
		}
	}

	// Save last hunk if exists.
	if currentHunk != nil && inHunk {
		hunks = append(hunks, *currentHunk)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan diff: %w", err)
	}

	return hunks, nil
}

// parseHunkHeader parses the @@ line to extract line numbers.
func parseHunkHeader(header string, hunk *Hunk) error {
	// Format: @@ -old_start,old_count +new_start,new_count @@ optional context.
	// Example: @@ -10,5 +10,7 @@.

	// Find the @@ markers.
	startIdx := strings.Index(header, "@@")
	endIdx := strings.LastIndex(header, "@@")
	if startIdx == -1 || endIdx == -1 || startIdx == endIdx {
		return fmt.Errorf("invalid hunk header format: %s", header)
	}

	// Extract the line number part.
	lineInfo := header[startIdx+2 : endIdx]
	lineInfo = strings.TrimSpace(lineInfo)

	// Split into old and new parts.
	parts := strings.Fields(lineInfo)
	if len(parts) != 2 {
		return fmt.Errorf("invalid line info format: %s", lineInfo)
	}

	// Parse old lines (-start,count).
	oldPart := strings.TrimPrefix(parts[0], "-")
	if err := parseLineRange(oldPart, &hunk.OldStartLine, &hunk.OldLineCount); err != nil {
		return fmt.Errorf("failed to parse old line range: %w", err)
	}

	// Parse new lines (+start,count).
	newPart := strings.TrimPrefix(parts[1], "+")
	if err := parseLineRange(newPart, &hunk.NewStartLine, &hunk.NewLineCount); err != nil {
		return fmt.Errorf("failed to parse new line range: %w", err)
	}

	return nil
}

// parseLineRange parses a line range like "10,5" or "10".
func parseLineRange(rangeStr string, start *int, count *int) error {
	parts := strings.Split(rangeStr, ",")

	// Parse start line.
	var err error
	*start = 0
	if len(parts) >= 1 && parts[0] != "" {
		if _, err = fmt.Sscanf(parts[0], "%d", start); err != nil {
			return fmt.Errorf("invalid start line: %s", parts[0])
		}
	}

	// Parse line count (default to 1 if not specified).
	*count = 1
	if len(parts) >= 2 {
		if _, err = fmt.Sscanf(parts[1], "%d", count); err != nil {
			return fmt.Errorf("invalid line count: %s", parts[1])
		}
	}

	return nil
}

// ApplyHunksAsFixup creates a fixup commit with specific hunks.
func (r *Repository) ApplyHunksAsFixup(ctx context.Context, hunks []Hunk, targetSHA string) error {
	if len(hunks) == 0 {
		return fmt.Errorf("no hunks to apply")
	}

	// Create a patch file with the selected hunks.
	patchFile, err := createPatchFile(hunks)
	if err != nil {
		return fmt.Errorf("failed to create patch file: %w", err)
	}
	defer os.Remove(patchFile)

	// Reset the working directory to remove the hunks we're about to apply.
	for _, hunk := range hunks {
		// Check out the file from HEAD to reset it.
		cmd := exec.CommandContext(ctx, "git", "checkout", "HEAD", "--", hunk.FilePath)
		cmd.Dir = r.Path
		if err := cmd.Run(); err != nil {
			// File might be new, that's okay.
			if !hunk.IsNew {
				return fmt.Errorf("failed to reset file %s: %w", hunk.FilePath, err)
			}
		}
	}

	// Apply the patch to both working directory and staging area.
	// Using --index applies to both at once.
	applyCmd := exec.CommandContext(ctx, "git", "apply", "--index", patchFile)
	applyCmd.Dir = r.Path
	if err := applyCmd.Run(); err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	// Create fixup commit.
	return r.CreateFixupCommit(ctx, targetSHA, "")
}

// createPatchFile creates a temporary patch file from hunks.
func createPatchFile(hunks []Hunk) (string, error) {
	// Create temp file.
	tmpFile, err := os.CreateTemp("", "cmt-absorb-*.patch")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	// Group hunks by file.
	fileHunks := make(map[string][]Hunk)
	for _, hunk := range hunks {
		fileHunks[hunk.FilePath] = append(fileHunks[hunk.FilePath], hunk)
	}

	// Write patch content.
	for file, hunks := range fileHunks {
		// Write file header.
		fmt.Fprintf(tmpFile, "diff --git a/%s b/%s\n", file, file)

		// Handle file status.
		if hunks[0].IsNew {
			fmt.Fprintf(tmpFile, "new file mode 100644\n")
		} else if hunks[0].IsDeleted {
			fmt.Fprintf(tmpFile, "deleted file mode 100644\n")
		} else if hunks[0].IsRenamed {
			fmt.Fprintf(tmpFile, "rename from %s\n", hunks[0].OldFilePath)
			fmt.Fprintf(tmpFile, "rename to %s\n", file)
		}

		// Write index line (simplified).
		fmt.Fprintf(tmpFile, "--- a/%s\n", file)
		fmt.Fprintf(tmpFile, "+++ b/%s\n", file)

		// Write each hunk.
		for _, hunk := range hunks {
			fmt.Fprint(tmpFile, hunk.Content)
		}
	}

	return tmpFile.Name(), nil
}

// SaveAbsorbState saves the current state for undo operations.
func SaveAbsorbState(repo *Repository, state *AbsorbState) error {
	rootPath, err := repo.GetRootPath()
	if err != nil {
		return fmt.Errorf("failed to get repository root: %w", err)
	}

	// Create .git/cmt directory if it doesn't exist.
	cmtDir := filepath.Join(rootPath, ".git", "cmt")
	if err := os.MkdirAll(cmtDir, 0755); err != nil {
		return fmt.Errorf("failed to create cmt directory: %w", err)
	}

	// Save state to file.
	stateFile := filepath.Join(cmtDir, "absorb-undo")
	file, err := os.Create(stateFile)
	if err != nil {
		return fmt.Errorf("failed to create state file: %w", err)
	}
	defer file.Close()

	// Write state information.
	fmt.Fprintf(file, "original_head=%s\n", state.OriginalHEAD)
	fmt.Fprintf(file, "backup_ref=%s\n", state.BackupRef)
	fmt.Fprintf(file, "current_branch=%s\n", state.CurrentBranch)
	fmt.Fprintf(file, "timestamp=%d\n", state.Timestamp)
	if state.StashSHA != "" {
		fmt.Fprintf(file, "stash_sha=%s\n", state.StashSHA)
	}

	for _, op := range state.Operations {
		fmt.Fprintf(file, "operation=%s\n", op)
	}

	return nil
}

// LoadAbsorbState loads the saved absorb state for undo operations.
func LoadAbsorbState(repo *Repository) (*AbsorbState, error) {
	rootPath, err := repo.GetRootPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get repository root: %w", err)
	}

	stateFile := filepath.Join(rootPath, ".git", "cmt", "absorb-undo")
	file, err := os.Open(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no absorb state found to undo")
		}
		return nil, fmt.Errorf("failed to open state file: %w", err)
	}
	defer file.Close()

	state := &AbsorbState{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		switch key {
		case "original_head":
			state.OriginalHEAD = value
		case "backup_ref":
			state.BackupRef = value
		case "current_branch":
			state.CurrentBranch = value
		case "timestamp":
			fmt.Sscanf(value, "%d", &state.Timestamp)
		case "stash_sha":
			state.StashSHA = value
		case "operation":
			state.Operations = append(state.Operations, value)
		// Ignore old backup_branch entries for graceful migration
		case "backup_branch":
			// Skip - no longer supported
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	return state, nil
}

// UndoAbsorb reverts the last absorb operation.
func (r *Repository) UndoAbsorb(ctx context.Context) error {
	// Load saved state.
	state, err := LoadAbsorbState(r)
	if err != nil {
		return err
	}

	// Ensure we have a backup ref.
	if state.BackupRef == "" {
		return fmt.Errorf("no backup reference found in state")
	}

	// Switch to the original branch.
	checkoutCmd := exec.CommandContext(ctx, "git", "checkout", state.CurrentBranch)
	checkoutCmd.Dir = r.Path
	if err := checkoutCmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout original branch: %w", err)
	}

	// Reset to the backup ref using --mixed to preserve working directory changes.
	resetCmd := exec.CommandContext(ctx, "git", "reset", "--mixed", state.BackupRef)
	resetCmd.Dir = r.Path
	if err := resetCmd.Run(); err != nil {
		return fmt.Errorf("failed to reset to backup: %w", err)
	}

	// Restore stash if it was saved.
	if state.StashSHA != "" {
		// Try to pop the stash.
		if err := r.StashPop(ctx); err != nil {
			// Non-fatal: warn but continue
			fmt.Printf("⚠️  Warning: Could not restore stashed changes: %v\n", err)
			fmt.Println("   You may need to manually run: git stash pop")
		}
	}

	// Delete the backup ref.
	if err := r.DeleteBackupRef(ctx, state.BackupRef); err != nil {
		// Non-fatal warning
		fmt.Printf("⚠️  Warning: Could not delete backup ref: %v\n", err)
	}

	// Remove the state file.
	rootPath, _ := r.GetRootPath()
	stateFile := filepath.Join(rootPath, ".git", "cmt", "absorb-undo")
	os.Remove(stateFile)

	return nil
}
