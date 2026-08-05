# walkline

A standalone Go CLI that maintains a persistent local record of every git commit and its push status, across ALL git repos on your machine.

## Installation

### Quick Install (Primary Path)

**Mac/Linux/WSL/Git-Bash:**
```bash
curl -fsSL https://raw.githubusercontent.com/Pantho-Haque/walkline/main/install.sh | sh
```

**Windows (native PowerShell):**
```powershell
irm https://raw.githubusercontent.com/Pantho-Haque/walkline/main/install.ps1 | iex
```

Both scripts auto-detect platform/architecture, download and checksum-verify the matching release binary, install it, clean up afterward, and run `walkline install` to set up the git hooks template and shell completion. No leftover files, nothing written outside the chosen install directory.

For a specific version, set `WALKLINE_VERSION`:
```bash
WALKLINE_VERSION=v0.3.0 ./install.sh
```
```powershell
$env:WALKLINE_VERSION = "v0.3.0"; irm ... | iex
```

### Manual / From Source (Secondary Path)

For contributors or unsupported platforms:

```bash
git clone https://github.com/Pantho-Haque/walkline.git
cd walkline
go build ./...
# Binary ends up at ./walkline
```

## Setup Order

**Important:** Follow this exact order for complete coverage:

### Step 1: Install (one-time machine setup)

```bash
walkline install
```

This:
1. Creates a global git hooks template at `~/.git-templates/hooks/`
2. Installs the git push wrapper to your shell rc
3. Installs shell completion for tab-completion support

Every **NEW** repo created after this command will automatically get the walkline post-commit hook.

> **NOTE:** `walkline install` does NOT affect existing repos that were created before this command ran. That's what Step 2 is for.

### Step 2: Scan existing repos (retroactive coverage)

```bash
walkline scan <root-directory> [--depth=1]
```

Scans a directory for existing git repos and installs the hook into each one found. The `--depth=1` flag (default) means it checks immediate subdirectories only.

For each repo found:
- **No existing hook** → installs fresh
- **Existing custom hook** → merges walkline call at the end, preserving existing behavior
- **Already has walkline** → no-op (safe to re-run)

Run this once for each directory containing your existing projects.

### After Steps 1 + 2

From this point forward:
- **New repos** automatically get the hook via the template mechanism
- **Existing repos** are already instrumented from Step 2
- **Push tracking** works automatically via the shell wrapper

## CLI Commands

### `walkline install`
Sets up the global git template for future repos, installs the push wrapper, and sets up shell completion.

```
walkline install
```

### `walkline scan <root-dir> [--depth=1]`
Scans existing repos and installs hooks retroactively.

```
walkline scan ~/projects --depth=1
```

Flags:
- `--depth int` - Directory depth to scan (default: 1)

### `walkline log-commit`
Records the most recent commit to the database. Called automatically by the post-commit hook.

```
walkline log-commit
```

### `walkline mark-pushed [ref]`
Marks commits as pushed. Auto-detects the current branch if no argument provided.

```
walkline mark-pushed              # Auto-detect branch
walkline mark-pushed origin/main..HEAD  # Manual range
```

### `walkline report [--since=<date>] [--project=<name>] [--author=<name>] [--pushed] [--pending]`
Prints a structured report of commits with their push status.

```
walkline report
walkline report --project=myrepo
walkline report --author=john
walkline report --pushed
walkline report --pending
```

Flags:
- `--since string` - Since date (RFC3339, e.g., `2024-01-15` or `2024-01-15T00:00:00Z`)
- `--project string` - Filter by project name
- `--author string` - Filter by author name or email (partial match)
- `--pushed` - Only show pushed commits
- `--pending` - Only show pending (unpushed) commits

### `walkline export --format=json|csv --out=<path>`
Exports commits to a file with the same filtering options as report.

```
walkline export --format=json --out=commits.json
walkline export --format=csv --out=commits.csv --project=myrepo
```

Flags:
- `--format string` - Format: `json` or `csv` (default: `csv`)
- `--out string` - Output file path (required)
- `--since string` - Since date
- `--project string` - Project name
- `--author string` - Author name or email
- `--pushed` - Only pushed
- `--pending` - Only pending

### `walkline shellwrap [--install]`
Shows the git push wrapper and offers to install it.

```
walkline shellwrap              # Show wrapper code
walkline shellwrap --install     # Install to shell rc
```

### `walkline uninstall [--include-db]`
Remove walkline and all installed components from your system.

```
walkline uninstall              # Remove but keep database
walkline uninstall --include-db  # Remove everything including database
```

Removes:
- Binary from `~/.local/bin` or `/usr/local/bin`
- Git template at `~/.git-templates`
- Push wrapper from shell rc
- Shell completion files

### `walkline update`
Update walkline to the latest version from GitHub releases.

```
walkline update
```

### `walkline completion bash|zsh|fish|powershell`
Generate shell completion script.

```
walkline completion zsh > ~/.zsh/completions/_walkline
```

## Git Push Wrapper

walkline uses a shell wrapper to track push status, since git has no reliable "push succeeded" hook.

The wrapper is installed automatically by `walkline install`. It:
1. Intercepts `git push` commands
2. Detects the pushed commit range (auto-detects branch)
3. Calls `walkline mark-pushed` to update the database

### Supported shells
- bash
- zsh

## .git/hooks Is Never Tracked

**This is the core privacy guarantee:**

`.git/hooks/` is never tracked by git. It is not part of any commit and never travels via `clone`, `pull`, `push`, or any other repo transfer operation.

Everything walkline installs (via `install`'s templateDir mechanism or via `scan`'s retroactive merge) is purely local to your machine. If someone clones a repo you've instrumented with walkline, they get a completely clean `.git/hooks/` with only git's default `.sample` files — nothing of walkline ever reaches them.

## Known Limitations

### Amended/Rebased Commits
If you amend or rebase a commit, the hash changes. The old hash's row remains in the database as a stale orphan. No automatic cleanup is attempted.

### Single Machine Only
walkline is single-machine only. There is no cross-machine sync. Commits recorded on machine A are not visible on machine B.

### No Automatic Backfill
Commits made before walkline was installed in a repo simply aren't in the database. No automatic backfill is attempted — this is by design to avoid performance issues with large histories.

## Project Structure

```
cmd/walkline/main.go      # Entry point, cobra root command
internal/
├── cli/                  # Cobra command definitions
├── store/                # SQLite storage layer
├── hooks/                # Git hook template and repo scanning
└── shellwrap/           # Git push wrapper generation
install.sh                # Mac/Linux/WSL/Git-Bash installer
install.ps1               # Windows PowerShell installer
```

## License

MIT
