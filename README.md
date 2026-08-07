# lan-share — 内网直链分享工具

一键把文件/目录变成局域网直链，服务器用 `curl`/`wget` 直接拉取，走满内网带宽。

## 用法

```bash
lan-share ./bigfile.iso              # 生成直链
lan-share ./dataset/                 # 目录列表 + 一键 zip 下载
lan-share -p 9999 -t 2h -k mysecret ./large/   # 自定义端口/过期/加密
```

**Windows 拖拽用法**：把文件/文件夹直接拖到 `lan-share.exe` 上，即自动开分享并显示直链；关掉窗口即停止。

参数：见 `lan-share -h`。

## 服务器端拉取（示例）

```bash
curl -O http://192.168.1.5:8080/bigfile.iso          # 完整下载
curl -C - -O http://192.168.1.5:8080/bigfile.iso     # 断点续传
wget -c http://192.168.1.5:8080/bigfile.iso
```

## 特性

- HTTP 直链，全平台浏览器/工具可下载
- Range 断点续传（`curl -C -` / `wget -c`）
- sendfile 零拷贝高吞吐
- 目录列表 + seekable 流式 zip 打包（`?zip=1`，支持 Range 断点续传）
- 可选 HMAC token 鉴权 + 过期时间
- 单二进制 ~5MB，`scp` 即用，零依赖

## 构建

在装有 Go 1.21+ 的机器上：

```powershell
# Windows
.\build.ps1
# Linux/macOS
./build.sh
```

产物在 `dist/`。传输到目标机后 `scp`/u 盘拷贝即可。

## 安全提醒

仅限互信的内网使用；不要按公网 IP 绑定暴露。如置于环境敏感处，请启用 `-k` 令牌。
