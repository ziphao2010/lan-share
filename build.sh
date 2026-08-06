#!/usr/bin/env bash
set -euo pipefail
mkdir -p dist
for p in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
  os=${p%/*}; arch=${p#*/}
  ext=""; [ "$os" = "windows" ] && ext=".exe"
  name="lan-share-$os-$arch$ext"
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags="-s -w" -o "dist/$name" .
  echo "built dist/$name"
done
echo Done.
