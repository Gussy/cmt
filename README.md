# cmt

`cmt` is a CLI tool that automatically generates meaningful commit messages by analyzing your staged changes using I. No API keys required, it uses the Claude Code CLI for authentication.

## Key Features

- **AI-Powered Messages** - Generates contextual commit messages using Claude AI (supports Haiku, Sonnet, and Opus models)
- **Interactive Review UI** - Built-in TUI for reviewing, regenerating, or editing messages before committing
- **Secret Detection** - Scans staged files for 15+ secret patterns (AWS keys, GitHub tokens, JWTs, private keys)
- **Smart Diff Processing** - Filters binary files, minified code, and generated files; truncates large diffs intelligently
- **Flexible Configuration** - Local `.cmt.yml`, global config, and environment variable overrides
- **No API Keys** - Uses Claude Code CLI for authentication (no separate API setup required)

## Prerequisites

- [Claude Code CLI](https://claude.ai/code) installed and available in your PATH
- Git repository

## Installation

Download the latest release for your platform from [GitHub Releases](https://github.com/gussy/cmt/releases):

```bash
# Download and extract (replace with actual release version and platform)
tar -xzf cmt_darwin_arm64.tar.gz

# Move to your PATH
mv cmt /usr/local/bin/

# Verify installation
cmt --help
```

## Configuration

Create a `.cmt.yml` in your project root or `~/.config/cmt/config.yml` globally:

```yaml
model: haiku-4.5        # Options: haiku-4.5, sonnet-4.5, opus-4.1
format: conventional     # Options: conventional, gitmoji, semantic
verbose: false
skip_secret_scan: false
```

See [config.example.yml](config.example.yml) for all available options.

### Environment Variables

Override any configuration option with `GAC_*` prefix:

```bash
export GAC_MODEL=sonnet-4.5
export GAC_VERBOSE=true
```

## How It Works

1. Stage your changes with `git add` or use `cmt --stage-all`
2. Run `cmt` to analyze your staged changes
3. Claude AI generates a contextual commit message based on your diff
4. Review, edit, or regenerate the message in the interactive UI
5. Accept to commit, optionally push with `--push`

### Common Usage

```bash
# Basic usage (commit staged changes)
cmt

# Stage all changes and commit
cmt --stage-all

# Auto-accept generated message
cmt --yes

# Generate and push in one command
cmt --stage-all --push

# Preview changes without committing
cmt diff

# Use a different model
cmt --model sonnet-4.5

# Initialize config file
cmt init
```
