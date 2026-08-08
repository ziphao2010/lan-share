# 内网直链分享工具 (lan-share) — 设计文档

日期：2026-08-07
状态：已确认

## 1. 背景与目标

用户需要往服务器传大文件（几百 MB 到几十 GB 不等），常规传输方式太慢。需要一个
"加速通道"：在本地电脑跑一个临时 HTTP 服务，生成内网直链，服务器上用 `curl`/`wget`
直接拉取。本质是**临时分享**工具，用完即关。

**核心目标：**
- 一条命令把文件/目录变成可下载的直链
- 局域网内任意设备（服务器、手机、其他电脑）可直接下载
- 支持大文件：断点续传（Range）、零拷贝、高吞吐
- 跨平台单二进制，零运行时依赖，`scp` 即用
- 可选鉴权与过期时间，避免误暴露

**非目标（YAGNI）：**
- 不做 Web 可视化管理界面（首版）
- 不做多用户/权限体系
- 不做 NAT 穿透/公网传输（那是另一个产品）
- 不做上传功能（首版只做单向分享）

## 2. 方案选型

| 方案 | 结论 |
|------|------|
| 单文件 HTTP server + 增强（mmap/sendfile/Range/token） | **采用** |
| Web UI 传输站 | 否决（首版过重） |
| P2P 加速器（croc 式） | 否决（需要信令服务，超出局域网场景） |

**技术栈：Go 标准库 `net/http`**，理由：
- 单二进制交叉编译（`CGO_ENABLED=0`），Windows/macOS/Linux 全覆盖
- 标准库自带 HTTP、Range 处理、文件服务，无第三方依赖
- `io.Copy` 自动走 sendfile 零拷贝路径
- 性能足够跑满千兆/万兆内网

## 3. 架构设计

```
CLI 入口 (main.go)
    ├── 命令解析：share / serve / version
    │
    ├── pkg/ipaddr        — 局域网 IP 发现（打印给用户）
    ├── pkg/token         — 直链 token 生成、验证、过期
    ├── pkg/server        — HTTP 服务核心
    │     ├── server.go   — 启动、端口管理、优雅退出
    │     ├── handler.go  — 文件/目录请求处理、Range
    │     ├── zip.go      — 目录流式打包下载
    │     └── ui.go       — 目录列表页（HTML，嵌入二进制）
    └── pkg/cli           — 参数解析、输出格式
```

## 4. CLI 设计

```bash
# 分享当前目录（打印所有网卡的 URL）
lan-share ./bigfile.iso
lan-share ./dataset/

# 自定义端口 / 过期时间 / token 保护
lan-share -p 9999 -t 2h -k mysecret ./large/

# 目录以 zip 打包下载
lan-share --zip ./project/

# 不带参数则分享当前工作目录
lan-share
```

**输出示例：**
```
分享中： D:\data\bigfile.iso (4.2 GB)
直链：   http://192.168.1.5:8080/bigfile.iso
         http://192.168.1.100:8080/bigfile.iso
加密：   (可选) 令牌 abc123...，2 小时后过期
Ctrl+C 停止服务
```

**参数表：**

| 参数 | 默认 | 说明 |
|------|------|------|
| `-p, --port` | 8080 | 监听端口，占用则自动 +1 |
| `-t, --expire` | 无 | 过期时长（`2h`、`30m`、`7d`），过期后 403 |
| `-k, --token` | 无 | 访问密钥，用于生成 URL 中的 `?token=`（输入的是密钥而非 token 本身） |
| `--zip` | false | 目录以 zip 包下载（`/dir/?zip=1`） |
| `--ip` | 自动 | 指定对外 IP（多网卡时手动指定） |
| `--bind` | 0.0.0.0 | 监听地址 |
| `-q, --quiet` | false | 只输出 URL，不输出说明 |

## 5. HTTP 协议设计

### 5.1 文件下载（核心）

- 响应头：`Content-Length`、`Content-Type`（按扩展名推断）、`Accept-Ranges: bytes`
- **Range 支持**：`206 Partial Content`，服务端用 `http.ServeContent` 处理
  - 支持 `Range: bytes=start-end`、`bytes=start-`、多段（自动退化为首段）
  - 配合 `io.Copy` 底层走 `sendfile` 零拷贝
- 断点续传验证：`curl -C -` 或 `wget -c` 可直接续传
- 附加响应头：`X-Lan-Share: true`

### 5.2 目录列表

- `GET /dir/` → HTML 列表页（嵌入模板），显示文件名/大小/修改时间
- `GET /dir/?zip=1` 或 `--zip` 模式 → 流式 zip 打包
  - 用 `archive/zip` 流式写入（边遍历边压缩，不占内存）
  - `Content-Type: application/zip`，支持 Range 前的完整包下载
  - 大目录 zip 打包时禁用 Range（流式无法 seek）

### 5.3 鉴权（可选）

- `-k secret` 时，直链格式：`http://ip:port/file?token=<hmac>&exp=<unix>`
- `token = HMAC-SHA256(secret, path + exp)`，hex 编码
- 服务端验证：exp 未过期 + token 匹配 → 200，否则 403
- 无 `-k` 时是开放模式，仅打印 URL 不鉴权

### 5.4 错误处理

| 场景 | 响应 |
|------|------|
| 文件/目录不存在 | 404 |
| token 缺失/错误/过期 | 403 |
| 路径穿越（`..`） | 400（统一走 `filepath.Clean` + 前缀校验） |
| 端口占用 | 自动尝试 +1（最多 10 次），全失败则报错退出 |
| 下载中断 | 客户端断连，服务端停止写（`Context` 取消），无资源泄漏 |

## 6. 大文件性能设计

1. **零拷贝**：`http.ServeContent` + `io.Copy` → Linux 自动 `sendfile`，Windows 走 `TransmitFile` 等价路径，避免用户态拷贝
2. **内存占用**：固定 32KB buffer 流式传输，不整文件载入内存；zip 同样流式
3. **并发下载**：`net/http` 自带 goroutine 并发，多连接同时下载无需额外处理
4. **千兆/万兆跑满**：Go 标准库 HTTP 足够；若不够，后续可加 `-threads` 分片下载客户端（非首版）

## 7. 安全性

- **路径穿越防护**：所有路径 `filepath.Clean` 后必须落在分享根目录内，否则 400
- **token 防暴露**：仅提示用户复制完整 URL；token 不写入日志
- **过期即失效**：exp 用 unix 时间戳，服务端严格校验
- **不绑定公网**：默认 `--bind 0.0.0.0` 但文档强调"仅限内网使用，勿暴露公网"；如被公网访问，token 保护可选兜底
- **无目录遍历**：默认只读分享，不做写操作（首版）

## 8. 测试策略

| 层级 | 内容 |
|------|------|
| 单元测试 (go test) | token 生成/验证/过期；Range 解析边界；路径穿越拦截；zip 打包正确性 |
| 集成测试 (httptest) | 启动服务 → 下载文件字节一致；Range 分段下载拼接一致；404/403 场景 |
| 手动验证 (文档记录) | `curl -O`、`curl -C -` 断点续传、`wget -c`、浏览器打开目录页、手机扫码下载 |
| 大文件验证 | 生成 1GB / 10GB 稀疏文件，对比 sha256，测带宽（iperf 参考值） |

## 9. 交付物

1. 源码仓库（Go module：`lan-share`）
2. 交叉编译脚本：`build.sh` / `build.ps1`（Linux amd64/arm64、macOS、Windows）
3. README：用法、示例、断点续传验证方法
4. 单二进制产物：`lan-share`（~5-8MB）

## 10. 里程碑

| 阶段 | 内容 |
|------|------|
| M1 | CLI 骨架 + 文件直链下载 + Range/断点续传 + 多网卡 IP 打印 |
| M2 | 目录列表页 + zip 流式打包 |
| M3 | token 鉴权 + 过期 + 路径安全 |
| M4 | 交叉编译脚本 + README + 手动验证手册 |

M1 完成即满足核心需求，可先投入使用；M2-M4 逐步补强。
