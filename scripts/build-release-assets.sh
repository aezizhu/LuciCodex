#!/bin/bash
set -euo pipefail

# Build multi-arch binaries and package .ipk for lucicodex and luci-app-lucicodex
# Requires: go, tar, ar (binutils)

VERSION=${VERSION:-"0.4.0"}
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
  if [[ "$arch" == "arm" ]]; then
    GOOS=linux GOARCH=arm GOARM=${GOARM:-$GOARM_DEFAULT} \
      go build -trimpath -ldflags "-s -w" -o "$OUT/lucicodex-linux-${arch}v${GOARM:-$GOARM_DEFAULT}" ./cmd/lucicodex
    outbin="$OUT/lucicodex-linux-${arch}v${GOARM:-$GOARM_DEFAULT}"
  else
    GOOS=linux GOARCH="$arch" \
      go build -trimpath -ldflags "-s -w" -o "$OUT/lucicodex-linux-${arch}" ./cmd/lucicodex
    outbin="$OUT/lucicodex-linux-${arch}"
  fi
  echo "$outbin"
}

ipk_pack_lucicodex() {
  local arch="$1"; shift
  local binpath="$1"; shift
  local arch_ipk="$arch"
  case "$arch" in
    amd64) arch_ipk="x86_64";;
    arm64) arch_ipk="aarch64";;
    arm) arch_ipk="arm_cortex-a7";;
    mipsle) arch_ipk="mipsel_24kc";;
    mips) arch_ipk="mips_24kc";;
  esac
  local work
  work=$(mktemp -d)
  local outdir
  outdir=$(readlink -f "$OUT")
  mkdir -p "$work/control" "$work/data/usr/bin"
  install -m0755 "$binpath" "$work/data/usr/bin/lucicodex"
  cat > "$work/control/control" <<EOF
Package: lucicodex
Version: $VERSION
Architecture: $arch_ipk
Maintainer: aezizhu
Section: utils
Priority: optional
Depends: libc
Description: LuciCodex - Natural-language CLI for OpenWrt
EOF
  # Build standard OpenWrt IPK (tar.gz format)
  # OpenWrt opkg requires the outer container to be a tar.gz, NOT an ar archive
  # It also requires specific file order: ./debian-binary ./data.tar.gz ./control.tar.gz
  (cd "$work"; 
   echo 2.0 > debian-binary; 
   $TAR_CMD --numeric-owner --owner=0 --group=0 -czf control.tar.gz -C control .; 
   $TAR_CMD --numeric-owner --owner=0 --group=0 -czf data.tar.gz -C data .; 
   $TAR_CMD --numeric-owner --owner=0 --group=0 -czf "$outdir/lucicodex_${VERSION}_${arch_ipk}.ipk" ./debian-binary ./data.tar.gz ./control.tar.gz
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
  install -m0644 package/luci-app-lucicodex/luasrc/view/lucicodex/overview.htm "$work/data/usr/lib/lua/luci/view/lucicodex/overview.htm"
  install -m0644 package/luci-app-lucicodex/luasrc/view/lucicodex/run.htm "$work/data/usr/lib/lua/luci/view/lucicodex/run.htm"
  install -m0644 package/luci-app-lucicodex/luasrc/view/cbi/lucicodex.htm "$work/data/usr/lib/lua/luci/view/cbi/lucicodex.htm" 2>/dev/null || true
  install -m0644 package/luci-app-lucicodex/luasrc/view/lucicodex/modern_run.htm "$work/data/usr/lib/lua/luci/view/lucicodex/modern_run.htm" 2>/dev/null || true
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
  # Build standard OpenWrt IPK (tar.gz format)
  (cd "$work"; 
   echo 2.0 > debian-binary; 
   $TAR_CMD --numeric-owner --owner=0 --group=0 -czf control.tar.gz -C control .; 
   $TAR_CMD --numeric-owner --owner=0 --group=0 -czf data.tar.gz -C data .; 
   $TAR_CMD --numeric-owner --owner=0 --group=0 -czf "$outdir/luci-app-lucicodex_${VERSION}_all.ipk" ./debian-binary ./data.tar.gz ./control.tar.gz
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


