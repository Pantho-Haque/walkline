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
1. Creates a global git hooks template at `~/.git-templates/hooks/` (post-commit and pre-push)
2. Installs shell completion for tab-completion support

Every **NEW** repo created after this command will automatically get walkline's post-commit and pre-push hooks.

> **NOTE:** `walkline install` does NOT affect existing repos that were created before this command ran. That's what Step 2 is for.

### Step 2: Scan existing repos (retroactive coverage)

```bash
walkline scan <root-directory> [--depth=1]
```

Scans a directory for existing git repos and installs both post-commit and pre-push hooks into each one found. The `--depth=1` flag (default) means it checks immediate subdirectories only.

For each repo found:
- **No existing hook** → installs fresh
- **Existing custom hook** → merges walkline call at the end, preserving existing behavior
- **Already has walkline** → no-op (safe to re-run)

Run this once for each directory containing your existing projects.

### After Steps 1 + 2

From this point forward:
- **New repos** automatically get both hooks via the template mechanism
- **Existing repos** are already instrumented from Step 2
- **Push tracking** works automatically via the pre-push hook

## CLI Commands

### `walkline install`
Sets up the global git template for future repos (post-commit and pre-push hooks) and sets up shell completion.

```
walkline install
```

### `walkline scan <root-dir> [--depth=1]`
Scans existing repos and installs both post-commit and pre-push hooks retroactively.

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
walkline mark-pushed --range abc123..def456 --remote-ref refs/heads/main  # From pre-push hook
```

Flags:
- `--range string` - Git rev-list range (e.g. `abc123..def456`), bypasses branch auto-detection
- `--remote-ref string` - Remote ref (informational, for logging)

### `walkline sync [--project=<name>]`
Reconcile pending commits against their upstream. For each pending commit, checks if it's an ancestor of `@{upstream}` and flips it to pushed if so.

```
walkline sync                    # Sync all projects
walkline sync --project=myrepo   # Sync one project
```

This is run automatically at the start of `walkline report` and `walkline export`, but can also be run manually.

Flags:
- `--project string` - Sync only this project

### `walkline report [--since=<date>] [--until=<date>] [--at=<date>] [--project=<name>] [--author=<name>] [--pushed] [--pending]`
Prints a structured report of commits with their push status. Automatically runs a reconciliation pass before reporting.

```
walkline report
walkline report --project=myrepo
walkline report --author=john
walkline report --pushed
walkline report --pending
walkline report --since=2024-01-15
walkline report --until=2024-12-31
walkline report --at=2024-06-15
```

Date filter flags (mutually exclusive — use only one):
- `--since string` - Commits from this date onward (inclusive). Bare date `2024-01-15` means from midnight that day; full RFC3339 timestamp for precision.
- `--until string` - Commits up to this date (inclusive). Bare date `2024-12-31` means through end of day (23:59:59); the given date itself IS included.
- `--at string` - Commits on that exact calendar day only (00:00:00 through 23:59:59 inclusive).

Other flags:
- `--project string` - Filter by project name
- `--author string` - Filter by author name or email (partial match)
- `--pushed` - Only show pushed commits
- `--pending` - Only show pending (unpushed) commits
- `-n`, `--limit int` - Limit number of results

### `walkline export --format=json|csv --out=<path>`
Exports commits to a file with the same filtering options as report. Automatically runs a reconciliation pass before exporting.

```
walkline export --format=json --out=commits.json
walkline export --format=csv --out=commits.csv --project=myrepo
walkline export --format=csv --out=commits.csv --since=2024-01-01 --until=2024-06-30
```

Flags:
- `--format string` - Format: `json` or `csv` (default: `csv`)
- `--out string` - Output file path (required)
- `--since string` - Commits from this date onward (inclusive)
- `--until string` - Commits up to this date (inclusive)
- `--at string` - Commits on this exact date only
- `--project string` - Project name
- `--author string` - Author name or email
- `--pushed` - Only pushed
- `--pending` - Only pending

### `walkline uninstall [--include-db]`
Remove walkline and all installed components from your system.

```
walkline uninstall              # Remove but keep database
walkline uninstall --include-db  # Remove everything including database
```

Removes:
- Binary from `~/.local/bin` or `/usr/local/bin`
- Git templates at `~/.git-templates` (both post-commit and pre-push hooks)
- Shell completion files

Note: Hooks already installed in individual repos are left in place (they are harmless).

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

## Git Push Tracking

walkline uses a git-native pre-push hook to track push status.

### Pre-push hook (primary, reliable)
A git-native pre-push hook that fires for every `git push`, regardless of how the push is initiated (shell, IDE, CI, etc.). This is the trigger installed by `walkline install` and `walkline scan`.

The hook:
1. Reads stdin lines in git's pre-push format
2. Skips ref deletions and no-op pushes (same SHA)
3. Calls `walkline mark-pushed --range <old-sha>..<new-sha> --remote-ref <ref>` for each updated ref
4. Fails open: any internal error logs to stderr and exits 0 so pushes are never blocked

### Reconciliation (`walkline sync`)
A self-healing safety net that reconciles pending commits against their upstream. Run automatically at the start of `walkline report` and `walkline export`, or manually via `walkline sync`.

For each pending commit, it checks `git merge-base --is-ancestor <commit> @{upstream}` and flips it to pushed if the commit has been pushed (even if the hook didn't fire).

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

### Pre-push Hook and Non-standard Git Setups
The pre-push hook relies on git's standard stdin protocol. Highly unusual git configurations that bypass the standard push flow (e.g., custom git wrappers, some CI systems) may not trigger the hook. In these cases, `walkline sync` provides a self-healing safety net.

## Project Structure

```
cmd/walkline/main.go      # Entry point, cobra root command
internal/
├── cli/                  # Cobra command definitions
│   ├── commands.go       # install, scan, uninstall, update
│   ├── commit.go         # log-commit, mark-pushed
│   ├── report.go         # report, export
│   └── sync.go           # sync (reconciliation)
├── store/                # SQLite storage layer
└── hooks/                # Git hook template and repo scanning
install.sh                # Mac/Linux/WSL/Git-Bash installer
install.ps1               # Windows PowerShell installer
```