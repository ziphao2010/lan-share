# 交叉编译所有平台（版本从 git tag 自动注入）
$ErrorActionPreference = "Stop"
$version = if (git describe --tags --abbrev=0 2>$null) { git describe --tags --abbrev=0 } else { "dev" }
$targets = @(
  @{os="linux";  arch="amd64"; ext=""},
  @{os="linux";  arch="arm64"; ext=""},
  @{os="darwin"; arch="amd64"; ext=""},
  @{os="darwin"; arch="arm64"; ext=""},
  @{os="windows"; arch="amd64"; ext=".exe"}
)
foreach ($t in $targets) {
  $env:CGO_ENABLED = "0"
  $env:GOOS = $t.os
  $env:GOARCH = $t.arch
  $name = "lan-share-$($t.os)-$($t.arch)$($t.ext)"
  go build -trimpath -ldflags="-s -w -X main.version=$version" -o "dist/$name" .
  Write-Host "built dist/$name ($version)"
}
Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
Write-Host "Done."
