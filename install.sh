#!/bin/sh

# LuciCodex Installer
# Usage: wget -O - https://raw.githubusercontent.com/aezizhu/LuciCodex/main/install.sh | sh

VERSION="v0.4.65"
REPO="aezizhu/LuciCodex"
URL="https://github.com/${REPO}/releases/download/${VERSION}/luci-app-lucicodex.ipk"

echo "========================================"
echo "      LuciCodex Installer ${VERSION}"
echo "========================================"

# Check for opkg
if ! command -v opkg >/dev/null 2>&1; then
    echo "Error: opkg not found. Is this OpenWrt?"
    exit 1
fi

echo "[1/4] Updating package lists..."
opkg update >/dev/null 2>&1

echo "[2/4] Installing dependencies..."
opkg install luci-base luci-compat ca-bundle curl >/dev/null 2>&1

echo "[3/4] Downloading and installing LuciCodex..."
cd /tmp
if wget -O luci-app-lucicodex.ipk "$URL"; then
    opkg install luci-app-lucicodex.ipk
    rm -f luci-app-lucicodex.ipk
    rm -rf /tmp/luci-modulecache/
else
    echo "Error: Failed to download package from $URL"
    exit 1
fi

echo "[4/4] Optimizing system configuration..."
# Increase uhttpd timeouts to prevent 502 errors
uci set uhttpd.main.script_timeout='300'
uci set uhttpd.main.network_timeout='300'
uci commit uhttpd
service uhttpd restart

echo "[5/4] Starting LuciCodex Daemon..."
if [ -f /etc/init.d/lucicodex ]; then
    /etc/init.d/lucicodex enable
    /etc/init.d/lucicodex restart
fi

echo "========================================"
echo "      Installation Complete!"
echo "========================================"
echo "1. Refresh your LuCI web interface."
echo "2. Go to System > LuciCodex."
echo "3. Configure your API key."
echo "========================================"
