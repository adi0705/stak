# Stak - PR Stacking Tool

A Go-based CLI tool that enables stacked PR workflows. Stack manages branch dependencies, automates PR creation/updates, and handles merging in the correct order.

## Features

- Create stacked PRs with automatic base branch management
- Visualize branch dependencies as a tree
- Sync changes across entire stak with automatic rebasing
- Submit (merge) PRs in correct order with automatic base updates
- Modify PRs and propagate changes to children
- Store metadata in git config (no external dependencies)

## Installation

### Homebrew (Recommended - Coming Soon)

```bash
brew tap adi0705/stak
brew install stak
```

Or in a single command:
```bash
brew install adi0705/stak/stak
```

### Direct Download

Download the latest release for your platform from [GitHub Releases](https://github.com/adi0705/stak/releases):

- **macOS (Apple Silicon)**: `stak-darwin-arm64`
- **macOS (Intel)**: `stak-darwin-amd64`
- **Linux (amd64)**: `stak-linux-amd64`
- **Linux (arm64)**: `stak-linux-arm64`
- **Windows**: `stak-windows-amd64.exe`

Then install:

```bash
# Make the binary executable
chmod +x ~/Downloads/stak-darwin-arm64

# Allow macOS to run the binary (removes quarantine attribute on macOS)
xattr -d com.apple.quarantine ~/Downloads/stak-darwin-arm64

# Move to local bin (ensure ~/.local/bin is in your PATH)
mv ~/Downloads/stak-darwin-arm64 ~/.local/bin/stak

# Verify installation
stak --version
```

### Build from Source

Use the provided build script (recommended):

```bash
./build.sh
```

This will build and install stak to `~/.local/bin/stak`.

Or manually:

```bash
go build -o bin/stak
cp bin/stak ~/.local/bin/
```

Or use it directly from the project directory:

```bash
go build -o bin/stak
./bin/stak --help
```

## Prerequisites

- Git
- GitHub CLI (`gh`) - Install with `brew install gh` or see [GitHub CLI docs](https://cli.github.com/)

**Important:** Authenticate GitHub CLI before using stak:
```bash
gh auth login
```

## Quick Start

1. Initialize your repository:
```bash
stak init
```

2. Create your first stacked branch:
```bash
# On main branch - create new branch with stak
stak create feature-a

# Make commits
git add . && git commit -m "Add feature A"

# Submit PR
stak submit
```

3. Stack another branch on top:
```bash
# Create second branch stacked on feature-a
stak create feature-b

# Make more commits
git add . && git commit -m "Add feature B"

# Submit PR (will be based on feature-a)
stak submit
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

### `stak create` (alias: `c`)

Create a new branch stacked on top of the current branch and create a PR.

```bash
stak create [branch-name]
stak create feature -am "Add feature"  # Create, stage all, and commit
stak create --title "My PR title" --body "Description"
stak create --draft  # Create as draft PR
```

**Flags:**
- `--title, -t`: PR title (will prompt if not provided)
- `--body, -b`: PR description
- `--draft`: Create as draft PR
- `--all, -a`: Stage all changes
- `--message, -m`: Commit message (implies -a if no staged changes)

### `stak list` (alias: `ls`)

Display a tree visualization of all stacked branches.

```bash
stak list
```

### `stak up` (alias: `u`)

Move to the parent branch of the current branch in the stack.

```bash
stak up        # Move up one level
stak up 3      # Move up 3 levels
```

### `stak down` (alias: `d`)

Move to a child branch of the current branch in the stack. If multiple children exist, shows an interactive menu to select one.

```bash
stak down      # Move down one level
stak down 2    # Move down 2 levels
```

### `stak top` (alias: `t`)

Jump to the topmost branch in the current stack.

```bash
stak top
```

### `stak bottom` (alias: `b`)

Jump to the bottommost branch in the current stack. If multiple paths exist, shows an interactive menu.

```bash
stak bottom
```

### `stak checkout` (alias: `co`)

Smart branch switching with stack context. Shows an interactive menu with parent/children information.

```bash
stak checkout              # Interactive selection
stak checkout feature-a    # Direct checkout
```

### `stak log` (alias: `lg`)

Show detailed information about all branches in the stack, including PR status, reviews, CI checks, and commit counts.

```bash
stak log          # Detailed view with PR information
stak log --short  # Simple tree view (same as list)
```

**Displays:**
- Branch name and parent
- PR number and title
- State (Open/Merged/Draft/Closed)
- Review status (Approved/Changes Requested/Pending)
- CI status (Passing/Failing/Running)
- Commit count

### `stak track` (alias: `tr`)

Add an existing branch to the stack by designating its parent branch. This allows you to incorporate branches not created with `stak create` into the stack system.

```bash
# Track current branch (interactive parent selection)
stak track

# Track specific branch
stak track feature/existing-branch

# Auto-detect parent from PR base
stak track --auto

# Specify parent explicitly
stak track --parent main

# Use most recent tracked ancestor as parent
stak track --force
```

**Flags:**
- `--parent <branch>`: Specify the parent branch explicitly
- `--auto`: Auto-detect parent from associated PR's base branch (requires PR to exist)
- `--force`: Automatically set parent to most recent tracked ancestor
- `--recursive`: Recursively track untracked parents without prompting

**Use cases:**
- Add manually created branches to your stack
- Fix missing metadata for branches
- Repair stack relationships after git operations
- Re-parent branches that were tracked incorrectly

### `stak sync` (alias: `sy`)

Sync **all branches** with stack metadata with remote changes. Rebases each branch onto its parent in dependency order.

**Syncs Everything:** Unlike traditional tools, `stak sync` always syncs ALL your stacked branches:
- Fetches latest changes from remote
- Updates base branches (main, etc.) from remote first
- Syncs all stack branches in correct dependency order (parents before children)
- Works across independent stacks

**Automatic Cleanup:** If a branch's PR has been merged on GitHub, `stak sync` will automatically:
- Delete the local branch
- Remove the metadata
- Update child branches to point to the new parent
- Update child PR bases on GitHub

**Smart Branch Selection:** If the current branch is deleted during sync (because its PR was merged):
- Automatically moves to another stack branch
- Falls back to main if no stack branches remain
- Ensures you never end up on a deleted branch

```bash
stak sync
stak sync --continue      # Continue after resolving conflicts
```

**Flags:**
- `--continue`: Continue sync after resolving conflicts

### `stak modify` (alias: `m`)

Modify the current branch and sync all children.

**Interactive Mode:** When run without flags and no staged changes exist, shows an interactive menu:
- Commit all file changes (--all)
- Select changes to commit (--patch)
- Just edit the commit message
- Abort this operation

```bash
stak modify                # Interactive menu (if no staged changes)
stak modify                # Push changes and sync children (if changes staged)
stak modify --amend        # Amend last commit
stak modify -c             # Create fresh commit (not amend)
stak modify -cam "Update"  # Commit all with message
stak modify --rebase 3     # Interactive rebase last 3 commits
stak modify --edit --title "New title"  # Update PR details
stak modify --push-only    # Only push, skip syncing children
stak modify --into parent  # Apply changes to parent branch
```

**Flags:**
- `--amend`: Amend the last commit
- `-c, --commit`: Create a fresh commit instead of amending
- `--rebase N`: Interactive rebase last N commits
- `--edit`: Edit PR title/body
- `--title`: New PR title
- `--body`: New PR body
- `--push-only`: Only push changes, skip syncing children
- `--into <branch>`: Apply changes to downstack (ancestor) branch

### `stak submit` (alias: `s`)

Create or update pull requests for branches in the stack. **Note:** This command does NOT merge PRs - use `stak merge` for that.

```bash
stak submit                # Create/update PR for current branch
stak submit --stack        # Create/update PRs for entire stack
stak submit --update-only  # Only update existing PRs, don't create new
stak submit --draft        # Create PRs as drafts
```

**Flags:**
- `-s, --stack`: Submit entire stack from current branch
- `-u, --update-only`: Only update existing PRs, don't create new
- `--draft`: Create PRs as drafts

### `stak merge` (alias: `mg`)

Merge approved PRs in the correct order (bottom to top). After each merge, updates dependent PRs and rebases children.

```bash
stak merge              # Merge current branch PR
stak merge --all        # Merge entire stack
stak merge --method merge  # Use merge instead of squash
stak merge --skip-checks   # Skip approval/CI checks
```

**Flags:**
- `--all`: Merge entire stack from current branch
- `--method`: Merge method: squash (default), merge, or rebase
- `--skip-checks`: Skip approval and CI checks

### `stak untrack` (alias: `ut`)

Stop tracking a branch without deleting it or its PR.

```bash
stak untrack              # Untrack current branch
stak untrack feature-a    # Untrack specific branch
stak untrack --recursive  # Untrack branch and all children
stak untrack --force      # Skip confirmation prompts
```

**Flags:**
- `-f, --force`: Skip confirmation prompts
- `-r, --recursive`: Recursively untrack all children

**Use cases:**
- Remove branches from stack tracking
- Clean up metadata for branches you no longer want managed
- Untrack before deleting branches

### `stak move` (alias: `mv`)

Change a branch's parent by rebasing it onto a different branch.

```bash
stak move                  # Interactive parent selection
stak move feature-b        # Move specific branch
stak move --parent main    # Explicit new parent
```

**Flags:**
- `--parent <branch>`: Specify new parent branch

**What it does:**
- Rebases branch onto new parent
- Updates metadata and PR base
- Prevents circular dependencies
- Syncs all children after move

### `stak fold` (alias: `fd`)

Merge a branch into its parent, combining the commits.

```bash
stak fold                  # Fold current branch into parent
stak fold feature-b        # Fold specific branch
stak fold --no-squash      # Merge without squashing
stak fold --force          # Skip confirmation
```

**Flags:**
- `--squash`: Squash commits when folding (default: true)
- `-f, --force`: Skip confirmation prompts

**What it does:**
- Merges branch commits into parent
- Updates children to point to parent
- Closes PR and deletes branch
- Rebases children onto parent

### `stak squash` (alias: `sq`)

Consolidate all commits in a branch into a single commit.

```bash
stak squash                       # Interactive commit message
stak squash -m "Final version"    # With message
stak squash feature-b             # Squash specific branch
```

**Flags:**
- `-m, --message <msg>`: Commit message for squashed commit

**What it does:**
- Resets branch to parent (keeping changes)
- Creates single commit with all changes
- Force pushes branch
- Syncs children

### `stak pop` (alias: `pp`)

Remove a branch from the stack while preserving its changes.

```bash
stak pop                  # Pop current branch
stak pop feature-b        # Pop specific branch
stak pop --keep           # Don't delete the branch
stak pop --force          # Skip confirmation
```

**Flags:**
- `--keep`: Keep the branch (don't delete it)
- `-f, --force`: Skip confirmation prompts

**What it does:**
- Stashes uncommitted changes
- Switches to parent branch
- Updates children to point to parent
- Closes PR (if exists)
- Optionally deletes branch
- Shows how to apply/discard stashed changes

### `stak reorder` (alias: `ro`)

Interactively reorder branches in the stack by changing their parent relationships.

```bash
stak reorder              # Interactive reordering
```

**What it does:**
- Shows current stack order
- Prompts for new order (comma-separated numbers)
- Rebases branches onto new parents in new order
- Updates all metadata and PR bases
- Force pushes all affected branches

**Example:**
```
Current order:
1. feature-a (main)
2. feature-b (feature-a)
3. feature-c (feature-b)

Enter new order: 1,3,2
→ feature-c will be moved to branch from feature-a
→ feature-b will branch from feature-c
```

### `stak split` (alias: `sp`)

Split a branch into two branches at a specific commit point.

```bash
stak split                       # Interactive commit selection
stak split feature-a             # Split specific branch
stak split --at abc123           # Split at specific commit
stak split --name feature-a-2    # Specify new branch name
```

**Flags:**
- `--at <commit>`: Commit hash to split at
- `--name <branch>`: Name for the new branch (default: original-name-2)

**What it does:**
- Shows all commits in branch
- Lets you select split point
- Creates new branch with commits after split point
- Original branch keeps commits up to split point
- Updates children to point to new branch
- Updates PR bases for children
- Creates PR for new branch

### `stak absorb` (alias: `ab`)

Distribute staged changes to appropriate commits automatically.

**Requires:** `git-absorb` must be installed
- macOS: `brew install git-absorb`
- Linux/Windows: `cargo install git-absorb`

```bash
# Make changes to files
git add .
stak absorb
# Automatically determines which commits changes belong to
# Updates commits in place
```

**What it does:**
- Uses `git-absorb` to analyze staged changes
- Matches each change to the commit that last touched those lines
- Amends changes to appropriate commits
- Force pushes branch
- Syncs all children

**Use case:** Perfect for fixing typos or small changes after review without creating new commits.

### `stak undo` (alias: `un`)

View recent stack operations and get guidance on how to undo them.

```bash
stak undo                 # Show last operation and undo guidance
stak undo --force         # Skip confirmation
```

**Flags:**
- `-f, --force`: Skip confirmation when removing from history

**What it does:**
- Shows details of last operation
- Provides specific guidance on how to undo it
- Offers to remove operation from history
- Maintains operation log in `.git/stak.log`

**Note:** Automatic undo is not yet fully implemented. This command provides manual undo guidance for each operation type.

### `stak get` (alias: `gt`)

Download and automatically track a colleague's stack from the remote repository.

```bash
stak get feature-branch              # Download branch and detect stack
stak get feature-branch --user john  # Specify GitHub user/org
```

**Flags:**
- `--user <username>`: Specify the GitHub user or organization (default: auto-detect from remote)

**What it does:**
- Fetches the specified branch from remote
- Creates local tracking branch
- Detects PR and stack structure automatically
- Walks up the stack to find ancestor branches
- Walks down the stack to find descendant branches
- Tracks all branches in the stack with correct parent relationships
- Checks out the requested branch

**Use case:** Perfect for reviewing or collaborating on a colleague's stacked PRs. One command downloads the entire stack structure.

### `stak freeze` (alias: `fr`)

Protect a branch from modifications by stack operations.

```bash
stak freeze                 # Freeze current branch
stak freeze feature-a       # Freeze specific branch
```

**What it does:**
- Marks branch as frozen in metadata
- Prevents modifications by:
  - `stak modify` operations
  - `stak sync` rebasing
  - `stak move` parent changes
  - Other destructive operations
- Useful for protecting stable branches while working on dependents

**Use case:** Freeze a branch after it's approved to prevent accidental modifications while working on dependent branches.

### `stak unfreeze` (alias: `uf`)

Remove protection from a frozen branch to allow modifications again.

```bash
stak unfreeze               # Unfreeze current branch
stak unfreeze feature-a     # Unfreeze specific branch
```

**What it does:**
- Removes frozen marker from branch
- Allows stack operations to modify the branch again

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
# After changes in main, sync from any branch
git checkout auth-backend  # or any other branch
stak sync
# Updates main from remote
# Rebases auth-backend onto main
# Rebases auth-frontend onto auth-backend
# Syncs ALL branches with stack metadata
```

### Automatic Cleanup After Merge

```bash
# After manually merging PR #1 on GitHub (auth-backend → main)
git checkout auth-backend  # On the merged branch
stak sync
# ℹ Fetching from remote
# ℹ Syncing 1 stack branch(es)
# ℹ Updating base branch main from remote
# ℹ Checking for merged branches
# ℹ PR #1 for branch auth-backend is merged, cleaning up
# ℹ Updating auth-frontend parent: auth-backend → main
# ℹ Updated PR #2 base to main
# ℹ Switching to main
# ℹ Deleting local branch auth-backend
# ✓ Deleted branch auth-backend
# ℹ Branch auth-backend was deleted, finding alternative
# ℹ Moving to auth-frontend
# ℹ Syncing branch auth-frontend
# ℹ Rebasing auth-frontend onto origin/main
# ✓ Synced auth-frontend
# ✓ Sync completed successfully
# (Now on auth-frontend branch automatically)
```

### Tracking Existing Branches

```bash
# You created branches manually without stak create
git checkout feature/pr1-add-brotli-compression
git checkout -b feature/pr2-add-question-endpoint
# ... made some changes and created PRs ...

# Now add them to the stack
git checkout feature/pr1-add-brotli-compression
stak track --auto  # Auto-detects parent from PR base
# ✓ Detected PR #4 with base: main
# ✓ Tracked feature/pr1-add-brotli-compression with parent main

git checkout feature/pr2-add-question-endpoint
stak track --parent feature/pr1-add-brotli-compression
# ✓ Tracked feature/pr2-add-question-endpoint with parent feature/pr1-add-brotli-compression

# Now you can use stak commands with these branches
stak list
# main
# └─ feature/pr1-add-brotli-compression (#4)
#    └─ feature/pr2-add-question-endpoint (#5)

# Navigate the stack
stak up    # Move to parent
stak down  # Move to child
```

### Submitting and Merging a Stack

```bash
# Create/update PRs for entire stack
git checkout auth-frontend
stak submit --stack
# Creates/updates PR #1 for auth-backend
# Creates/updates PR #2 for auth-frontend

# After PRs are approved, merge them
stak merge --all
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

## Command Aliases

All commands support shorthand aliases for faster workflow:

- `c` → create
- `m` → modify
- `u` → up
- `d` → down
- `t` → top
- `b` → bottom
- `co` → checkout
- `tr` → track
- `s` → submit
- `mg` → merge
- `ls` → list
- `lg` → log
- `ut` → untrack
- `mv` → move
- `fd` → fold
- `sq` → squash
- `pp` → pop
- `ro` → reorder
- `sp` → split
- `ab` → absorb
- `un` → undo
- `gt` → get
- `fr` → freeze
- `uf` → unfreeze
- `sy` → sync

Examples:
```bash
stak c feature -am "Add feature"  # Create and commit
stak t                             # Jump to top
stak d 2                           # Move down 2 levels
stak m -c                          # Fresh commit
stak lg                            # Detailed log
stak s --stack                     # Submit entire stack
stak mg --all                      # Merge entire stack
stak mv --parent main              # Move to different parent
stak fd                            # Fold into parent
stak sq -m "Clean commit"          # Squash all commits
stak pp --keep                     # Pop branch, keep it locally
stak ro                            # Reorder stack
stak sp --at abc123                # Split at commit
stak ab                            # Absorb staged changes
stak un                            # View undo history
stak gt feature-x                  # Download colleague's stack
stak fr                            # Freeze current branch
stak uf feature-a                  # Unfreeze specific branch
```

## Tips

- Use `stak log` (or `stak lg`) to see detailed PR status information
- Use `stak list` (or `stak ls`) for a quick tree visualization
- Run `stak sync` after merging PRs on GitHub to automatically clean up merged branches
- Always sync before making new changes: `stak sync`
- Use `stak submit --draft` when creating WIP PRs
- Use `stak submit --update-only` to push updates without creating new PRs
- Separate concerns: `stak submit` for PRs, `stak merge` for merging
- Use `stak modify --amend` for quick fixes
- Use `stak create -am "message"` for quick branch creation with commit
- Use `stak top` and `stak bottom` for quick navigation
- Use `stak untrack` to remove branches from stack without deleting them
- Use `stak move` to reorganize your stack structure
- Use `stak fold` to combine a branch into its parent
- Use `stak squash` to clean up commit history before review
- Use `stak pop` to remove a branch while keeping your work
- Use `stak reorder` to rearrange the order of branches in your stack
- Use `stak split` to divide large branches into smaller, focused PRs
- Use `stak absorb` to automatically fix small changes without new commits (requires git-absorb)
- Use `stak undo` to view operation history and get undo guidance
- Use `stak get` to download and track a colleague's entire stack with one command
- Use `stak freeze` to protect approved branches from accidental modifications
- Use `stak unfreeze` when you need to make changes to a frozen branch

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
