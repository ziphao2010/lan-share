#!/usr/bin/env bash
set -euo pipefail
VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo dev)
mkdir -p dist
for p in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
  os=${p%/*}; arch=${p#*/}
  ext=""; [ "$os" = "windows" ] && ext=".exe"
  name="lan-share-$os-$arch$ext"
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o "dist/$name" .
  echo "built dist/$name (v$VERSION)"
done
echo Done.
