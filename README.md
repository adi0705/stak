# Stak - PR Stacking Tool

A Go-based CLI tool that enables stacked PR workflows similar to Graphite. Stak manages branch dependencies, automates PR creation/updates, and maintains a clean single commit per PR.

## Features

- Create stacked PRs with automatic base branch management
- **Single commit per PR** - automatically squashes multiple commits
- Visualize branch dependencies as a tree
- Submit (push) changes and update PRs
- Navigate between branches with `stak up` and `stak down`
- Modify commits with `stak modify`
- Store metadata in git config and GitHub comments (no external dependencies)

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

1. Create your first stacked branch:
```bash
# On main branch
stak create feature-a
# Make changes and commit
git add . && git commit -m "Add feature A"
# Push and create PR
stak submit
# Enter PR title when prompted
```

2. Stack another branch on top:
```bash
stak create feature-b
# Make more commits
git add . && git commit -m "Add feature B"
# Push and create PR
stak submit
```

3. Visualize your stack:
```bash
stak list
# Output:
# main
# └─ feature-a (#1)
#    └─ feature-b (#2)
```

4. Navigate between branches:
```bash
stak down  # Move to feature-a
stak up    # Move back to feature-b
```

## Commands

### `stak init`

Initialize repository for stack. Verifies git setup and GitHub CLI authentication.

```bash
stak init
```

### `stak create`

Create a new branch stacked on top of the current branch. After creating the branch, make your changes, commit them, and run `stak submit` to create the PR.

```bash
stak create [branch-name]
```

The branch is created and checked out immediately. Make your changes, commit them, then use `stak submit` to create the PR.

### `stak list`

Display a tree visualization of all stacked branches.

```bash
stak list
```

### `stak sync`

Sync the current branch and its children with remote changes. Pulls latest changes from remote and rebases branches onto their parents.

**What it does:**
1. **Fetches from remote** - gets all latest changes
2. **Updates local parent branches** - pulls latest changes from remote for all parent branches
3. **Rebases current branch** onto its parent
4. **Recursively syncs children** - repeats for all dependent branches
5. **Automatic cleanup** - if any branch's PR has been merged on GitHub:
   - Deletes the local branch
   - Removes the metadata
   - Updates child branches to point to the new parent
   - Updates child PR bases on GitHub

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

Modify commits in the current branch by opening the commit editor. Changes are made locally only.

```bash
stak modify                # Opens commit editor to amend last commit (default)
stak modify --amend        # Same as above (explicit)
stak modify --rebase 3     # Interactive rebase last 3 commits
```

**Flags:**
- `--amend`: Amend the last commit (default behavior)
- `--rebase N`: Interactive rebase last N commits

After modifying, run `stak submit` to push changes and update the PR.

**Note:** If you quit the editor without saving changes, the modify operation is aborted.

### `stak submit`

Push changes and create/update PR. Automatically squashes multiple commits into one to maintain a single commit per PR.

```bash
stak submit              # Push current branch and update PR
```

**Behavior:**
- If no PR exists, prompts for title and creates one
- **Checks if parent branches have been merged** - if so, prompts you to run `stak sync` first
- If PR exists, pushes changes and updates it
- Automatically squashes multiple commits into one
- Updates stack visualization on all PRs in the stack

## Workflow Example

### Creating a Stack

```bash
# Start on main
git checkout main

# Create first branch
stak create auth-backend
# Make changes
git add . && git commit -m "Add authentication backend"
# Push and create PR
stak submit
# Enter title: "Add authentication backend"
# PR #1 created: auth-backend → main

# Create second branch stacked on first
stak create auth-frontend
# Make changes
git add . && git commit -m "Add authentication UI"
# Push and create PR
stak submit
# Enter title: "Add authentication UI"
# PR #2 created: auth-frontend → auth-backend

# Visualize
stak list
# main
# └─ auth-backend (#1)
#    └─ auth-frontend (#2)
```

### Modifying a Branch

```bash
# Make changes to auth-backend
stak down  # or: git checkout auth-backend
# Edit files and amend commit
stak modify
# Modify your changes in the editor, save and quit
# Push changes
stak submit
# PR #1 updated with your changes
```

### Making Additional Changes

```bash
# If you accidentally made multiple commits:
git commit -m "Change 1"
git commit -m "Change 2"
git commit -m "Change 3"

# stak submit automatically squashes them into one
stak submit
# ℹ Found 3 commits, squashing into one
# ✓ Commits squashed into one
# ✓ Pushed auth-backend
```

### Syncing with Remote

```bash
# After someone pushes changes to main on GitHub
git checkout auth-backend
stak sync
# ℹ Fetching from remote
# ℹ Updating local main to match origin/main
# ℹ Syncing branch auth-backend
# ℹ Rebasing auth-backend onto origin/main
# ✓ Synced auth-backend
# ℹ Syncing 1 child branch(es)
# ℹ Updating local auth-backend to match origin/auth-backend
# ℹ Syncing branch auth-frontend
# ℹ Rebasing auth-frontend onto origin/auth-backend
# ✓ Synced auth-frontend
```

### Automatic Cleanup After Merge

```bash
# After manually merging PR #1 on GitHub (auth-backend → main)
# Try to submit without syncing first
git checkout auth-frontend
stak submit
# ⚠ Stack is out of sync!
#
# The following parent branches have been merged on GitHub:
#   • auth-backend (PR #1)
#
# You need to sync first to update your stack:
#   stak sync

# Now sync to clean up
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

# Now submit works
stak submit
# ✓ Pushed auth-frontend
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

Branch relationships are stored in two places:

1. **Git config** (local):
```ini
[stack "branch.feature-a"]
    parent = main
    pr-number = 123

[stack "branch.feature-b"]
    parent = feature-a
    pr-number = 124
```

2. **GitHub PR comments** (remote):
Stack visualization comments include hidden machine-readable metadata that team members can restore using `stak restore <pr-number>`.

### Single Commit Per PR

Each PR maintains exactly one commit:

- When creating a PR, if multiple commits exist, they're automatically squashed into one
- When updating a PR, multiple commits are squashed before pushing
- This keeps PR history clean and makes rebasing easier
- Use `stak modify` to amend your single commit when making changes

### Branch Relationships

- Each branch tracks its parent, forming a tree
- PRs target parent branches (not always main)
- Stack visualization is posted as a comment on each PR in the stack

### Syncing Algorithm

1. Fetch from remote
2. Rebase current branch onto parent
3. Force push with `--force-with-lease`
4. Recursively rebase and push all children

## Conflict Resolution

When `stak sync` encounters conflicts, it provides detailed step-by-step guidance:

### What You'll See

```bash
stak sync

# If conflicts occur, you'll see:
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#   🔀 Rebase conflict on branch: feature-branch
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#
# 📝 Conflicted files:
#    • README.md
#    • cmd/submit.go
#
# 🔧 How to resolve conflicts:
#
#    1️⃣  Open the conflicted files in your editor
#       Look for conflict markers:
#       <<<<<<< HEAD
#       your changes
#       =======
#       incoming changes
#       >>>>>>> parent branch
#
#    2️⃣  Edit the files to keep the code you want
#       Remove the conflict markers (<<<<<<<, =======, >>>>>>>)
#
#    3️⃣  Stage the resolved files:
#       git add README.md
#       git add cmd/submit.go
#
#    4️⃣  Continue the sync:
#       stak sync --continue
```

### Resolution Steps

1. **Open the conflicted files** in your editor
2. **Find conflict markers** (<<<<<<< HEAD, =======, >>>>>>>)
3. **Edit the file** to keep the code you want
4. **Remove the conflict markers**
5. **Stage the resolved files**: `git add <file>`
6. **Continue**: `stak sync --continue`

### Aborting

If you want to undo the rebase:
```bash
git rebase --abort
```

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
- Use `stak modify` to amend commits instead of creating new ones
- Use `stak up` and `stak down` to navigate between branches in your stack
- Don't worry about multiple commits - `stak submit` automatically squashes them into one
- The workflow: `stak create` → make changes → `git commit` → `stak submit`

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
