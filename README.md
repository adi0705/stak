# Stak - PR Stacking Tool

A Go-based CLI tool that enables stacked PR workflows similar to Graphite. Stack manages branch dependencies, automates PR creation/updates, and handles merging in the correct order.

## Features

- Create stacked PRs with automatic base branch management
- Visualize branch dependencies as a tree
- Sync changes across entire stak with automatic rebasing
- Submit (merge) PRs in correct order with automatic base updates
- Modify PRs and propagate changes to children
- Store metadata in git config (no external dependencies)

## Installation

### Build from source

```bash
go build -o stak
sudo mv stak /usr/local/bin/
```

Or use it directly from the project directory:

```bash
go build -o stak
./stak --help
```

## Prerequisites

- Git
- GitHub CLI (`gh`) - Install with `brew install gh` or see [GitHub CLI docs](https://cli.github.com/)
- GitHub CLI must be authenticated: `gh auth login`

## Quick Start

1. Initialize your repository:
```bash
stak init
```

2. Create your first stacked PR:
```bash
# On main branch
git checkout -b feature-a
# Make commits
git add . && git commit -m "Add feature A"
stak create --title "Add feature A"
```

3. Stack another PR on top:
```bash
git checkout -b feature-b
# Make more commits
git add . && git commit -m "Add feature B"
stak create --title "Add feature B"
```

4. Visualize your stack:
```bash
stak list
# Output:
# main
# └─ feature-a (#1)
#    └─ feature-b (#2)
```

## Commands

### `stak init`

Initialize repository for stack. Verifies git setup and GitHub CLI authentication.

```bash
stak init
```

### `stak create`

Create a new branch stacked on top of the current branch and create a PR.

```bash
stak create [branch-name]
stak create --title "My PR title" --body "Description"
stak create --draft  # Create as draft PR
```

**Flags:**
- `--title, -t`: PR title (will prompt if not provided)
- `--body, -b`: PR description
- `--draft`: Create as draft PR

### `stak list`

Display a tree visualization of all stacked branches.

```bash
stak list
```

### `stak sync`

Sync the current branch and its children with remote changes. Rebases current branch onto its parent and recursively syncs all child branches.

**Automatic Cleanup:** If a branch's PR has been merged on GitHub, `stak sync` will automatically:
- Delete the local branch
- Remove the metadata
- Update child branches to point to the new parent
- Update child PR bases on GitHub

```bash
stak sync
stak sync --current-only  # Skip syncing children
stak sync --continue      # Continue after resolving conflicts
```

**Flags:**
- `--recursive, -r`: Sync child branches recursively (default: true)
- `--current-only`: Only sync current branch, skip children
- `--continue`: Continue sync after resolving conflicts

### `stak modify`

Modify the current branch and sync all children.

```bash
stak modify                # Push changes and sync children
stak modify --amend        # Amend last commit
stak modify --rebase 3     # Interactive rebase last 3 commits
stak modify --edit --title "New title"  # Update PR details
stak modify --push-only    # Only push, skip syncing children
```

**Flags:**
- `--amend`: Amend the last commit
- `--rebase N`: Interactive rebase last N commits
- `--edit`: Edit PR title/body
- `--title`: New PR title
- `--body`: New PR body
- `--push-only`: Only push changes, skip syncing children

### `stak submit`

Submit and merge PRs in the correct order (bottom to top).

```bash
stak submit              # Submit current PR
stak submit --all        # Submit entire stack
stak submit --method merge  # Use merge instead of squash
stak submit --skip-checks   # Skip approval/CI checks
```

**Flags:**
- `--all`: Submit entire stak from current branch
- `--method`: Merge method: squash (default), merge, or rebase
- `--skip-checks`: Skip approval and CI checks

## Workflow Example

### Creating a Stack

```bash
# Start on main
git checkout main

# Create first branch
git checkout -b auth-backend
# Make changes
git add . && git commit -m "Add authentication backend"
stak create --title "Add authentication backend"
# PR #1 created: auth-backend → main

# Create second branch stacked on first
git checkout -b auth-frontend
# Make changes
git add . && git commit -m "Add authentication UI"
stak create --title "Add authentication UI"
# PR #2 created: auth-frontend → auth-backend

# Visualize
stak list
# main
# └─ auth-backend (#1)
#    └─ auth-frontend (#2)
```

### Modifying a Stack

```bash
# Make changes to auth-backend
git checkout auth-backend
# Edit files
git commit --amend --no-edit
stak modify
# Pushes auth-backend and rebases auth-frontend
```

### Syncing with Remote

```bash
# After changes in main
git checkout auth-backend
stak sync
# Rebases auth-backend onto main and auth-frontend onto auth-backend
```

### Automatic Cleanup After Merge

```bash
# After manually merging PR #1 on GitHub (auth-backend → main)
git checkout auth-frontend
stak sync
# ℹ Fetching from remote
# ℹ PR #1 for branch auth-backend is merged, cleaning up
# ℹ Updating auth-frontend parent: auth-backend → main
# ℹ Updated PR #2 base to main
# ℹ Switching to main
# ℹ Deleting local branch auth-backend
# ✓ Deleted branch auth-backend
# ℹ Syncing branch auth-frontend
# ℹ Rebasing auth-frontend onto origin/main
# ✓ Synced auth-frontend
```

### Submitting a Stack

```bash
# When all PRs are approved
git checkout auth-frontend
stak submit --all
# Merges PR #1 into main
# Updates PR #2 base to main
# Rebases auth-frontend onto main
# Merges PR #2 into main
# Cleans up local branches
```

## How It Works

### Metadata Storage

Branch relationships are stored in git config:

```ini
[stack "branch.feature-a"]
    parent = main
    pr-number = 123

[stack "branch.feature-b"]
    parent = feature-a
    pr-number = 124
```

### Branch Relationships

- Each branch tracks its parent, forming a tree
- PRs target parent branches (not always main)
- When a parent is merged, children are rebased onto the new base

### Syncing Algorithm

1. Fetch from remote
2. Rebase current branch onto parent
3. Force push with `--force-with-lease`
4. Recursively rebase and push all children

### Submit Algorithm

1. Build ancestor chain from current to base
2. For each branch (bottom to top):
   - Check PR approval and CI status
   - Merge PR
   - Update children's parent to new base
   - Rebase children onto new base
   - Clean up merged branch

## Conflict Resolution

If a rebase conflict occurs:

1. Stack pauses and shows conflicted files
2. Resolve conflicts manually
3. Stage resolved files: `git add <file>`
4. Continue: `stak sync --continue`

Or abort: `git rebase --abort`

## Project Structure

```
stacking/
├── main.go                 # Entry point
├── cmd/                    # Command implementations
│   ├── root.go            # Root command
│   ├── create.go          # Create command
│   ├── list.go            # List command
│   ├── sync.go            # Sync command
│   ├── modify.go          # Modify command
│   ├── submit.go          # Submit command
│   └── init.go            # Init command
├── internal/
│   ├── git/               # Git operations
│   │   ├── config.go      # Git config operations
│   │   ├── branch.go      # Branch operations
│   │   └── rebase.go      # Rebase operations
│   ├── github/            # GitHub CLI wrapper
│   │   └── pr.go          # PR operations
│   ├── stack/             # Stack management
│   │   ├── metadata.go    # Metadata operations
│   │   └── tree.go        # Tree traversal
│   └── ui/                # User interface
│       └── display.go     # Display utilities
└── pkg/models/
    └── branch.go          # Branch model
```

## Tips

- Use `stak list` frequently to visualize your stack
- Run `stak sync` after merging PRs on GitHub to automatically clean up merged branches
- Always sync before making new changes: `stak sync`
- Use `--draft` flag when creating WIP PRs
- Use `stak modify --amend` for quick fixes
- Test your changes before submitting: `stak submit` (without `--all`)

## Troubleshooting

### "not in a git repository"
Run `git init` to initialize a git repository.

### "gh CLI not authenticated"
Run `gh auth login` to authenticate.

### "branch has no associated PR"
The branch was not created with `stak create`. You can manually add metadata with git config.

### Rebase conflicts
Resolve conflicts manually, then run `stak sync --continue`.

## License

MIT

## Credits
Claude Code
