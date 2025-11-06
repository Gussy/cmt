package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gussy/cmt/internal/ai"
	"github.com/gussy/cmt/internal/config"
	"github.com/gussy/cmt/internal/git"
	"github.com/gussy/cmt/internal/ui"
	"github.com/urfave/cli/v3"
)

// absorbCommand creates the absorb subcommand.
func absorbCommand() *cli.Command {
	return &cli.Command{
		Name:    "absorb",
		Aliases: []string{"a"},
		Usage:   "Intelligently absorb staged changes into previous commits",
		Description: `The absorb command uses AI to analyze staged changes and automatically
assign them to the most relevant previous commits, similar to git-absorb but
with semantic understanding. It creates fixup commits that can be autosquashed.`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "yes",
				Aliases: []string{"y"},
				Usage:   "Skip interactive review and apply assignments automatically",
			},
			&cli.BoolFlag{
				Name:  "dry-run",
				Usage: "Preview assignments without making any changes",
			},
			&cli.IntFlag{
				Name:    "depth",
				Aliases: []string{"d"},
				Usage:   "Number of commits to analyze (from HEAD)",
			},
			&cli.BoolFlag{
				Name:  "to-branch-point",
				Usage: "Analyze all commits back to where branch diverged from main/master",
			},
			&cli.BoolFlag{
				Name:  "no-new-commit",
				Usage: "Don't create a new commit for unmatched hunks",
			},
			&cli.StringFlag{
				Name:    "model",
				Aliases: []string{"m"},
				Usage:   "AI model to use for analysis",
			},
			&cli.BoolFlag{
				Name:  "rebase",
				Usage: "Automatically perform autosquash rebase after creating fixup commits",
			},
			&cli.BoolFlag{
				Name:  "undo",
				Usage: "Undo the last absorb operation",
			},
			&cli.BoolFlag{
				Name:  "list-backups",
				Usage: "List all backup refs and old backup branches",
			},
			&cli.BoolFlag{
				Name:  "cleanup-backups",
				Usage: "Clean up old backup refs and branches",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			// Handle special operations.
			if cmd.Bool("undo") {
				return runAbsorbUndo(ctx)
			}
			if cmd.Bool("list-backups") {
				return runListBackups(ctx)
			}
			if cmd.Bool("cleanup-backups") {
				return runCleanupBackups(ctx)
			}
			return runAbsorb(ctx, cmd)
		},
	}
}

// runAbsorb executes the absorb workflow.
func runAbsorb(ctx context.Context, cmd *cli.Command) error {
	// Load configuration.
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize git repository.
	repo, err := git.NewRepository("")
	if err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	// Step 1: Check for staged changes.
	ui.SimpleProgress("Checking for staged changes...")
	hasChanges, err := repo.HasStagedChanges(ctx)
	if err != nil {
		return fmt.Errorf("failed to check staged changes: %w", err)
	}

	if !hasChanges {
		fmt.Println("❌ No staged changes to absorb.")
		fmt.Println("\nUse 'git add' to stage the changes you want to absorb.")
		return nil
	}

	// Step 2: Determine commit range.
	ui.SimpleProgress("Determining commit range...")
	var commits []git.CommitInfo

	if cmd.Bool("to-branch-point") || cfg.AbsorbRange == "branch-point" {
		// Get all commits from branch point.
		commits, err = repo.GetCommitsFromBranchPoint(ctx)
		if err != nil {
			return fmt.Errorf("failed to get commits from branch point: %w", err)
		}
	} else if depth := cmd.Int("depth"); depth > 0 {
		// Get specific number of commits.
		commits, err = repo.GetCommitRange(ctx, fmt.Sprintf("HEAD~%d", depth), "HEAD")
		if err != nil {
			return fmt.Errorf("failed to get commit range: %w", err)
		}
	} else {
		// Default: get unpushed commits.
		commits, err = repo.GetUnpushedCommits(ctx)
		if err != nil {
			return fmt.Errorf("failed to get unpushed commits: %w", err)
		}
	}

	if len(commits) == 0 {
		fmt.Println("❌ No commits found in the specified range.")
		fmt.Println("\nThe absorb command needs existing commits to absorb changes into.")
		fmt.Println("Try using --to-branch-point or --depth to expand the range.")
		return nil
	}

	fmt.Printf("📝 Found %d commit(s) to analyze\n", len(commits))

	// Step 3: Get staged diff and split into hunks.
	ui.SimpleProgress("Analyzing staged changes...")
	diff, err := repo.GetDiff(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to get diff: %w", err)
	}

	hunks, err := git.SplitDiffIntoHunks(diff)
	if err != nil {
		return fmt.Errorf("failed to split diff into hunks: %w", err)
	}

	if len(hunks) == 0 {
		fmt.Println("❌ No hunks found in staged changes.")
		return nil
	}

	fmt.Printf("🔍 Found %d hunk(s) to absorb\n", len(hunks))

	// Step 4: Check for potential conflicts (unless dry-run).
	if !cmd.Bool("dry-run") {
		ui.SimpleProgress("Checking for potential conflicts...")
		shas := make([]string, len(commits))
		for i, c := range commits {
			shas[i] = c.SHA
		}

		hasConflicts, conflictFiles, err := repo.CheckRebaseConflicts(ctx, shas)
		if err != nil {
			return fmt.Errorf("failed to check for conflicts: %w", err)
		}

		if hasConflicts {
			fmt.Println("\n⚠️  Warning: Absorbing these changes may cause rebase conflicts")
			fmt.Printf("   Conflicted files: %s\n", strings.Join(conflictFiles, ", "))
			if !cmd.Bool("yes") {
				fmt.Print("\nDo you want to continue anyway? (y/n): ")
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "yes" {
					fmt.Println("❌ Absorb cancelled.")
					return nil
				}
			}
		}
	}

	// Step 5: Initialize AI provider.
	ui.SimpleProgress("Initializing AI provider...")
	model := cmd.String("model")
	if model == "" {
		model = cfg.Model
	}

	providerCfg := &ai.ProviderConfig{
		DefaultModel: model,
		Timeout:      60,
	}

	provider, err := ai.NewClaudeCLI(providerCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize AI provider: %w", err)
	}

	// Check if provider is available.
	available, err := provider.IsAvailable(ctx)
	if err != nil || !available {
		return fmt.Errorf("AI provider is not available: %w", err)
	}

	// Step 6: Analyze hunk assignments with AI.
	ui.SimpleProgress("Analyzing hunk assignments with AI...")

	// Determine strategy from config.
	strategy := cfg.AbsorbAmbiguity
	if strategy == "" {
		strategy = "interactive"
	}

	// Get confidence threshold from config.
	confidence := cfg.AbsorbConfidence
	if confidence == 0 {
		confidence = 0.7
	}

	absorbReq := &ai.AbsorbRequest{
		Hunks:               hunks,
		Commits:             commits,
		Strategy:            strategy,
		ConfidenceThreshold: confidence,
		Model:               model,
		Temperature:         cfg.Temperature,
		MaxTokens:           cfg.MaxTokens,
	}

	absorbResp, err := provider.AnalyzeHunkAssignment(ctx, absorbReq)
	if err != nil {
		return fmt.Errorf("failed to analyze hunk assignments: %w", err)
	}

	// Step 7: Show analysis results.
	fmt.Println("\n📊 Analysis Results:")
	fmt.Println("=" + strings.Repeat("=", 40))

	if len(absorbResp.Assignments) > 0 {
		fmt.Printf("\n✅ Assigned hunks: %d\n", len(absorbResp.Assignments))
		for _, assignment := range absorbResp.Assignments {
			fmt.Printf("   • %s → %s: %.1f%% confidence\n",
				assignment.Hunk.FilePath,
				assignment.CommitSHA[:8],
				assignment.Confidence*100)
			if assignment.Reasoning != "" && cfg.Verbose {
				fmt.Printf("     Reason: %s\n", assignment.Reasoning)
			}
		}
	}

	if len(absorbResp.UnmatchedHunks) > 0 {
		fmt.Printf("\n❓ Unmatched hunks: %d\n", len(absorbResp.UnmatchedHunks))
		for _, hunk := range absorbResp.UnmatchedHunks {
			fmt.Printf("   • %s\n", hunk.FilePath)
		}
	}

	// Step 8: Interactive review (unless --yes or dry-run).
	if !cmd.Bool("yes") && !cmd.Bool("dry-run") && len(absorbResp.Assignments) > 0 {
		// Show interactive UI for reviewing assignments.
		accepted, modifiedAssignments, err := ui.ShowAbsorbReview(absorbResp, commits)
		if err != nil {
			return fmt.Errorf("failed to show absorb review: %w", err)
		}

		if !accepted {
			fmt.Println("\n❌ Absorb cancelled.")
			return nil
		}

		// Use modified assignments if user made changes.
		if modifiedAssignments != nil {
			absorbResp = modifiedAssignments
		}
	}

	// Step 9: Dry-run mode - show plan and exit.
	if cmd.Bool("dry-run") {
		fmt.Println("\n🔍 DRY RUN - No changes will be made")
		fmt.Println("\nPlan:")
		for _, assignment := range absorbResp.Assignments {
			fmt.Printf("• Create fixup commit for %s with hunks from %s\n",
				assignment.CommitSHA[:8], assignment.Hunk.FilePath)
		}

		if len(absorbResp.UnmatchedHunks) > 0 && !cmd.Bool("no-new-commit") {
			fmt.Printf("• Create new commit with %d unmatched hunk(s)\n",
				len(absorbResp.UnmatchedHunks))
		}

		if cmd.Bool("rebase") || cfg.AbsorbStrategy == "direct" {
			fmt.Println("• Perform autosquash rebase")
		}

		return nil
	}

	// Step 10: Apply assignments (create fixup commits).
	if len(absorbResp.Assignments) > 0 {
		ui.SimpleProgress("Creating fixup commits...")

		// Group hunks by target commit.
		commitHunks := make(map[string][]git.Hunk)
		for _, assignment := range absorbResp.Assignments {
			commitHunks[assignment.CommitSHA] = append(
				commitHunks[assignment.CommitSHA],
				assignment.Hunk,
			)
		}

		// Create fixup commit for each target.
		for sha, hunks := range commitHunks {
			if err := repo.ApplyHunksAsFixup(ctx, hunks, sha); err != nil {
				return fmt.Errorf("failed to create fixup commit for %s: %w", sha[:8], err)
			}
			fmt.Printf("✅ Created fixup commit for %s\n", sha[:8])
		}
	}

	// Step 11: Create backup ref AFTER fixup commits to capture the correct state.
	// Uses custom refs namespace to avoid polluting branch list.
	ui.SimpleProgress("Creating backup...")
	backupName := fmt.Sprintf("absorb-%d", time.Now().Unix())
	backupRef, err := repo.CreateBackupRef(ctx, backupName)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	fmt.Printf("✅ Created backup: %s\n", backupName)

	// Step 12: Handle unmatched hunks.
	if len(absorbResp.UnmatchedHunks) > 0 && !cmd.Bool("no-new-commit") {
		if cfg.AbsorbAutoCommit {
			ui.SimpleProgress("Creating commit for unmatched hunks...")

			// Re-stage the unmatched hunks.
			for _, _ = range absorbResp.UnmatchedHunks {
				// The hunks should still be staged if they weren't absorbed.
			}

			// Check if there are still staged changes.
			hasChanges, _ := repo.HasStagedChanges(ctx)
			if hasChanges {
				// Generate commit message for unmatched hunks.
				fmt.Println("📝 Generating commit message for unmatched hunks...")

				// Use the regular commit message generation.
				diff, _ := repo.GetDiff(ctx, true)
				stagedFiles, _ := repo.GetStagedFiles(ctx)

				commitReq := &ai.CommitRequest{
					Diff:        diff,
					StagedFiles: stagedFiles,
					Model:       model,
					Temperature: cfg.Temperature,
					MaxTokens:   cfg.MaxTokens,
				}

				commitResp, err := provider.GenerateCommitMessage(ctx, commitReq)
				if err != nil {
					return fmt.Errorf("failed to generate commit message: %w", err)
				}

				// Create the commit.
				if err := repo.Commit(ctx, commitResp.Message); err != nil {
					return fmt.Errorf("failed to create commit: %w", err)
				}
				fmt.Printf("✅ Created commit for unmatched hunks: %s\n",
					strings.Split(commitResp.Message, "\n")[0])
			}
		}
	}

	// Step 13: Save absorb state for undo.
	currentBranch, _ := repo.GetCurrentBranch(ctx)
	// Get actual HEAD SHA instead of string "HEAD" for proper restoration.
	headSHA, err := repo.GetCurrentCommitSHA(ctx)
	if err != nil {
		// Fallback to using backup ref as reference
		headSHA = backupRef
	}
	state := &git.AbsorbState{
		OriginalHEAD:  headSHA,
		BackupRef:     backupRef,  // Use new ref format
		CurrentBranch: currentBranch,
		Timestamp:     time.Now().Unix(),
		Operations: []string{
			fmt.Sprintf("Created %d fixup commits", len(absorbResp.Assignments)),
			fmt.Sprintf("Backup ref: %s", backupRef),
		},
	}

	if err := git.SaveAbsorbState(repo, state); err != nil {
		// Non-fatal error.
		fmt.Printf("⚠️  Warning: Failed to save undo state: %v\n", err)
	}

	// Step 14: Perform rebase if requested.
	if cmd.Bool("rebase") || cfg.AbsorbStrategy == "direct" {
		ui.SimpleProgress("Performing autosquash rebase...")

		// Find the base commit (oldest absorbed commit's parent).
		var baseCommit string
		if len(absorbResp.Assignments) > 0 {
			// Use the oldest commit that received assignments.
			for _, commit := range commits {
				for _, assignment := range absorbResp.Assignments {
					if commit.SHA == assignment.CommitSHA {
						baseCommit = fmt.Sprintf("%s^", commit.SHA)
						break
					}
				}
				if baseCommit != "" {
					break
				}
			}
		}

		if baseCommit != "" {
			if err := repo.AutosquashRebase(ctx, baseCommit); err != nil {
				fmt.Printf("⚠️  Warning: Rebase failed: %v\n", err)
				fmt.Println("You can manually run: git rebase --autosquash -i " + baseCommit)
			} else {
				fmt.Println("✅ Successfully performed autosquash rebase")
			}
		}
	} else {
		fmt.Println("\n💡 To complete the absorb, run:")
		fmt.Println("   git rebase --autosquash -i <base-commit>")
	}

	fmt.Println("\n✨ Absorb completed successfully!")
	fmt.Printf("💾 To undo, run: cmt absorb --undo\n")

	return nil
}

// runAbsorbUndo undoes the last absorb operation.
func runAbsorbUndo(ctx context.Context) error {
	ui.SimpleProgress("Undoing last absorb operation...")

	repo, err := git.NewRepository("")
	if err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	if err := repo.UndoAbsorb(ctx); err != nil {
		return fmt.Errorf("failed to undo absorb: %w", err)
	}

	fmt.Println("✅ Successfully undone last absorb operation")
	return nil
}

// runListBackups lists all backup refs.
func runListBackups(ctx context.Context) error {
	repo, err := git.NewRepository("")
	if err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	// List backup refs.
	refs, err := repo.ListBackupRefs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list backup refs: %w", err)
	}

	if len(refs) == 0 {
		fmt.Println("No backup refs found.")
		return nil
	}

	fmt.Println("📚 Absorb backups:")
	for _, ref := range refs {
		// Extract timestamp from ref name
		parts := strings.Split(ref, "/")
		name := parts[len(parts)-1]

		// Parse timestamp if possible
		var timeStr string
		if strings.HasPrefix(name, "absorb-") {
			timestampStr := strings.TrimPrefix(name, "absorb-")
			if timestamp, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
				t := time.Unix(timestamp, 0)
				timeStr = fmt.Sprintf(" (%s)", t.Format("2006-01-02 15:04:05"))
			}
		}

		fmt.Printf("  • %s%s\n", name, timeStr)
	}

	return nil
}

// runCleanupBackups cleans up old backup refs.
func runCleanupBackups(ctx context.Context) error {
	repo, err := git.NewRepository("")
	if err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	// Check if there's an active absorb state.
	state, stateErr := git.LoadAbsorbState(repo)
	activeBackupRef := ""
	if stateErr == nil && state != nil {
		activeBackupRef = state.BackupRef
	}

	// List and clean up backup refs.
	refs, err := repo.ListBackupRefs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list backup refs: %w", err)
	}

	if len(refs) == 0 {
		fmt.Println("No backups to clean up.")
		return nil
	}

	deletedCount := 0
	for _, ref := range refs {
		// Don't delete the active backup.
		if ref == activeBackupRef {
			parts := strings.Split(ref, "/")
			name := parts[len(parts)-1]
			fmt.Printf("⏭️  Skipping active backup: %s\n", name)
			continue
		}

		// Delete the ref.
		if err := repo.DeleteBackupRef(ctx, ref); err != nil {
			fmt.Printf("⚠️  Failed to delete ref %s: %v\n", ref, err)
		} else {
			parts := strings.Split(ref, "/")
			name := parts[len(parts)-1]
			fmt.Printf("🗑️  Deleted backup: %s\n", name)
			deletedCount++
		}
	}

	if deletedCount == 0 {
		fmt.Println("No backups were deleted (all are active or failed to delete).")
	} else {
		fmt.Printf("\n✅ Cleaned up %d backup(s)\n", deletedCount)
	}

	return nil
}