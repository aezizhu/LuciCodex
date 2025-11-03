#!/bin/bash
set -euo pipefail

# Build multi-arch binaries and package .ipk for lucicodex and luci-app-lucicodex
# Requires: go, tar, ar (binutils)

VERSION=${VERSION:-"0.3.0"}
OUT=${OUT:-"dist"}
ARCHES=(amd64 arm64 arm mipsle mips)
GOARM_DEFAULT=7

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
  (cd "$work"; echo 2.0 > debian-binary; tar -czf control.tar.gz -C control .; tar -czf data.tar.gz -C data .; ar -r "$outdir/lucicodex_${VERSION}_${arch_ipk}.ipk" debian-binary control.tar.gz data.tar.gz >/dev/null)
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
  (cd "$work"; echo 2.0 > debian-binary; tar -czf control.tar.gz -C control .; tar -czf data.tar.gz -C data .; ar -r "$outdir/luci-app-lucicodex_${VERSION}_all.ipk" debian-binary control.tar.gz data.tar.gz >/dev/null)
  rm -rf "$work"
}

sha256sum_all() {
  (cd "$OUT" && sha256sum * > SHA256SUMS)
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


