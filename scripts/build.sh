#!/bin/bash
# LocalMind Extension Build Script for Unix systems
# Usage: ./build.sh [platform]
# Platforms: darwin, linux, all (default)

set -e

PLATFORM=${1:-all}
PACKAGE=${2:-false}

PROJECT_ROOT=$(dirname "$(dirname "$(realpath "$0")")")
CORE_DIR="$PROJECT_ROOT/localmind/packages/core"
EXTENSION_DIR="$PROJECT_ROOT/localmind/packages/extension"
BIN_DIR="$EXTENSION_DIR/bin"

# Ensure bin directory exists
mkdir -p "$BIN_DIR"

build_binary() {
    local GOOS=$1
    local GOARCH=$2
    local OUTPUT=$3

    echo "Building for $GOOS/$GOARCH..."
    
    cd "$CORE_DIR"
    GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 go build -tags nocgo -ldflags="-s -w" -o "$BIN_DIR/$OUTPUT" ./cmd/localmind
    echo "  Built: $OUTPUT"
}

# Build for requested platforms
case "$PLATFORM" in
    darwin)
        build_binary darwin amd64 localmind-darwin-amd64
        build_binary darwin arm64 localmind-darwin-arm64
        ;;
    linux)
        build_binary linux amd64 localmind-linux-amd64
        build_binary linux arm64 localmind-linux-arm64
        ;;
    windows)
        build_binary windows amd64 localmind-windows-amd64.exe
        ;;
    all)
        build_binary windows amd64 localmind-windows-amd64.exe
        build_binary darwin amd64 localmind-darwin-amd64
        build_binary darwin arm64 localmind-darwin-arm64
        build_binary linux amd64 localmind-linux-amd64
        build_binary linux arm64 localmind-linux-arm64
        ;;
    *)
        echo "Unknown platform: $PLATFORM"
        echo "Usage: $0 [darwin|linux|windows|all]"
        exit 1
        ;;
esac

# Build TypeScript extension
echo "Building TypeScript extension..."
cd "$EXTENSION_DIR"
npm run compile

# Package VSIX if requested
if [ "$PACKAGE" = "package" ]; then
    echo "Packaging VSIX..."
    
    # Check if vsce is available
    if ! command -v vsce &> /dev/null; then
        echo "Installing vsce..."
        npm install -g @vscode/vsce
    fi
    
    vsce package --no-dependencies
    echo "VSIX package created"
fi

echo ""
echo "Build complete!"
echo "Binaries in: $BIN_DIR"
