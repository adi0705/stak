# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `stak track` command to add existing branches to stack
  - Interactive parent selection with categorized options
  - Auto-detection of parent from PR base (`--auto` flag)
  - Force mode to find most recent tracked ancestor (`--force` flag)
  - Recursive tracking for untracked parent chains (`--recursive` flag)
  - Cycle detection to prevent circular dependencies
  - Support for updating existing branch metadata
- `stak up` and `stak down` commands for stack navigation
- Interactive modify menu when no staged changes exist
- Version flag (`--version` or `-v`)

### Fixed
- `stak up` navigation to untracked parents (relaxed strict metadata check)
- `stak modify` force push behavior (now conditional based on history rewriting)
- PR creation repository resolution error (removed unnecessary `--head` flag)
- Tree visualization display for root branch children

### Changed
- Updated installation path to `~/.local/bin/stak`
- Improved error messages with actionable suggestions
- Build script now installs to `~/.local/bin` automatically

## [1.0.0] - YYYY-MM-DD (Template for first release)

### Added
- Initial release
- Core commands: `init`, `create`, `list`, `sync`, `modify`, `submit`
- Stack visualization with tree display
- Automatic PR creation and management
- Branch metadata storage in git config
- GitHub CLI integration
- Interactive prompts for user input
- Conflict resolution workflow
- Automatic cleanup after PR merges
