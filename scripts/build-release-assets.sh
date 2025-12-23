#!/bin/bash
set -euo pipefail

# Build multi-arch binaries and package .ipk for lucicodex and luci-app-lucicodex
# Requires: go, tar, ar (binutils)

VERSION=${VERSION:-"0.6.16"}
OUT=${OUT:-"dist"}
ARCHES=(amd64 arm64 arm mipsle mips)
GOARM_DEFAULT=7

# Detect OS and set appropriate tools
OS="$(uname -s)"
if [[ "$OS" == "Darwin" ]]; then
  if command -v gtar >/dev/null 2>&1; then
    TAR_CMD="gtar"
  else
    echo "Warning: GNU tar not found. Install with: brew install gnu-tar" >&2
    echo "Falling back to BSD tar - IPK packages may not work on OpenWrt" >&2
    TAR_CMD="tar"
  fi
  SHASUM_CMD="shasum -a 256"
else
  TAR_CMD="tar"
  SHASUM_CMD="sha256sum"
fi

mkdir -p "$OUT"

build_bin() {
  local arch="$1"
  local outbin
  # CGO_ENABLED=0 for static linking (smaller, more portable)
  # -s -w strips debug symbols
  # -trimpath removes build paths
  if [[ "$arch" == "arm" ]]; then
    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=${GOARM:-$GOARM_DEFAULT} \
      go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o "$OUT/lucicodex-linux-${arch}v${GOARM:-$GOARM_DEFAULT}" ./cmd/lucicodex
    outbin="$OUT/lucicodex-linux-${arch}v${GOARM:-$GOARM_DEFAULT}"
  else
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" \
      go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o "$OUT/lucicodex-linux-${arch}" ./cmd/lucicodex
    outbin="$OUT/lucicodex-linux-${arch}"
  fi
  echo "$outbin"
}

ipk_pack_lucicodex() {
  local arch="$1"; shift
  local binpath="$1"; shift
  
  # Determine architecture name for filename to match release.yml expectations
  # release.yml expects: mips_24kc, arm_cortex-a7, x86_64, aarch64, mipsel_24kc
  local arch_filename=""
  local arch_match=""
  
  case "$arch" in
    amd64) 
      arch_filename="x86_64"
      arch_match="x86_64"
      ;;
    arm64) 
      arch_filename="aarch64"
      arch_match="aarch64"
      ;;
    arm) 
      arch_filename="arm_cortex-a7"
      arch_match="arm"
      ;;
    mipsle) 
      arch_filename="mipsel_24kc"
      arch_match="mips"
      ;;
    mips) 
      arch_filename="mips_24kc"
      arch_match="mips"
      ;;
  esac

  local work
  work=$(mktemp -d)
  local outdir
  outdir=$(readlink -f "$OUT")
  mkdir -p "$work/control" "$work/data/usr/bin"
  install -m0755 "$binpath" "$work/data/usr/bin/lucicodex"
  
  # Generate control file with Architecture: all
  cat > "$work/control/control" <<EOF
Package: lucicodex
Version: $VERSION
Architecture: all
Maintainer: aezizhu
Section: utils
Priority: optional
Depends: libc
Description: LuciCodex - Natural-language CLI for OpenWrt
EOF

  # Generate preinst script to verify architecture
  cat > "$work/control/preinst" <<EOF
#!/bin/sh
# Universal IPK pre-install check
# Verifies that the actual CPU architecture matches the binary
ARCH_MATCH="$arch_match"
CURRENT_ARCH=\$(uname -m)

if echo "\$CURRENT_ARCH" | grep -q "\$ARCH_MATCH"; then
    exit 0
else
    echo "Error: CPU architecture mismatch."
    echo "This package contains a binary for: \$ARCH_MATCH"
    echo "Your router's CPU architecture is: \$CURRENT_ARCH"
    echo "Please download the correct package for your router."
    exit 1
fi
EOF
  chmod 0755 "$work/control/preinst"

  # Build standard OpenWrt IPK (tar.gz format for outer container)
  # CRITICAL: 
  # 1. Inner tarballs MUST use --numeric-owner --owner=0 --group=0
  # 2. Use tar.gz for outer container (more portable than ar - Linux ar adds trailing slashes)
  # 3. File order MUST be: debian-binary data.tar.gz control.tar.gz (or similar, but debian-binary first)
  # 4. Do NOT use ./ prefix in tar inputs to avoid path issues
  (cd "$work"; 
   echo 2.0 > debian-binary; 
   $TAR_CMD --numeric-owner --owner=0 --group=0 -czf control.tar.gz -C control .; 
   $TAR_CMD --numeric-owner --owner=0 --group=0 -czf data.tar.gz -C data .; 
   $TAR_CMD --numeric-owner --owner=0 --group=0 -czf "$outdir/lucicodex_${VERSION}_${arch_filename}.ipk" debian-binary data.tar.gz control.tar.gz
  )
  
  rm -rf "$work"
}

ipk_pack_luci() {
  local work
  work=$(mktemp -d)
  local outdir
  outdir=$(readlink -f "$OUT")
  mkdir -p "$work/control" "$work/data/usr/lib/lua/luci/controller" "$work/data/usr/lib/lua/luci/model/cbi" "$work/data/usr/lib/lua/luci/view/lucicodex" "$work/data/usr/lib/lua/luci/view/cbi"
  install -m0644 package/luci-app-lucicodex/luasrc/controller/lucicodex.lua "$work/data/usr/lib/lua/luci/controller/lucicodex.lua"
  install -m0644 package/luci-app-lucicodex/luasrc/model/cbi/lucicodex.lua "$work/data/usr/lib/lua/luci/model/cbi/lucicodex.lua"
  install -m0644 package/luci-app-lucicodex/luasrc/view/lucicodex/home.htm "$work/data/usr/lib/lua/luci/view/lucicodex/home.htm"
  install -m0644 package/luci-app-lucicodex/luasrc/view/lucicodex/run.htm "$work/data/usr/lib/lua/luci/view/lucicodex/run.htm"
  install -m0644 package/luci-app-lucicodex/luasrc/view/lucicodex/config.htm "$work/data/usr/lib/lua/luci/view/lucicodex/config.htm"
  install -m0644 package/luci-app-lucicodex/luasrc/view/cbi/lucicodex.htm "$work/data/usr/lib/lua/luci/view/cbi/lucicodex.htm" 2>/dev/null || true
  
  # Copy root directory contents (init scripts, ACLs, uci-defaults)
  if [ -d "package/luci-app-lucicodex/root" ]; then
    cp -r package/luci-app-lucicodex/root/* "$work/data/"
    # Ensure init script is executable
    if [ -f "$work/data/etc/init.d/lucicodex" ]; then
      chmod 0755 "$work/data/etc/init.d/lucicodex"
    fi
  fi
  cat > "$work/control/control" <<EOF
Package: luci-app-lucicodex
Version: $VERSION
Architecture: all
Maintainer: aezizhu
Section: luci
Priority: optional
Depends: luci-base, lucicodex
Description: LuCI web UI for LuciCodex
EOF
  # Build standard OpenWrt IPK (tar.gz format for outer container)
  (cd "$work"; 
   echo 2.0 > debian-binary; 
   $TAR_CMD --numeric-owner --owner=0 --group=0 -czf control.tar.gz -C control .; 
   $TAR_CMD --numeric-owner --owner=0 --group=0 -czf data.tar.gz -C data .; 
   $TAR_CMD --numeric-owner --owner=0 --group=0 -czf "$outdir/luci-app-lucicodex_${VERSION}_all.ipk" debian-binary data.tar.gz control.tar.gz
  )
  
  rm -rf "$work"
}

sha256sum_all() {
  (cd "$OUT" && $SHASUM_CMD * > SHA256SUMS)
}

main() {
  for arch in "${ARCHES[@]}"; do
    bin=$(build_bin "$arch")
    ipk_pack_lucicodex "$arch" "$bin"
  done
  ipk_pack_luci
  sha256sum_all
}

main "$@"


