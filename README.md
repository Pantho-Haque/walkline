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

Both scripts auto-detect platform/architecture, download and checksum-verify the matching release binary, install it, clean up afterward, and run `walkline install` to set up the git hooks template. No leftover files, nothing written outside the chosen install directory.

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

This creates a global git hooks template at `~/.git-templates/hooks/` and configures git to use it. Every **NEW** repo created after this command will automatically get the walkline post-commit hook.

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
- **Push tracking** requires adding the git wrapper to your shell (see below)

## Git Push Wrapper

walkline uses a shell wrapper to track push status, since git has no reliable "push succeeded" hook.

After running `walkline install`, you'll see a wrapper script printed. Add it to your shell rc file (`.bashrc` or `.zshrc`).

The wrapper:
1. Intercepts `git push` commands
2. Detects the pushed commit range
3. Calls `walkline mark-pushed` to update the database

### Supported shells
- bash
- zsh

## CLI Commands

### `walkline log-commit`
Records the most recent commit to the database. Called automatically by the post-commit hook.

### `walkline mark-pushed <ref>`
Marks commits as pushed given a ref range (e.g., `origin/main..HEAD`).

### `walkline report [--since=<date>] [--project=<name>] [--pushed|--unpushed]`
Prints a table of commits with their push status.

`--since` accepts an RFC3339 date string (e.g., `2024-01-15T00:00:00Z` or `2024-01-15`).

### `walkline export --format=json|csv --out=<path>`
Exports commits to a file.

### `walkline install`
Sets up the global git template for future repos.

### `walkline scan <root-dir> [--depth=1]`
Scans existing repos and installs hooks.

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
└── shellwrap/            # Git push wrapper generation
install.sh                # Mac/Linux/WSL/Git-Bash installer
install.ps1               # Windows PowerShell installer
```

## License

MIT
