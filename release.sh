#!/bin/bash
set -e

# Get the version from main.go
VERSION=$(grep 'const version' main.go | sed 's/.*"\(.*\)".*/\1/')

if [ -z "$VERSION" ]; then
    echo "Error: Could not extract version from main.go"
    exit 1
fi

echo "Building version $VERSION..."

# Build the application
go build -ldflags="-s -w" -o claude-session-browser

# Install locally
echo "Installing to ~/.local/bin/..."
mv claude-session-browser ~/.local/bin/

echo ""
echo "✅ Build complete!"
echo "   Version: $VERSION"
echo "   Binary installed to: ~/.local/bin/claude-session-browser"
echo ""
echo "Run 'claude-session-browser' to test"
