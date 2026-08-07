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
Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue

# Windows 图形版启动器（仅本机 Windows 上可用）
if ($env:OS -eq "Windows_NT") {
  $env:CGO_ENABLED = "0"
  go build -trimpath -ldflags="-s -w -H=windowsgui" -o "dist/lan-share-gui.exe" ./cmd/gui
  if ($?) { Write-Host "built dist/lan-share-gui.exe" }
}
Write-Host "Done."
