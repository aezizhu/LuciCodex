#!/bin/sh
set -e

# LuciCodex Universal Installer
# Run this script on your OpenWrt router to install the latest version.

REPO="aezizhu/LuciCodex"
BASE_URL="https://github.com/$REPO/releases/latest/download"

echo "=== LuciCodex Installer ==="

# 1. Detect Architecture
ARCH=$(uname -m)
echo "Detected Architecture: $ARCH"

case "$ARCH" in
  x86_64)
    TARGET="amd64"
    ;;
  aarch64)
    TARGET="arm64"
    ;;
  armv7*|armv8*)
    # Most modern ARM routers (Cortex-A7/A9/A53 etc)
    TARGET="arm"
    ;;
  mips)
    TARGET="mips"
    ;;
  mipsle)
    TARGET="mipsle"
    ;;
  *)
    echo "⚠️  Warning: Unknown architecture '$ARCH'. Trying to detect via opkg..."
    if opkg print-architecture | grep -q "aarch64"; then
        TARGET="arm64"
    elif opkg print-architecture | grep -q "arm_"; then
        TARGET="arm"
    elif opkg print-architecture | grep -q "mips_"; then
        TARGET="mips"
    elif opkg print-architecture | grep -q "mipsel_"; then
        TARGET="mipsle"
    elif opkg print-architecture | grep -q "x86_64"; then
        TARGET="amd64"
    else
        echo "❌ Error: Could not determine compatible package for '$ARCH'."
        exit 1
    fi
    ;;
esac

echo "Selected Target: $TARGET"

# 2. Download Files
echo "⬇️  Downloading packages..."
cd /tmp

# Remove old files if they exist
rm -f lucicodex.ipk luci-app-lucicodex.ipk

if ! curl -L -o lucicodex.ipk "$BASE_URL/lucicodex-$TARGET.ipk"; then
    echo "❌ Error: Failed to download core package."
    exit 1
fi

if ! curl -L -o luci-app-lucicodex.ipk "$BASE_URL/luci-app-lucicodex.ipk"; then
    echo "❌ Error: Failed to download LuCI app package."
    exit 1
fi

# 3. Install
echo "📦 Installing..."
if opkg install lucicodex.ipk luci-app-lucicodex.ipk; then
    echo ""
    echo "✅ Success! LuciCodex installed."
    echo "   Go to System -> LuciCodex in your router's web interface."
    echo "   Don't forget to configure your API key!"
    
    # Clean up
    rm -f lucicodex.ipk luci-app-lucicodex.ipk
else
    echo "❌ Error: Installation failed."
    echo "   Try running 'opkg update' first and run this script again."
    exit 1
fi
