#!/bin/bash
set -e

# Universal Build Script for LuciCodex
# This builds for ALL OpenWrt architectures

VERSION="1.0.0"
BUILD_DIR="build"
DIST_DIR="dist"

echo "🚀 LuciCodex Universal Build Script"
echo "===================================="
echo ""

# Architectures to build
ARCHS=(
    "mips:mips:softfloat"
    "mipsle:mipsle:softfloat"
    "arm:arm:6"
    "arm64:aarch64"
    "x86:386"
    "x86_64:amd64"
)

mkdir -p "$BUILD_DIR" "$DIST_DIR"

echo "📦 Building Go binaries..."
echo ""

for arch_config in "${ARCHS[@]}"; do
    IFS=':' read -r openwrt_arch go_arch go_extra <<< "$arch_config"
    
    echo "Building for $openwrt_arch (GOARCH=$go_arch)..."
    
    export GOOS=linux
    export GOARCH="$go_arch"
    
    # Set ARM version if needed
    if [ "$go_arch" = "arm" ] && [ -n "$go_extra" ]; then
        export GOARM="$go_extra"
    fi
    
    # Set MIPS features if needed
    if [[ "$go_arch" =~ ^mips ]]; then
        export GOMIPS="$go_extra"
    fi
    
    output="$BUILD_DIR/lucicodex-$openwrt_arch"
    
    go build -ldflags="-s -w" -o "$output" ./cmd/lucicodex
    
    if [ $? -eq 0 ]; then
        size=$(du -h "$output" | cut -f1)
        echo "  ✓ Built: $output ($size)"
    else
        echo "  ✗ Failed to build $openwrt_arch"
        exit 1
    fi
    
    echo ""
done

echo "✅ All binaries built successfully!"
echo ""
echo "📋 Build Summary:"
ls -lh "$BUILD_DIR"
echo ""
echo "🎯 Next steps:"
echo "  1. Test binaries on actual hardware"
echo "  2. Create IPK packages: ./scripts/package.sh"
echo "  3. Upload to releases"
