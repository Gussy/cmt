# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**cmt** is a Go rewrite of the Python-based Git Auto Commit tool that generates contextual commit messages using Claude AI through the Claude Code CLI (no API keys required). The project uses Bubble Tea for rich interactive TUI experiences.

**Status**: All 8 implementation phases completed (as of 2025-10-29)
**Go Version**: 1.25.3 (via Hermit)

## Build Commands

```bash
# Build with version info
task build

# Run with CLI arguments
task run -- --help
task run -- --stage-all --push

# Install to $GOPATH/bin
task install

# Testing
task test                   # Run all tests with race detection
task test:coverage          # Generate HTML coverage report
go test -v ./internal/config  # Run single package tests

# Development workflow
task check                  # Run all pre-commit checks (fmt, vet, test, build)
task dev                    # Run with auto-reload using air

# Release management
task release:dry            # Test release process
task release                # Create GitHub release (requires tag)
```

## Architecture

### Package Structure

**cmd/cmt/main.go** - CLI orchestration
- Main workflow: stage → check changes → scan secrets → generate message → review → commit → push
- 3 subcommands: `init` (initialize config), `config get/set`, `diff` (preview changes)
- Retry logic (3 attempts) for message generation

**internal/ai/** - AI provider abstraction
- `provider.go`: Provider interface defining CommitRequest/Response contract
- `claudecli.go`: Claude Code CLI wrapper with model mapping (haiku-4.5, sonnet-4.5, opus-4.1)

**internal/git/** - Repository operations
- All operations use context with timeout support
- Staging, diff generation, commit creation, push to remote
- Branch detection and git hook awareness

**internal/config/** - Configuration management
- Precedence: Environment vars (GAC_*) > Local .cmt.yml > Global ~/.config/cmt/config.yml > Defaults
- 14 configurable options including model, temperature, filtering options
- Get/Set methods with Save functionality
- Follows XDG Base Directory specification for global config

**internal/security/** - Secret detection
- 15+ regex patterns (AWS keys, GitHub tokens, JWT, private keys, etc.)
- Two-level false positive filtering (placeholder detection + repetition analysis)
- Integration with interactive UI for remediation

**internal/preprocess/** - Diff optimization
- Binary file filtering (30+ extensions)
- Minified/generated file exclusion
- Token-aware truncation (default 16384 tokens, ~4 chars/token)

**internal/ui/** - Bubble Tea interactive UI
- `review.go`: Commit message review with viewport, accept/reject/regenerate/edit options
  - Edit mode respects `editor_mode` config: inline (textarea) or external (system editor)
- `secrets.go`: Security warning display with abort/unstage/continue actions
- `editor.go`: System editor integration for external editing
- `progress.go`: Inline progress indicators

**internal/prompt/** - Prompt building
- Template system supporting conventional, gitmoji, semantic formats
- Builder pattern for flexible prompt construction

## Configuration

### Environment Variables (GAC_* prefix)
```bash
export GAC_MODEL=haiku-4.5          # AI model (haiku-4.5, sonnet-4.5, opus-4.1)
export GAC_VERBOSE=true             # Detailed logging
export GAC_SKIP_SECRET_SCAN=false   # Security scanning
export GAC_MAX_DIFF_TOKENS=16384    # Truncation limit
export GAC_FILTER_BINARY=true       # Filter binary files
export GAC_EDITOR_MODE=inline       # Editor mode: "inline" or "external"
```

### Configuration Files
- Local: `.cmt.yml` (project-specific)
- Global: `~/.config/cmt/config.yml` (user-wide, XDG Base Directory standard)
- Example: `config.example.yml` (comprehensive template with documentation)

## Testing

```bash
# Run specific test file
go test -v ./internal/config/config_test.go

# Run with race detection and coverage
go test -v -race -coverprofile=coverage.txt ./...

# Generate HTML coverage report
task test:coverage
open coverage.html
```

Test patterns:
- Table-driven tests for comprehensive coverage
- Temp directories for file system operations
- Environment variable isolation with cleanup

## Development Standards

From user's CLAUDE.md preferences:
- Use urfave/cli/v3 for CLI framework (✓ implemented)
- All Go comments must end with periods (godot style)
- Always include newline at end of code files
- Use hermit for installing tooling
- Use `git mv` and `git rm` for tracked files
- Never modify git submodules
- Never commit to git (user preference)

## Workflow Integration Points

### Main Commit Flow (runCommit function)
1. Load configuration (merge files + env vars)
2. Initialize git repository
3. Optionally stage all changes (`--stage-all`)
4. Check for staged changes
5. Security scan (unless `--skip-secret-scan`)
6. Preprocess diff (filter + truncate)
7. Generate message via Claude CLI
8. Interactive review (Bubble Tea UI)
9. Create commit
10. Push if requested (`--push`)

### CLI Flags
- `--stage-all, -a`: Stage all changes before commit
- `--yes, -y`: Auto-accept generated message
- `--oneline`: Generate single-line message
- `--verbose, -v`: Show detailed processing info
- `--hint`: Add context hint for generation
- `--scope`: Add scope to commit message
- `--push, -p`: Push after commit
- `--model, -m`: Override AI model
- `--skip-secret-scan`: Disable security scanning
- `--non-interactive`: Disable Bubble Tea UI

### Editor Configuration
The edit action ('e' key) behavior depends on the `editor_mode` setting:
- `inline` (default): Opens a textarea within the TUI for quick edits
- `external`: Launches your system editor ($EDITOR or vim/nano fallback)

## Key Implementation Details

### Claude CLI Integration
- Automatic executable discovery via PATH
- Model name mapping to Claude CLI format
- Response cleaning (removes code blocks, quotes)
- Message splitting for multi-line commits

### Secret Scanning
- Runs before commit generation
- Interactive remediation through Bubble Tea UI
- Can unstage files with secrets or abort operation
- False positive filtering for test/demo data

### Diff Processing
- Token estimation for context limits
- Smart filtering of non-essential files
- Statistics tracking in verbose mode
- Configurable via environment/config

## Progress Tracking

Implementation progress and session notes are maintained in:
- `.notes/progress.md` - Detailed session-by-session progress
- `.notes/IMPLEMENTATION_PLAN.md` - Original 4-week plan

## Common Development Tasks

### Adding New Secret Pattern
1. Add regex to `internal/security/scanner.go` patterns slice
2. Update pattern count in documentation
3. Add test case to verify detection

### Modifying Prompt Templates
1. Edit `internal/prompt/templates.go`
2. Update Builder methods if new fields needed
3. Test with different formats (oneline, verbose, standard)

### Adding Configuration Option
1. Add to defaults in `internal/config/config.go`
2. Update LoadConfig to handle new key
3. Add environment variable mapping (GAC_* prefix)
4. Update config subcommand completion
5. Add test coverage in config_test.go
6. 