#!/bin/bash
# Build script for stak
# Builds the binary and installs it to ~/.local/bin

set -e

echo "Building stak..."
go build -o bin/stak

echo "Installing to ~/.local/bin/stak..."
cp bin/stak /Users/adi/.local/bin/stak

echo "✓ Build complete. stak installed at /Users/adi/.local/bin/stak"
