# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**cmt** generates contextual commit messages using Claude AI through Claude Code CLI (no API keys required). Go rewrite with Bubble Tea TUI. Status: Complete (8 phases). Go 1.25.3 via Hermit.

## Essential Commands

```bash
# Build & run
task build              # Build with version info
task run -- [flags]     # Run with CLI arguments
task install            # Install to $GOPATH/bin

# Testing & dev
task test               # Run all tests with race detection
task test:coverage      # Generate HTML coverage report
task check              # Run all pre-commit checks (fmt, vet, test, build)

# Release
task release:dry        # Test release process
task release            # Create GitHub release (requires tag)
```

## Architecture

**cmd/cmt/main.go** - Main workflow: stage → check → scan secrets → generate → review → commit → push. Subcommands: `init`, `config get/set`, `diff`. Retry logic: 3 attempts.

**internal/ai/** - Provider abstraction. Claude CLI wrapper with model mapping (haiku-4.5, sonnet-4.5, opus-4.1). Response cleaning and message splitting.

**internal/git/** - Repository operations with context/timeout. Staging, diff, commit, push, branch detection, git hook awareness.

**internal/config/** - Precedence: Env vars (GAC_*) > .cmt.yml > ~/.config/cmt/config.yml > Defaults. 14 options. XDG Base Directory compliant.

**internal/security/** - 15+ secret patterns. Two-level false positive filtering. Interactive remediation UI.

**internal/preprocess/** - Binary file filtering (30+ extensions). Minified/generated exclusion. Token truncation (default 16384, ~4 chars/token).

**internal/ui/** - Bubble Tea components: review (accept/reject/regenerate/edit), secrets (abort/unstage/continue), editor (inline textarea or external $EDITOR), progress indicators.

**internal/prompt/** - Template system (conventional, gitmoji, semantic). Builder pattern for flexible construction.

## Configuration

**Environment Variables (GAC_* prefix)**:
```bash
GAC_MODEL=haiku-4.5              # AI model
GAC_VERBOSE=true                 # Detailed logging
GAC_SKIP_SECRET_SCAN=false       # Security scanning
GAC_MAX_DIFF_TOKENS=16384        # Truncation limit
GAC_FILTER_BINARY=true           # Filter binary files
GAC_EDITOR_MODE=inline           # inline or external
```

**Files**: `.cmt.yml` (local), `~/.config/cmt/config.yml` (global), `config.example.yml` (template)

## Development Standards

- Use urfave/cli/v3 for CLI framework
- All Go comments must end with periods (godot style)
- Always include newline at end of code files
- Use hermit for installing tooling
- Use `git mv` and `git rm` for tracked files
- Never modify git submodules
- Never commit to git (user preference)

## Main Workflow

1. Load config (files + env vars)
2. Initialize git repository
3. Stage changes (optional: `--stage-all`)
4. Check for staged changes
5. Security scan (skip with `--skip-secret-scan`)
6. Preprocess diff (filter + truncate)
7. Generate message via Claude CLI (3 retries)
8. Interactive review (Bubble Tea UI)
9. Create commit
10. Push (optional: `--push`)

**Key flags**: `-a/--stage-all`, `-y/--yes`, `--oneline`, `-v/--verbose`, `--hint`, `--scope`, `-p/--push`, `-m/--model`, `--skip-secret-scan`, `--non-interactive`
