#!/usr/bin/env bash
# Builds the release archives for every supported target, with the version
# stamped into each binary, plus a SHA256SUMS file covering them.
#
#   scripts/build-release.sh <version> <out-dir>
#
# <version> is the tag, e.g. v0.1.0; `release --version` prints it. Archives
# are named release_<version-without-v>_<os>_<arch>.tar.gz (.zip on windows)
# and hold the binary alone. The release workflow publishes them; the cli
# workflow runs this on every pull request so a target that stops building
# fails there rather than at release time.
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <version> <out-dir>" >&2
  exit 2
fi
version="$1"
out="$2"
bare="${version#v}"

# The supported targets. No cgo: each binary is a single static file.
targets="
linux/amd64 linux/arm64
darwin/amd64 darwin/arm64
windows/amd64 windows/arm64
"

root="$(cd "$(dirname "$0")/.." && pwd)"
mkdir -p "$out"
out="$(cd "$out" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

for target in $targets; do
  os="${target%/*}"
  arch="${target#*/}"
  exe=release
  [ "$os" = windows ] && exe=release.exe
  name="release_${bare}_${os}_${arch}"
  echo "==> $target"
  mkdir -p "$work/$name"
  (cd "$root" && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
    -ldflags="-s -w -X github.com/brettcallaghan/release/internal/buildinfo.version=$version" \
    -o "$work/$name/$exe" ./cmd/release)
  if [ "$os" = windows ]; then
    (cd "$work/$name" && zip -q -X "$out/$name.zip" "$exe")
  else
    tar -C "$work/$name" -czf "$out/$name.tar.gz" "$exe"
  fi
done

(cd "$out" && sha256sum release_* > SHA256SUMS)
cat "$out/SHA256SUMS"
