#!/bin/bash
# Release script for stak
# Usage: ./release.sh v1.0.0

set -e

VERSION=$1

if [ -z "$VERSION" ]; then
    echo "Usage: ./release.sh <version>"
    echo "Example: ./release.sh v1.0.0"
    exit 1
fi

# Validate version format
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Error: Version must be in format vX.Y.Z (e.g., v1.0.0)"
    exit 1
fi

echo "Creating release $VERSION"

# Create release directory
RELEASE_DIR="release/$VERSION"
mkdir -p "$RELEASE_DIR"

# Build for different platforms
echo "Building binaries..."

# macOS (Intel)
echo "  - macOS (Intel)"
GOOS=darwin GOARCH=amd64 go build -o "$RELEASE_DIR/stak-darwin-amd64" -ldflags "-X main.version=$VERSION"

# macOS (Apple Silicon)
echo "  - macOS (Apple Silicon)"
GOOS=darwin GOARCH=arm64 go build -o "$RELEASE_DIR/stak-darwin-arm64" -ldflags "-X main.version=$VERSION"

# Linux (amd64)
echo "  - Linux (amd64)"
GOOS=linux GOARCH=amd64 go build -o "$RELEASE_DIR/stak-linux-amd64" -ldflags "-X main.version=$VERSION"

# Linux (arm64)
echo "  - Linux (arm64)"
GOOS=linux GOARCH=arm64 go build -o "$RELEASE_DIR/stak-linux-arm64" -ldflags "-X main.version=$VERSION"

# Windows (amd64)
echo "  - Windows (amd64)"
GOOS=windows GOARCH=amd64 go build -o "$RELEASE_DIR/stak-windows-amd64.exe" -ldflags "-X main.version=$VERSION"

# Create checksums
echo "Generating checksums..."
cd "$RELEASE_DIR"
shasum -a 256 stak-* > checksums.txt
cd ../..

echo "✓ Binaries built in $RELEASE_DIR"
echo ""
echo "Next steps:"
echo "1. Test the binaries"
echo "2. Create git tag: git tag $VERSION"
echo "3. Push tag: git push origin $VERSION"
echo "4. Create GitHub release and upload binaries from $RELEASE_DIR"
echo ""
echo "Or use GitHub CLI:"
echo "  gh release create $VERSION $RELEASE_DIR/* --title \"$VERSION\" --notes \"Release notes here\""
