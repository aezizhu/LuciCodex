#!/bin/sh

# LuciCodex Installer
# Usage: wget -qO- https://raw.githubusercontent.com/aezizhu/LuciCodex/main/install.sh | sh

VERSION="1.0.0"
REPO="aezizhu/LuciCodex"

echo "========================================"
echo "      LuciCodex Installer v${VERSION}"
echo "========================================"

# Check for opkg
if ! command -v opkg >/dev/null 2>&1; then
    echo "Error: opkg not found. Is this OpenWrt?"
    exit 1
fi

echo "[1/5] Updating package lists..."
opkg update >/dev/null 2>&1

echo "[2/5] Installing dependencies..."
opkg install luci-base luci-compat ca-bundle curl >/dev/null 2>&1

echo "[3/5] Detecting architecture..."
cd /tmp

# Detect architecture from opkg - use exact OpenWrt arch names
ARCH=""
for arch in aarch64 arm_cortex-a7 arm_cortex-a9 mips_24kc mipsel_24kc x86_64; do
    if opkg print-architecture | grep -q "$arch"; then
        ARCH="$arch"
        break
    fi
done

# Fallback to uname if opkg detection fails
if [ -z "$ARCH" ]; then
    UARCH=$(uname -m)
    case "$UARCH" in
        x86_64) ARCH="x86_64" ;;
        aarch64) ARCH="aarch64" ;;
        armv7*) ARCH="arm_cortex-a7" ;;
        mips) ARCH="mips_24kc" ;;
        mipsel) ARCH="mipsel_24kc" ;;
        *)
            echo "Error: Unsupported architecture: $UARCH"
            echo "Supported: aarch64, arm_cortex-a7, mips_24kc, mipsel_24kc, x86_64"
            exit 1
            ;;
    esac
fi

echo "    Architecture: $ARCH"

echo "[4/5] Downloading and installing packages..."

# Package URLs (new naming format)
BINARY_URL="https://github.com/${REPO}/releases/download/v${VERSION}/lucicodex_${VERSION}_${ARCH}.ipk"
LUCI_URL="https://github.com/${REPO}/releases/download/v${VERSION}/luci-app-lucicodex_${VERSION}_all.ipk"

# Download and install binary package
echo "    Downloading lucicodex binary..."
if wget -q -O lucicodex.ipk "$BINARY_URL"; then
    opkg install --force-reinstall lucicodex.ipk 2>/dev/null
    rm -f lucicodex.ipk
else
    echo "Error: Failed to download binary package"
    echo "URL: $BINARY_URL"
    echo ""
    echo "Available architectures:"
    echo "  - aarch64 (ARM64 routers like GL-BE3600)"
    echo "  - arm_cortex-a7 (ARMv7 routers)"
    echo "  - mips_24kc (MIPS routers)"
    echo "  - mipsel_24kc (MIPS Little Endian)"
    echo "  - x86_64 (x86 systems)"
    exit 1
fi

# Download and install LuCI app
echo "    Downloading LuCI interface..."
if wget -q -O luci-app-lucicodex.ipk "$LUCI_URL"; then
    opkg install --force-reinstall luci-app-lucicodex.ipk 2>/dev/null
    rm -f luci-app-lucicodex.ipk
else
    echo "Error: Failed to download LuCI package"
    echo "URL: $LUCI_URL"
    exit 1
fi

echo "[5/5] Configuring system..."

# Increase uhttpd timeouts for LLM API calls
uci set uhttpd.main.script_timeout='300'
uci set uhttpd.main.network_timeout='300'
uci commit uhttpd

# CRITICAL: Clear ALL LuCI caches to ensure new controller routes are registered
rm -rf /tmp/luci-modulecache/
rm -rf /tmp/luci-indexcache*
rm -rf /tmp/luci-sessions/

/etc/init.d/uhttpd restart >/dev/null 2>&1

# Enable and start daemon
if [ -f /etc/init.d/lucicodex ]; then
    /etc/init.d/lucicodex enable
    /etc/init.d/lucicodex restart
fi

echo ""
echo "========================================"
echo "      Installation Complete!"
echo "========================================"
echo ""
echo "Next steps:"
echo "  1. Open LuCI web interface"
echo "  2. Go to System -> LuciCodex"
echo "  3. Add your API key (Gemini/OpenAI/Anthropic)"
echo "  4. Start chatting with your router!"
echo ""
echo "Features in v${VERSION}:"
echo "  - Collapsible AI responses for better readability"
echo "  - Improved WebSocket handling (auto-fallback to HTTP)"
echo "  - Better error messages for proxy timeouts"
echo ""
