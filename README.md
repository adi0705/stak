# Stack - PR Stacking Tool

A Go-based CLI tool that enables stacked PR workflows similar to Graphite. Stack manages branch dependencies, automates PR creation/updates, and handles merging in the correct order.

## Features

- Create stacked PRs with automatic base branch management
- Visualize branch dependencies as a tree
- Sync changes across entire stack with automatic rebasing
- Submit (merge) PRs in correct order with automatic base updates
- Modify PRs and propagate changes to children
- Store metadata in git config (no external dependencies)

## Installation

### Build from source

```bash
go build -o stack
sudo mv stack /usr/local/bin/
```

Or use it directly from the project directory:

```bash
go build -o stack
./stack --help
```

## Prerequisites

- Git
- GitHub CLI (`gh`) - Install with `brew install gh` or see [GitHub CLI docs](https://cli.github.com/)
- GitHub CLI must be authenticated: `gh auth login`

## Quick Start

1. Initialize your repository:
```bash
stack init
```

2. Create your first stacked PR:
```bash
# On main branch
git checkout -b feature-a
# Make commits
git add . && git commit -m "Add feature A"
stack create --title "Add feature A"
```

3. Stack another PR on top:
```bash
git checkout -b feature-b
# Make more commits
git add . && git commit -m "Add feature B"
stack create --title "Add feature B"
```

4. Visualize your stack:
```bash
stack list
# Output:
# main
# └─ feature-a (#1)
#    └─ feature-b (#2)
```

## Commands

### `stack init`

Initialize repository for stack. Verifies git setup and GitHub CLI authentication.

```bash
stack init
```

### `stack create`

Create a new branch stacked on top of the current branch and create a PR.

```bash
stack create [branch-name]
stack create --title "My PR title" --body "Description"
stack create --draft  # Create as draft PR
```

**Flags:**
- `--title, -t`: PR title (will prompt if not provided)
- `--body, -b`: PR description
- `--draft`: Create as draft PR

### `stack list`

Display a tree visualization of all stacked branches.

```bash
stack list
```

### `stack sync`

Sync the current branch and its children with remote changes. Rebases current branch onto its parent and recursively syncs all child branches.

```bash
stack sync
stack sync --current-only  # Skip syncing children
stack sync --continue      # Continue after resolving conflicts
```

**Flags:**
- `--recursive, -r`: Sync child branches recursively (default: true)
- `--current-only`: Only sync current branch, skip children
- `--continue`: Continue sync after resolving conflicts

### `stack modify`

Modify the current branch and sync all children.

```bash
stack modify                # Push changes and sync children
stack modify --amend        # Amend last commit
stack modify --rebase 3     # Interactive rebase last 3 commits
stack modify --edit --title "New title"  # Update PR details
stack modify --push-only    # Only push, skip syncing children
```

**Flags:**
- `--amend`: Amend the last commit
- `--rebase N`: Interactive rebase last N commits
- `--edit`: Edit PR title/body
- `--title`: New PR title
- `--body`: New PR body
- `--push-only`: Only push changes, skip syncing children

### `stack submit`

Submit and merge PRs in the correct order (bottom to top).

```bash
stack submit              # Submit current PR
stack submit --all        # Submit entire stack
stack submit --method merge  # Use merge instead of squash
stack submit --skip-checks   # Skip approval/CI checks
```

**Flags:**
- `--all`: Submit entire stack from current branch
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
stack create --title "Add authentication backend"
# PR #1 created: auth-backend → main

# Create second branch stacked on first
git checkout -b auth-frontend
# Make changes
git add . && git commit -m "Add authentication UI"
stack create --title "Add authentication UI"
# PR #2 created: auth-frontend → auth-backend

# Visualize
stack list
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
stack modify
# Pushes auth-backend and rebases auth-frontend
```

### Syncing with Remote

```bash
# After changes in main
git checkout auth-backend
stack sync
# Rebases auth-backend onto main and auth-frontend onto auth-backend
```

### Submitting a Stack

```bash
# When all PRs are approved
git checkout auth-frontend
stack submit --all
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
4. Continue: `stack sync --continue`

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

- Use `stack list` frequently to visualize your stack
- Always sync before making new changes: `stack sync`
- Use `--draft` flag when creating WIP PRs
- Use `stack modify --amend` for quick fixes
- Test your changes before submitting: `stack submit` (without `--all`)

## Troubleshooting

### "not in a git repository"
Run `git init` to initialize a git repository.

### "gh CLI not authenticated"
Run `gh auth login` to authenticate.

### "branch has no associated PR"
The branch was not created with `stack create`. You can manually add metadata with git config.

### Rebase conflicts
Resolve conflicts manually, then run `stack sync --continue`.

## License

MIT
