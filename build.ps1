# 交叉编译所有平台
$ErrorActionPreference = "Stop"
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
  go build -trimpath -ldflags="-s -w" -o "dist/$name" .
  Write-Host "built dist/$name"
}
Remove-Item Env:GOOS
Remove-Item Env:GOARCH
Write-Host "Done."
