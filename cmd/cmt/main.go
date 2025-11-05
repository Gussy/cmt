package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gussy/cmt/internal/ai"
	"github.com/gussy/cmt/internal/config"
	"github.com/gussy/cmt/internal/git"
	"github.com/gussy/cmt/internal/preprocess"
	"github.com/gussy/cmt/internal/security"
	"github.com/gussy/cmt/internal/ui"
	"github.com/urfave/cli/v3"
)

var (
	// Version is set at build time.
	Version = "dev"
	// BuildTime is set at build time.
	BuildTime = "unknown"
)

func main() {
	app := &cli.Command{
		Name:                  "cmt",
		Usage:                 "Commit Message Tool - Generate contextual commit messages using Claude AI",
		Version:               fmt.Sprintf("%s (built %s)", Version, BuildTime),
		EnableShellCompletion: true,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "stage-all",
				Aliases: []string{"a"},
				Usage:   "Stage all changes before generating commit message",
			},
			&cli.BoolFlag{
				Name:    "yes",
				Aliases: []string{"y"},
				Usage:   "Skip confirmation and auto-commit",
			},
			&cli.BoolFlag{
				Name:    "oneline",
				Aliases: []string{"o"},
				Usage:   "Generate single-line commit message (50 chars max)",
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Generate verbose commit message with detailed explanation",
			},
			&cli.StringFlag{
				Name:    "hint",
				Aliases: []string{"h"},
				Usage:   "Additional context or requirements for the commit message",
			},
			&cli.StringFlag{
				Name:    "scope",
				Aliases: []string{"s"},
				Usage:   "Scope for conventional commits (e.g., auth, api, ui)",
			},
			&cli.BoolFlag{
				Name:    "push",
				Aliases: []string{"p"},
				Usage:   "Push to remote after committing",
			},
			&cli.StringFlag{
				Name:  "model",
				Usage: "Claude model to use (default: haiku-4.5)",
				Value: "haiku-4.5",
			},
			&cli.BoolFlag{
				Name:  "no-secret-scan",
				Usage: "Skip scanning for secrets in staged files",
			},
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Enable debug output",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "init",
				Usage: "Initialize cmt configuration in current repository",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return initConfig(ctx)
				},
			},
			{
				Name:  "config",
				Usage: "Manage configuration",
				Commands: []*cli.Command{
					{
						Name:  "get",
						Usage: "Get configuration value",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if cmd.Args().Len() < 1 {
								return fmt.Errorf("usage: cmt config get <key>")
							}
							return getConfig(ctx, cmd.Args().First())
						},
					},
					{
						Name:  "set",
						Usage: "Set configuration value",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							if cmd.Args().Len() < 2 {
								return fmt.Errorf("usage: cmt config set <key> <value>")
							}
							return setConfig(ctx, cmd.Args().Get(0), cmd.Args().Get(1))
						},
					},
				},
			},
			{
				Name:  "diff",
				Usage: "Show the diff that will be committed",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return showDiff(ctx)
				},
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return runCommit(ctx, cmd)
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

// runCommit is the main workflow for generating and creating a commit.
func runCommit(ctx context.Context, cmd *cli.Command) error {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Step 1: Initialize git repository
	repo, err := git.NewRepository("")
	if err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	// Step 2: Stage files if requested
	if cmd.Bool("stage-all") {
		ui.SimpleProgress(ui.ProgressMessages.StagingFiles)
		if err := repo.StageAll(ctx); err != nil {
			return fmt.Errorf("failed to stage files: %w", err)
		}
	}

	// Step 3: Check if there are staged changes
	hasChanges, err := repo.HasStagedChanges(ctx)
	if err != nil {
		return fmt.Errorf("failed to check staged changes: %w", err)
	}

	if !hasChanges {
		fmt.Println("❌ No staged changes to commit.")
		fmt.Println("\nUse 'git add' to stage files or use the -a flag to stage all changes.")
		return nil
	}

	// Step 4: Get diff and staged files
	ui.SimpleProgress(ui.ProgressMessages.AnalyzingChanges)
	diff, err := repo.GetDiff(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to get diff: %w", err)
	}

	stagedFiles, err := repo.GetStagedFiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to get staged files: %w", err)
	}

	// Step 5: Security scan (unless skipped via flag or config)
	skipScan := cmd.Bool("no-secret-scan") || cfg.SkipSecretScan
	if !skipScan {
		ui.SimpleProgress(ui.ProgressMessages.ScanningSecrets)
		scanner := security.NewScanner()
		secrets, err := scanner.Scan(diff)
		if err != nil {
			return fmt.Errorf("security scan failed: %w", err)
		}

		if len(secrets) > 0 {
			// Show interactive secret warning.
			action, err := ui.ShowSecretWarning(secrets)
			if err != nil {
				return fmt.Errorf("failed to show secret warning: %w", err)
			}

			switch action {
			case ui.ActionAbort:
				fmt.Println("\n❌ Commit aborted due to detected secrets.")
				return nil

			case ui.ActionUnstage:
				// Unstage files with secrets.
				uniqueFiles := make(map[string]bool)
				for _, secret := range secrets {
					uniqueFiles[secret.FilePath] = true
				}

				for file := range uniqueFiles {
					if err := repo.UnstageFiles(ctx, []string{file}); err != nil {
						fmt.Printf("Warning: Failed to unstage %s: %v\n", file, err)
					}
				}
				fmt.Printf("\n⚠️  Unstaged %d file(s) containing secrets.\n", len(uniqueFiles))
				fmt.Println("Please review and fix the issues before committing.")
				return nil

			case ui.ActionContinue:
				// User explicitly chose to continue despite warnings.
				fmt.Println("\n⚠️  Continuing with commit despite secret warnings.")
			}
		}
	}

	// Step 6: Initialize AI provider with config
	providerConfig := &ai.ProviderConfig{
		DefaultModel: cfg.Model,
		Timeout:      60, // Default timeout
	}
	provider, err := ai.NewClaudeCLI(providerConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize Claude CLI: %w", err)
	}

	// Check if Claude is available
	available, err := provider.IsAvailable(ctx)
	if !available || err != nil {
		return fmt.Errorf("Claude CLI is not available. Please ensure 'claude' is installed and in your PATH")
	}

	// Step 7: Preprocess diff for AI
	preprocessOpts := preprocess.Options{
		MaxTokens:       cfg.MaxDiffTokens,
		FilterBinary:    cfg.FilterBinary,
		FilterMinified:  cfg.FilterMinified,
		FilterGenerated: cfg.FilterGenerated,
	}

	// Use ProcessWithStats to get information about filtering
	processedDiff, stats := preprocess.ProcessWithStats(diff, preprocessOpts)

	// Log preprocessing stats if verbose
	if cfg.Verbose {
		if stats.FilteredFiles > 0 {
			fmt.Printf("📝 Preprocessed diff: %d/%d files included\n",
				stats.TotalFiles-stats.FilteredFiles, stats.TotalFiles)
			if stats.BinaryFiles > 0 {
				fmt.Printf("   - Filtered %d binary file(s)\n", stats.BinaryFiles)
			}
			if stats.MinifiedFiles > 0 {
				fmt.Printf("   - Filtered %d minified file(s)\n", stats.MinifiedFiles)
			}
			if stats.GeneratedFiles > 0 {
				fmt.Printf("   - Filtered %d generated/lock file(s)\n", stats.GeneratedFiles)
			}
		}
		if stats.Truncated {
			fmt.Printf("   - Truncated at %d tokens (limit: %d)\n",
				stats.TokensUsed, cfg.MaxDiffTokens)
		}
	}

	// Step 8: Build prompt and generate commit message
	ui.SimpleProgress(ui.ProgressMessages.GeneratingMessage)

	// Determine message format
	var msgFormat ai.MessageFormat
	if cmd.Bool("oneline") {
		msgFormat = ai.FormatOneLine
	} else if cmd.Bool("verbose") {
		msgFormat = ai.FormatVerbose
	} else {
		msgFormat = ai.FormatStandard
	}

	// Build the request with config values (command flags override config)
	model := cmd.String("model")
	if model == "" {
		model = cfg.Model
	}

	// Apply scope from config if always_scope is enabled and no scope provided
	scope := cmd.String("scope")
	if scope == "" && cfg.AlwaysScope {
		// Auto-detect scope from staged files if configured
		// This is a placeholder - could implement intelligent scope detection
		scope = ""
	}

	req := &ai.CommitRequest{
		Diff:        processedDiff, // Use preprocessed diff instead of raw diff
		StagedFiles: stagedFiles,
		Format:      msgFormat,
		Hint:        cmd.String("hint"),
		Scope:       scope,
		Model:       model,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
	}

	// Generate commit message with retry logic
	var response *ai.CommitResponse
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		response, err = provider.GenerateCommitMessage(ctx, req)
		if err == nil && response != nil && response.Message != "" {
			break // Success
		}

		if attempt < maxRetries {
			if err != nil {
				fmt.Fprintf(os.Stderr, "Attempt %d failed: %v. Retrying...\n", attempt, err)
			} else if response == nil || response.Message == "" {
				fmt.Fprintf(os.Stderr, "Attempt %d: Empty response received. Retrying...\n", attempt)
			}
			// Wait a bit before retrying
			time.Sleep(time.Second * 2)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to generate commit message after %d attempts: %w", maxRetries, err)
	}
	if response == nil || response.Message == "" {
		return fmt.Errorf("received empty commit message after %d attempts", maxRetries)
	}

	// Step 8: Interactive review (unless auto-commit or non-interactive mode in config)
	if !cmd.Bool("yes") && cfg.Interactive {
		// Use the interactive Bubble Tea UI for review
		for {
			action, feedback, err := ui.ShowCommitReview(response.Message, diff, cfg.EditorMode)
			if err != nil {
				return fmt.Errorf("failed to show review UI: %w", err)
			}

			switch action {
			case ui.ReviewAccept:
				// Continue to commit
				goto commit

			case ui.ReviewReject:
				fmt.Println("\n❌ Commit cancelled.")
				return nil

			case ui.ReviewRegenerate:
				// Regenerate with feedback
				ui.SimpleProgress(ui.ProgressMessages.Regenerating)
				response, err = provider.RegenerateWithFeedback(ctx, req, response.Message, feedback)
				if err != nil {
					return fmt.Errorf("failed to regenerate: %w", err)
				}
				// Loop back to show the new message
				continue

			case ui.ReviewEdit:
				// Open external editor for manual editing
				fmt.Println("\n💭 Opening your editor...")
				editedMessage, err := ui.EditInEditor(response.Message)
				if err != nil {
					fmt.Printf("Failed to edit message: %v\n", err)
					continue
				}
				response.Message = editedMessage
				fmt.Println("✓ Message updated")
				// Loop back to show the edited message for review
				continue

			case ui.ReviewEditInline:
				// Inline editing was done in the UI, update the message
				response.Message = feedback  // feedback contains the edited message
				// Loop back to show the edited message for review
				continue
			}
		}
	}

commit:

	// Step 9: Create the commit
	ui.SimpleProgress(ui.ProgressMessages.CreatingCommit)
	if err := repo.Commit(ctx, response.Message); err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}
	fmt.Println("\n✅ Commit created successfully!")

	// Step 10: Push if requested
	if cmd.Bool("push") {
		ui.SimpleProgress(ui.ProgressMessages.PushingChanges)
		if err := repo.Push(ctx); err != nil {
			return fmt.Errorf("failed to push: %w", err)
		}
		fmt.Println("✅ Pushed successfully!")
	}

	// Show final status
	fmt.Println("\n✨ Done! Your changes have been committed.")

	// Show the commit message one more time
	lastMsg, _ := repo.GetLastCommitMessage(ctx)
	if lastMsg != "" {
		fmt.Println("\nCommit message:")
		fmt.Println(lastMsg)
	}

	return nil
}

// initConfig initializes a .cmt.yml configuration file in the current repository.
func initConfig(ctx context.Context) error {
	// Create default config
	cfg := config.Default()

	// Save to local .cmt.yml
	if err := cfg.Save(false); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("✓ Created .cmt.yml with default configuration")
	return nil
}

// getConfig retrieves a configuration value.
func getConfig(ctx context.Context, key string) error {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get value by key
	value, err := cfg.Get(key)
	if err != nil {
		return err
	}

	fmt.Printf("%s: %v\n", key, value)
	return nil
}

// setConfig sets a configuration value.
func setConfig(ctx context.Context, key, value string) error {
	// Load existing configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Set the new value
	if err := cfg.Set(key, value); err != nil {
		return err
	}

	// Save to local config file
	if err := cfg.Save(false); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✓ Set %s = %s\n", key, value)
	return nil
}

// showDiff displays the diff that will be committed.
func showDiff(ctx context.Context) error {
	repo, err := git.NewRepository("")
	if err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	// Check for staged changes
	hasChanges, err := repo.HasStagedChanges(ctx)
	if err != nil {
		return fmt.Errorf("failed to check staged changes: %w", err)
	}

	if !hasChanges {
		fmt.Println("No staged changes. Showing unstaged diff:")
		diff, err := repo.GetDiff(ctx, false)
		if err != nil {
			return fmt.Errorf("failed to get diff: %w", err)
		}
		if diff == "" {
			fmt.Println("No changes to display.")
		} else {
			fmt.Println(diff)
		}
	} else {
		fmt.Println("Staged changes that will be committed:")
		diff, err := repo.GetDiff(ctx, true)
		if err != nil {
			return fmt.Errorf("failed to get diff: %w", err)
		}
		fmt.Println(diff)
	}

	return nil
}