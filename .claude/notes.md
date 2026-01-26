# Development Notes for stak

## Build and Installation

**IMPORTANT**: Whenever building stak, always install it to `/Users/adi/.local/bin/stak`

### Build and Install Command
```bash
./build.sh
```

Or manually:
```bash
go build -o bin/stak
cp bin/stak /Users/adi/.local/bin/stak
```

## Repository Configuration

- **GitHub Repo**: https://github.com/adi0705/stak
- **Binary Location**: `/Users/adi/.local/bin/stak` (in user's PATH)
- **Build Directory**: `bin/` (git ignored)

## Key Implementation Details

- Uses `gh` CLI for GitHub operations (PR creation, comments, etc.)
- Stores stack metadata in git config (`.git/config`)
- Branch relationships stored as `stack.branch.<name>.parent` and `stack.branch.<name>.pr-number`
- Interactive prompts use `promptui` library
- Force push only when history is rewritten (amend/rebase)
- Navigation commands (`up`/`down`) stay within stack boundaries
