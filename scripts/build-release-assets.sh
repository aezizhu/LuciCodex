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
  local legacy_bin
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

# Map architecture to GL-iNet architecture identifiers
get_glinet_arch() {
  local arch="$1"
  case "$arch" in
    amd64) echo "x86_64";;
    arm64) echo "aarch64_cortex-a53";; # GL-iNet specific!
    arm) echo "arm_cortex-a7";;
    mipsle) echo "mipsel_24kc";;
    mips) echo "mips_24kc";;
    *) echo "$arch";;
  esac
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
  # Build standard OpenWrt IPK (ar format)
  (cd "$work"; echo 2.0 > debian-binary; tar -czf control.tar.gz -C control .; tar -czf data.tar.gz -C data .; ar -r "$outdir/lucicodex_${VERSION}_${arch_ipk}.ipk" debian-binary control.tar.gz data.tar.gz >/dev/null)
  
  # Build GL-iNet IPK (gzip tar format) - only for arm64 (GL-iNet routers)
  if [ "$arch" = "arm64" ]; then
    local glinet_arch=$(get_glinet_arch "$arch")
    local glinet_work=$(mktemp -d)
    mkdir -p "$glinet_work/control" "$glinet_work/data/usr/bin"
    install -m0755 "$binpath" "$glinet_work/data/usr/bin/lucicodex"
    cat > "$glinet_work/control/control" <<EOF
Package: lucicodex
Version: $VERSION
Architecture: $glinet_arch
Maintainer: aezizhu
Section: utils
Priority: optional
Depends: libc
Description: LuciCodex - Natural-language CLI for OpenWrt
EOF
    # Build GL-iNet format IPK (gzip tar, not ar)
    (cd "$glinet_work" && \
      echo "2.0" > debian-binary && \
      $TAR_CMD --numeric-owner --owner=0 --group=0 --mtime='1970-01-01 00:00:00' -czf control.tar.gz -C control . && \
      $TAR_CMD --numeric-owner --owner=0 --group=0 --mtime='1970-01-01 00:00:00' -czf data.tar.gz -C data . && \
      $TAR_CMD --numeric-owner --owner=0 --group=0 --mtime='1970-01-01 00:00:00' -czf "$outdir/lucicodex_${VERSION}_${glinet_arch}.ipk" debian-binary control.tar.gz data.tar.gz)
    rm -rf "$glinet_work"
  fi
  
  rm -rf "$work"
}

ipk_pack_luci() {
  local work
  work=$(mktemp -d)
  local outdir
  outdir=$(readlink -f "$OUT")
  mkdir -p "$work/control" "$work/data/usr/lib/lua/luci/controller" "$work/data/usr/lib/lua/luci/view/lucicodex"
  install -m0644 package/luci-app-lucicodex/luasrc/controller/lucicodex.lua "$work/data/usr/lib/lua/luci/controller/lucicodex.lua"
  install -m0644 package/luci-app-lucicodex/luasrc/view/lucicodex/overview.htm "$work/data/usr/lib/lua/luci/view/lucicodex/overview.htm"
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
  # Build standard OpenWrt IPK (ar format)
  (cd "$work"; echo 2.0 > debian-binary; tar -czf control.tar.gz -C control .; tar -czf data.tar.gz -C data .; ar -r "$outdir/luci-app-lucicodex_${VERSION}_all.ipk" debian-binary control.tar.gz data.tar.gz >/dev/null)
  
  # Build GL-iNet IPK (gzip tar format) for LuCI app
  local glinet_work=$(mktemp -d)
  mkdir -p "$glinet_work/control" "$glinet_work/data/usr/lib/lua/luci/controller" "$glinet_work/data/usr/lib/lua/luci/view/lucicodex"
  install -m0644 package/luci-app-lucicodex/luasrc/controller/lucicodex.lua "$glinet_work/data/usr/lib/lua/luci/controller/lucicodex.lua"
  install -m0644 package/luci-app-lucicodex/luasrc/view/lucicodex/overview.htm "$glinet_work/data/usr/lib/lua/luci/view/lucicodex/overview.htm"
  cat > "$glinet_work/control/control" <<EOF
Package: luci-app-lucicodex
Version: $VERSION
Architecture: all
Maintainer: aezizhu
Section: luci
Priority: optional
Depends: luci-base, lucicodex
Description: LuCI web UI for LuciCodex
EOF
  # Build GL-iNet format IPK (gzip tar, not ar)
  (cd "$glinet_work" && \
    echo "2.0" > debian-binary && \
    $TAR_CMD --numeric-owner --owner=0 --group=0 --mtime='1970-01-01 00:00:00' -czf control.tar.gz -C control . && \
    $TAR_CMD --numeric-owner --owner=0 --group=0 --mtime='1970-01-01 00:00:00' -czf data.tar.gz -C data . && \
    $TAR_CMD --numeric-owner --owner=0 --group=0 --mtime='1970-01-01 00:00:00' -czf "$outdir/luci-app-lucicodex_${VERSION}_all_glinet.ipk" debian-binary control.tar.gz data.tar.gz)
  rm -rf "$glinet_work"
  
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


