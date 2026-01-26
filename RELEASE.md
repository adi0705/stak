# Release Guide for Stak

This document explains how to create releases and distribute stak through GitHub Releases and Homebrew.

## Prerequisites

1. **GitHub CLI (`gh`)** installed and authenticated
   ```bash
   brew install gh
   gh auth login
   ```

2. **Write access** to the repository

## Creating a Release

### Step 1: Choose Version Number

Follow [Semantic Versioning](https://semver.org/):
- **MAJOR** version (v2.0.0): Breaking changes
- **MINOR** version (v1.1.0): New features, backwards compatible
- **PATCH** version (v1.0.1): Bug fixes, backwards compatible

### Step 2: Update Changelog (Optional but Recommended)

Create/update `CHANGELOG.md` with release notes:

```markdown
## [1.0.0] - 2026-01-26

### Added
- `stak track` command to add existing branches to stack
- Auto-detection of parent from PR base with `--auto` flag
- Cycle detection for branch relationships

### Fixed
- `stak up` navigation to untracked parents
- Force push behavior in `stak modify`

### Changed
- Relaxed parent metadata check in navigation commands
```

### Step 3: Build Release Binaries

Run the release script:

```bash
./release.sh v1.0.0
```

This will:
- Build binaries for macOS (Intel & Apple Silicon), Linux (amd64 & arm64), and Windows
- Generate checksums
- Place everything in `release/v1.0.0/`

### Step 4: Test Binaries

Test at least one binary to ensure it works:

```bash
./release/v1.0.0/stak-darwin-arm64 --version
./release/v1.0.0/stak-darwin-arm64 --help
```

### Step 5: Create Git Tag

```bash
git tag v1.0.0
git push origin v1.0.0
```

### Step 6: Create GitHub Release

#### Option A: Using GitHub CLI (Recommended)

```bash
gh release create v1.0.0 \
  release/v1.0.0/* \
  --title "v1.0.0" \
  --notes "$(cat <<EOF
## What's New

- New \`stak track\` command to add existing branches to stack
- Fixed \`stak up\` navigation to untracked parents
- Added auto-detection of parent from PR base

## Installation

### Direct Download

Download the appropriate binary for your platform:
- **macOS (Apple Silicon)**: stak-darwin-arm64
- **macOS (Intel)**: stak-darwin-amd64
- **Linux (amd64)**: stak-linux-amd64
- **Windows**: stak-windows-amd64.exe

Then:
\`\`\`bash
chmod +x stak-darwin-arm64  # Make executable (macOS/Linux)
sudo mv stak-darwin-arm64 /usr/local/bin/stak  # Move to PATH
\`\`\`

### Homebrew (coming soon)

\`\`\`bash
brew tap adi0705/stak
brew install stak
\`\`\`

## Full Changelog
See [CHANGELOG.md](https://github.com/adi0705/stak/blob/main/CHANGELOG.md)
EOF
)"
```

#### Option B: Using GitHub Web UI

1. Go to https://github.com/adi0705/stak/releases/new
2. Select tag: `v1.0.0`
3. Title: `v1.0.0`
4. Upload all files from `release/v1.0.0/`
5. Add release notes (see template above)
6. Click "Publish release"

## Setting Up Homebrew Distribution

Homebrew formulas are distributed through "taps" (third-party repositories).

### Step 1: Create Homebrew Tap Repository

1. Create a new GitHub repository named `homebrew-stak`
   ```bash
   gh repo create adi0705/homebrew-stak --public --description "Homebrew tap for stak"
   ```

2. Clone it locally:
   ```bash
   cd ..
   git clone https://github.com/adi0705/homebrew-stak.git
   cd homebrew-stak
   ```

### Step 2: Create Formula

1. Copy the formula template:
   ```bash
   cp ../stak/Formula/stak.rb Formula/stak.rb
   ```

2. Update version and URLs in `Formula/stak.rb`:
   ```ruby
   version "1.0.0"
   url "https://github.com/adi0705/stak/releases/download/v1.0.0/stak-darwin-arm64"
   ```

3. Get SHA256 checksums from the release:
   ```bash
   # From the checksums.txt file in your release
   cat ../stak/release/v1.0.0/checksums.txt
   ```

4. Update the `sha256` values in the formula

5. Commit and push:
   ```bash
   git add Formula/stak.rb
   git commit -m "Add stak formula v1.0.0"
   git push origin main
   ```

### Step 3: Test Homebrew Installation

```bash
# Add the tap
brew tap adi0705/stak

# Install stak
brew install stak

# Test
stak --version
```

### Step 4: Update README

Add Homebrew installation instructions to the main README:

```markdown
## Installation

### Homebrew (macOS/Linux)

```bash
brew install adi0705/stak/stak
```

### Direct Download

Download from [GitHub Releases](https://github.com/adi0705/stak/releases)
```

## Updating the Homebrew Formula for New Releases

When you create a new release (e.g., v1.1.0):

1. Build and publish the GitHub release (steps above)

2. Update the Homebrew formula:
   ```bash
   cd ../homebrew-stak
   ```

3. Edit `Formula/stak.rb`:
   - Update `version`
   - Update all `url` fields with new version
   - Update all `sha256` checksums (from new release's checksums.txt)

4. Commit and push:
   ```bash
   git add Formula/stak.rb
   git commit -m "Update stak to v1.1.0"
   git push origin main
   ```

5. Users update with:
   ```bash
   brew update
   brew upgrade stak
   ```

## Automated Release Script (Optional)

For fully automated releases, create a GitHub Actions workflow:

`.github/workflows/release.yml`:
```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - uses: actions/setup-go@v4
        with:
          go-version: '1.24'

      - name: Build
        run: |
          ./release.sh ${{ github.ref_name }}

      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          files: release/${{ github.ref_name }}/*
          generate_release_notes: true
```

Then releases are automatically created when you push a tag:
```bash
git tag v1.0.0
git push origin v1.0.0
# GitHub Actions will build and create the release
```

## Distribution Comparison

| Method | Pros | Cons |
|--------|------|------|
| **GitHub Releases** | Simple, no extra repos, direct download | Manual download, no auto-updates |
| **Homebrew** | Easy install/update, version management | Requires separate tap repo, more setup |

Recommendation: **Support both** - GitHub Releases for direct downloads, Homebrew for package management.

## Quick Reference

```bash
# Create release
./release.sh v1.0.0
git tag v1.0.0
git push origin v1.0.0
gh release create v1.0.0 release/v1.0.0/* --title "v1.0.0" --notes "..."

# Update Homebrew
cd ../homebrew-stak
# Edit Formula/stak.rb with new version and checksums
git commit -am "Update to v1.0.0"
git push
```
