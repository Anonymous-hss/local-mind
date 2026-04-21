#!/bin/bash
# Build script for LocalMind
# Usage: ./scripts/build.sh [core|extension|all]

set -e

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

build_core() {
    echo "Building Core Engine (Go)..."
    cd "$ROOT_DIR/packages/core"
    go build -o bin/localmind ./cmd/localmind
    echo "Core engine built: packages/core/bin/localmind"
}

build_extension() {
    echo "Building VS Code Extension..."
    cd "$ROOT_DIR/packages/extension"
    npm install
    npm run compile
    echo "Extension compiled: packages/extension/out/"
}

case "${1:-all}" in
    core)
        build_core
        ;;
    extension)
        build_extension
        ;;
    all)
        build_core
        build_extension
        ;;
    *)
        echo "Usage: $0 [core|extension|all]"
        exit 1
        ;;
esac

echo "Build complete!"
