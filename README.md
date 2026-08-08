# lan-share — 内网直链分享工具

一键把本地文件/目录变成局域网 HTTP 直链，服务器用 `curl`/`wget` 直接拉取，走满内网带宽。

## 特性

- HTTP 直链，全平台浏览器/工具可下载
- Range 断点续传（`curl -C -` / `wget -c`）
- sendfile 零拷贝高吞吐
- 目录浏览 + seekable 流式 ZIP 打包（`?zip=1`，支持 Range 断点续传）
- 可选 HMAC token 鉴权 + 过期时间
- 单二进制，零依赖

## 安装

### 从源码构建

需要 Go 1.21+：

```bash
go build -o lan-share .
```

### 交叉编译（全平台）

```bash
# Linux/macOS
./build.sh

# Windows
.\build.ps1
```

产物在 `dist/` 目录。

## 使用

```bash
# 分享当前目录
lan-share

# 分享单个文件
lan-share ./bigfile.iso

# 分享目录（带 zip 下载）
lan-share ./dataset/

# 自定义端口、过期时间、鉴权
lan-share -p 9999 -t 2h -k mysecret ./large/
```

### 参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-p` | 8080 | 监听端口（忙时自动递增） |
| `-t` | 永久 | 过期时间（如 `2h`, `30m`, `7d`） |
| `-k` | 无 | 访问密钥，生成带 token 的直链 |
| `-zip` | false | 目录直接以 ZIP 归档响应 |
| `-ip` | 自动检测 | 指定广播的 IP 地址 |
| `-bind` | 0.0.0.0 | 监听地址 |
| `-q` | false | 安静模式，仅输出链接 |

### 服务器端拉取

```bash
curl -O http://192.168.1.5:8080/bigfile.iso
curl -C - -O http://192.168.1.5:8080/bigfile.iso     # 断点续传
wget -c http://192.168.1.5:8080/bigfile.iso
```

## 项目结构

```
.
├── .github/workflows/ci.yml   # CI/CD
├── pkg/
│   ├── ipaddr/                # LAN IP 发现
│   ├── server/                # HTTP 服务核心（含 seekable ZIP）
│   └── token/                 # HMAC-SHA256 令牌
├── main.go                    # CLI 入口
├── build.ps1 / build.sh       # 交叉编译脚本
└── README.md
```

## 安全提醒

仅限互信的内网使用；不要暴露到公网。敏感环境请启用 `-k` 令牌。

## License

MIT
