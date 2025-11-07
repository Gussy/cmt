# cmt

`cmt` is a CLI tool that automatically generates meaningful commit messages by analyzing your staged changes using I. No API keys required, it uses the Claude Code CLI for authentication.

## Key Features

- **AI-Powered Messages** - Generates contextual commit messages using Claude AI (supports Haiku, Sonnet, and Opus models)
- **AI-Driven Absorb** - Intelligently assigns staged hunks to previous commits using semantic analysis (like git-absorb but smarter)
- **Interactive Review UI** - Built-in TUI for reviewing, regenerating, or editing messages before committing
- **Secret Detection** - Scans staged files for 15+ secret patterns (AWS keys, GitHub tokens, JWTs, private keys)
- **Smart Diff Processing** - Filters binary files, minified code, and generated files; truncates large diffs intelligently
- **Flexible Configuration** - Local `.cmt.yml`, global config, and environment variable overrides
- **No API Keys** - Uses Claude Code CLI for authentication (no separate API setup required)

## Prerequisites

- [Claude Code CLI](https://claude.ai/code) installed and available in your PATH
- Git repository

## Installation

### Homebrew (macOS/Linux)

```bash
brew install --cask gussy/tap/cmt
```

### Debian/Ubuntu (.deb)

Download the `.deb` package from [GitHub Releases](https://github.com/gussy/cmt/releases) and install:

```bash
# Download the latest .deb package
wget https://github.com/gussy/cmt/releases/download/v0.x.x/cmt_0.x.x_amd64.deb

# Install with dpkg
sudo dpkg -i cmt_0.x.x_amd64.deb

# Or install with apt (resolves dependencies automatically)
sudo apt install ./cmt_0.x.x_amd64.deb
```

Shell completions are automatically installed to standard locations.

### RHEL/Fedora/CentOS (.rpm)

Download the `.rpm` package from [GitHub Releases](https://github.com/gussy/cmt/releases) and install:

```bash
# Download the latest .rpm package
wget https://github.com/gussy/cmt/releases/download/v0.x.x/cmt_0.x.x_x86_64.rpm

# Install with rpm
sudo rpm -i cmt_0.x.x_x86_64.rpm

# Or install with dnf/yum (resolves dependencies automatically)
sudo dnf install cmt_0.x.x_x86_64.rpm
```

Shell completions are automatically installed to standard locations.

### Alpine Linux (.apk)

Download the `.apk` package from [GitHub Releases](https://github.com/gussy/cmt/releases) and install:

```bash
# Download the latest .apk package
wget https://github.com/gussy/cmt/releases/download/v0.x.x/cmt_0.x.x_x86_64.apk

# Install with apk (use --allow-untrusted for unsigned packages)
sudo apk add --allow-untrusted cmt_0.x.x_x86_64.apk
```

Shell completions are automatically installed to standard locations.

### Arch Linux

Download the `.pkg.tar.zst` package from [GitHub Releases](https://github.com/gussy/cmt/releases) and install:

```bash
# Download the latest Arch package
wget https://github.com/gussy/cmt/releases/download/v0.x.x/cmt_0.x.x_x86_64.pkg.tar.zst

# Install with pacman
sudo pacman -U cmt_0.x.x_x86_64.pkg.tar.zst
```

Shell completions are automatically installed to standard locations.

### Manual Installation

Download the latest release for your platform from [GitHub Releases](https://github.com/gussy/cmt/releases):

```bash
# Download and extract (replace with actual release version and platform)
tar -xzf cmt_darwin_arm64.tar.gz

# Move to your PATH
mv cmt /usr/local/bin/

# Verify installation
cmt --help
```

### Shell Completions

Shell completions are automatically installed with Homebrew. For manual installations, generate completion scripts for your shell:

**Bash:**
```bash
cmt completion bash > ~/.local/share/bash-completion/completions/cmt
```

**Zsh:**
```bash
cmt completion zsh > "${fpath[1]}/_cmt"
```

**Fish:**
```bash
cmt completion fish > ~/.config/fish/completions/cmt.fish
```

After installing completions, restart your shell or source your shell configuration file.

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

Override any configuration option with `CMT_*` prefix:

```bash
export CMT_MODEL=sonnet-4.5
export CMT_VERBOSE=true
export CMT_ABSORB_STRATEGY=direct
export CMT_ABSORB_CONFIDENCE=0.8
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

## AI-Driven Absorb Feature

The `cmt absorb` command uses AI to intelligently assign staged changes to previous commits based on semantic similarity. This is similar to `git-absorb` but with AI-powered understanding of code context and meaning.

### How It Works

1. Analyzes your staged changes and splits them into individual hunks
2. Uses AI to match each hunk with the most semantically related previous commit
3. Creates fixup commits that can be autosquashed into the target commits
4. Provides an interactive UI to review and modify assignments
5. Optionally performs an autosquash rebase automatically

### Absorb Usage

```bash
# Basic absorb (analyzes unpushed commits)
cmt absorb

# Skip interactive review
cmt absorb --yes

# Analyze specific number of commits
cmt absorb --depth 5

# Analyze all commits back to branch point
cmt absorb --to-branch-point

# Dry run to preview without changes
cmt absorb --dry-run

# Automatically rebase after creating fixup commits
cmt absorb --rebase

# Undo the last absorb operation
cmt absorb --undo
```

### Absorb Configuration

Add to your `.cmt.yml`:

```yaml
# Absorb-specific settings
absorb_strategy: fixup        # fixup (default) or direct
absorb_range: unpushed        # unpushed (default) or branch-point
absorb_ambiguity: interactive # interactive (default) or best-match
absorb_auto_commit: true      # Create new commit for unmatched hunks
absorb_confidence: 0.7        # Min confidence threshold (0.0-1.0)
```

### Absorb Workflow Example

```bash
# Make various fixes across multiple files
vim src/auth.js    # Fix auth bug
vim src/api.js     # Update API endpoint
vim tests/auth.js  # Fix related test

# Stage all changes
git add -A

# Use AI to absorb changes into relevant commits
cmt absorb

# Review assignments in the interactive UI
# - Navigate with arrow keys or tab
# - Press 'a' to see alternative assignments
# - Press 'u' to unassign a hunk
# - Press 'y' to accept assignments

# Complete with autosquash rebase
git rebase --autosquash -i HEAD~3
```
