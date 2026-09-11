# Static Message Protocol (SMP)

## en-uk

Lightweight message protocol with Rust core library, Go server, and Go CLI client. Messages are stored on the server and retrieved via explicit `push` / `pull` operations — no automatic delivery.

### Quick Start

```bash
# 1. Build
.\scripts\build-all.ps1 -Clean

# 2. Start server
./target/smp-server.exe --listen 0.0.0.0:9932 --verbose

# 3. Bootstrap a token. `create` is what mints your token, but it refuses to run
#    unless a token is already present. Seed a well-formed dummy: the token tail is
#    the last 8 hex chars, so anything else blows the head past its 8-byte limit
#    and the server answers -1003 ERR_HEAD_LIMIT.
export SMP_TOKEN=smpt128-00000000000000000000000000000000

# 4. Register a user -> ok:<user>:token=<tok>:permissions=[...]
#    The real token is saved to ~/.smp-token automatically.
./target/smp.exe create smp@127.0.0.1 smp@alice

# Drop the bootstrap value so later commands read ~/.smp-token instead of the
# SMP_TOKEN env var, which takes precedence.
unset SMP_TOKEN

# 5. Push a message. The payload is read from stdin; the message is addressed
#    to smp@<username>. -u remembers the server for later commands.
echo "Hello, SMP!" | ./target/smp.exe push -u smp@127.0.0.1 smp@alice

# 6. Pull messages. Must be the same user the token is bound to.
./target/smp.exe pull smp@127.0.0.1 smp@alice

# 7. Large file hosting (CFM). Upload only -- there is no download subcommand.
./target/smp.exe cfm push smp@127.0.0.1 largefile.zip
```

> **Verified against a live server** in this working tree: `create` ->
> `ok:alice:token=...` (saved to `~/.smp-token`), `push` -> `ok:<msgid>`, `pull`
> returns the message, and `list`, `watch-context` and `cfm push` all return
> normally -- with the server still alive after every one of them. Writing to, or
> pulling from, an inbox the token is not bound to is rejected with `Error 1006`;
> a token tail the server never issued is rejected with `Error 1007`; and a command
> the config does not permit is rejected with `Error 6005`. So the bootstrap token
> above works for `create` only and cannot be reused to read or write anyone else's
> inbox. `watch-context` now prints the ID the server reports for the caller's own
> inbox, and `_cfm_download` returns the uploaded bytes to the uploader while
> rejecting every other known user.
>
> **Open:** the server discards the message `ext` block. The Rust FFI exposes no
> accessor for it (`smp_parser_get_extensions` does not exist), so
> `handleCFMUpload` reads `expire` / `public` out of the *file bytes* instead of out
> of the ext block where `client/internal/cli/cfm.go:100` puts them. The client's
> `public=false` literal is silently ignored and `public=true` is unreachable; fixing
> it needs a new C FFI in `core` plus a rebuild of `smp_core.dll`.

### Command Policy

`conf-mode` with `base-permission` / `special-permission` decides which commands a
user may run. Each pattern is a glob matched against the CLI command form of the
route:

| route | command |
|-------|---------|
| `_pull` | `smp pull` |
| user inbox (`smp@alice`) | `smp push` |
| `_create` | `smp create` |
| `_list` | `smp list` |
| `_watch` | `smp watch-context` |
| `_cfm_upload` | `smp cfm upload` |
| `_cfm_download` | `smp cfm download` |
| `_token_generate` | `smp token generate` |
| `_token_list` | `smp token list` |
| `_token_revoke` | `smp token revoke` |

| `conf-mode` | effect |
|-------------|--------|
| `"whitelist"` | allowed only when some pattern matches |
| `"blacklist"` | allowed only when no pattern matches |
| anything else, or missing | treated as `blacklist` |

A user's effective set is `base-permission` plus `special-permission[<user>]`,
de-duplicated in order, so `admin: [smp *]` grants everything. `_create` is exempt
from both the command policy and the token check: it is the unauthenticated
bootstrap, and gating it would deadlock the first user out of a token.

Example (the shipped `server/config.yml`):

```yaml
conf-mode: "whitelist"
base-permission:
  - smp *pull*
  - smp *push*
special-permission:
  admin:
    - smp *
```

With this, non-admin users can push and pull only; `create` still works because it is
exempt.

### Project Structure

```
project-root/
├── core/          # Rust core library (parsing, assembly, validation, FFI)
├── cfm/           # Rust CFM module (file hosting, FFI)
├── server/        # Go server (TCP, storage, idempotency, routing)
├── client/        # Go CLI client (push, pull, cfm, token)
├── scripts/       # Build scripts (build-all.ps1)
├── docs/          # All documentation (this directory)
│   ├── README.md          # This file (overview + quick start)
│   ├── core-docs.md       # Core library (protocol format, FFI API, state machine)
│   ├── server-docs.md     # Server (architecture, storage, routing)
│   ├── client-docs.md     # Client (command structure, FFI calls)
│   └── cfm-docs.md        # CFM module (file hosting, FFI API)
└── target/        # Build artifacts (smp_core.dll, cfm.dll, smp-server.exe, smp.exe)
```

### Protocol Overview

Messages consist of 5 sequential blocks: `Head → SubHead → Context → User Data → Check (MD5)`

| Block | Content |
|-------|---------|
| **Head** | Token tail (8 hex), User Data length, Message ID (8B, bit63=CFM) |
| **SubHead** | Version `smp/0.1b`, client address, route, Extensions (key=value) |
| **Context** | Reference ID + timestamp pairs (multiple allowed) |
| **User Data** | Arbitrary binary (0–130 KB) |
| **Check** | MD5 of the first 4 blocks concatenated (128 bit) |

Error codes: 1xxx(Head) / 2xxx(SubHead) / 3xxx(Context) / 4xxx(Check) / 5xxx(Internal) / 6xxx(Auth) / 7xxx(CFM)

### Documentation Index

| Document | Content |
|----------|---------|
| [core-docs.md](core-docs.md) | Rust core library: FFI API, message format, state machine, MD5, Token |
| [cfm-docs.md](cfm-docs.md) | CFM module: file hosting, FFI API, storage management |
| [server-docs.md](server-docs.md) | Go server: architecture, storage, idempotency, CFM, routing |
| [client-docs.md](client-docs.md) | Go CLI client: command structure, FFI calls, network communication |

### Building

#### One-command build (recommended)

```powershell
# Windows
.\scripts\build-all.ps1 -Clean

# Artifacts output to target/
#   smp_core.dll      Core library DLL
#   cfm.dll           CFM module DLL
#   smp-server.exe    Server executable
#   smp.exe           CLI client executable
```

#### Manual build

```bash
# 1. Rust core library
cd core && cargo build --release

# 2. Rust CFM module
cd cfm && cargo build --release

# 3. Go server
cd server && go build -o smp-server.exe ./cmd/smp-server/

# 4. Go client
cd client && go build -o smp.exe ./cmd/smp/
```

#### Cross-platform build (GitHub Actions)

The project is configured with GitHub Actions for automatic building, supporting:
- **Operating systems**: Windows, macOS, Linux
- **Architectures**: x86-64, x86, arm64

### Build Requirements

| Component | Version |
|-----------|---------|
| Rust | 1.70+ |
| Go | 1.21+ |
| C compiler | GCC/Clang (Windows cgo requires LLVM-MinGW) |

#### Windows (LLVM-MinGW)

```bash
cd core
cargo build --release --target x86_64-pc-windows-gnullvm

cd server
set CGO_ENABLED=1
set CC=x86_64-w64-mingw32-gcc
go build -o smp-server.exe ./cmd/smp-server/
```

### Server Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `--listen` | `0.0.0.0:9932` | TCP listen address |
| `--retention` | `10` | Message retention time (minutes) |
| `--cfm-path` | `./cfm-storage` | CFM storage directory |
| `--cfm-max` | `100` | CFM max file size (MB) |
| `--cfm-interval` | `10` | CFM cleanup interval (minutes) |
| `--verbose` | `false` | Verbose logging |
| `--config` | — | Config file path (YAML) |

### CLI Commands

| Command | Description |
|---------|-------------|
| `smp push [-u] <smp@server_ip> <smp@username>` | Push message; payload is read from stdin. Flags: `--force`, `--http`, `--ssh`, `--tcp`, `--smp`, `--context <id>` |
| `smp pull <smp@server_ip> <smp@username>` | Pull messages for a user. Flags: `-u`, `--force`, `--http`, `--ssh`, `--tcp`, `--smp` |
| `smp create <smp@server_ip> <smp@username>` | Register a user; returns a token bound to that user, saved to `~/.smp-token`. Flags: `--force`, `--http`, `--ssh`, `--tcp`, `--smp` |
| `smp list <smp@server_ip>` | List all users on the server. Flags: `--force`, `--http`, `--ssh`, `--tcp`, `--smp` |
| `smp watch-context` | Query the most recent message ID. Flags: `--http`, `--ssh`, `--tcp`, `--smp` |
| `smp cfm install [--force]` | Install the CFM DLL (`--force` bypasses the cache) |
| `smp cfm push <smp@server_ip> <filename>` | Upload a file to CFM. Flags: `--force`, `--http`, `--ssh`, `--tcp`, `--smp` |

Servers are addressed `smp@<ip>`, users `smp@<username>`. Authentication is the
`SMP_TOKEN` environment variable, falling back to `~/.smp-token`. There are no
global `--server` / `--token` / `--timeout` flags.

`_token_generate`, `_token_list` and `_token_revoke` exist as server routes
(`server/internal/handler/message.go`) but have no client command.

### Running Tests

```bash
cd core && cargo test --release
```

### License

MIT

---

## zh-cn

轻量级消息协议，Rust 核心库 + Go 服务端 + Go CLI 客户端。消息存储在服务器上，通过 `push` / `pull` 显式操作，不自动推送。

### 快速开始

```bash
# 1. 构建
.\scripts\build-all.ps1 -Clean

# 2. 启动服务端
./target/smp-server.exe --listen 0.0.0.0:9932 --verbose

# 3. 引导 token。create 才是签发 token 的命令，但它要求已有 token 才会执行，
#    所以先塞一个格式正确的假 token：tail 取 hex 部分最后 8 个字符，
#    塞别的值会让 head 超出 8 字节上限，服务端回 -1003 ERR_HEAD_LIMIT。
export SMP_TOKEN=smpt128-00000000000000000000000000000000

# 4. 注册用户 -> ok:<user>:token=<tok>:permissions=[...]
#    真正的 token 会自动保存到 ~/.smp-token。
./target/smp.exe create smp@127.0.0.1 smp@alice

# 去掉引导值，后面的命令才会读到 ~/.smp-token；SMP_TOKEN 环境变量优先级更高。
unset SMP_TOKEN

# 5. 推送消息。消息体从 stdin 读入，寻址到 smp@<username>。-u 记住服务端。
echo "Hello, SMP!" | ./target/smp.exe push -u smp@127.0.0.1 smp@alice

# 6. 拉取消息。必须是 token 绑定的那个用户。
./target/smp.exe pull smp@127.0.0.1 smp@alice

# 7. 大文件托管 (CFM)。只有上传，没有下载子命令。
./target/smp.exe cfm push smp@127.0.0.1 largefile.zip
```

> **已对着真服务端验证通过**（本工作树）：`create` -> `ok:alice:token=...`
> （存进 `~/.smp-token`），`push` -> `ok:<msgid>`，`pull` 能取回消息，`list`、
> `watch-context`、`cfm push` 都正常返回，且服务端在以上每一步之后都还活着。
> 往 token 未绑定的信箱写、或从那里读，被 `Error 1006` 拒绝；服务端从未签发过的
> token tail 被 `Error 1007` 拒绝；配置不允许的命令被 `Error 6005` 拒绝。
> 所以上面那个引导 token 只对 `create` 有效，不能拿它读或写别人的信箱。
> `watch-context` 现在打印的是服务端返回的、属于调用方自己信箱的 ID；
> `_cfm_download` 会把上传的字节还给上传者，其他已知用户一律拒绝。
>
> **仍未修**：服务端会丢掉消息的 `ext` 块。Rust FFI 没有对应的取值函数
> （`smp_parser_get_extensions` 不存在），所以 `handleCFMUpload` 是从**文件字节**
> 里解析 `expire` / `public`，而不是从 `client/internal/cli/cfm.go:100` 放它们的
> ext 块里解析。客户端那句 `public=false` 被静默忽略，`public=true` 根本到不了
> 服务端；要修得先在 `core` 里加一个 C FFI 再重建 `smp_core.dll`。

### 命令权限

`conf-mode` 配合 `base-permission` / `special-permission` 决定用户能跑哪些命令。
每个模式是按路由对应的 CLI 命令形式做 glob 匹配：

| 路由 | 命令 |
|------|------|
| `_pull` | `smp pull` |
| 用户信箱（`smp@alice`） | `smp push` |
| `_create` | `smp create` |
| `_list` | `smp list` |
| `_watch` | `smp watch-context` |
| `_cfm_upload` | `smp cfm upload` |
| `_cfm_download` | `smp cfm download` |
| `_token_generate` | `smp token generate` |
| `_token_list` | `smp token list` |
| `_token_revoke` | `smp token revoke` |

| `conf-mode` | 效果 |
|-------------|------|
| `"whitelist"` | 有模式命中才放行 |
| `"blacklist"` | 有模式命中就拒绝 |
| 其他任何值 / 未配置 | 按 `blacklist` 处理 |

某个用户的实际集合是 `base-permission` 加上 `special-permission[<user>]`，
按顺序去重，所以 `admin: [smp *]` 等于全放行。`_create` 同时豁免命令策略和
token 校验：它是无认证的引导入口，管它会让第一个用户拿不到 token。

示例（即仓库里的 `server/config.yml`）：

```yaml
conf-mode: "whitelist"
base-permission:
  - smp *pull*
  - smp *push*
special-permission:
  admin:
    - smp *
```

这样普通用户只能 push 和 pull；`create` 仍然可用，因为它豁免了。

### 项目结构

```
project-root/
├── core/          # Rust 核心库（协议解析/组装/校验/FFI）
├── cfm/           # Rust CFM 模块（文件托管/FFI）
├── server/        # Go 服务端（TCP 监听/存储/幂等/路由分发）
├── client/        # Go CLI 客户端（push/pull/cfm/token）
├── scripts/       # 构建脚本（build-all.ps1）
├── docs/          # 全部文档（本目录）
│   ├── README.md          # 本文件（项目总览 + 快速开始）
│   ├── core-docs.md       # 核心库（协议格式、FFI API、状态机）
│   ├── server-docs.md     # 服务端（架构、存储、路由分发）
│   ├── client-docs.md     # 客户端（命令结构、FFI 调用）
│   └── cfm-docs.md        # CFM 模块（文件托管、FFI API）
└── target/        # 构建产物（smp_core.dll, cfm.dll, smp-server.exe, smp.exe）
```

### 协议概要

消息由 5 个顺序块组成：`Head → SubHead → Context → User Data → Check (MD5)`

| 块 | 内容 |
|----|------|
| **Head** | Token 尾（8 hex）、User Data 长度、Message ID（8B, bit63=CFM） |
| **SubHead** | 版本 `smp/0.1b`、客户端地址、路由、Extensions（key=value） |
| **Context** | 引用 ID + 时间戳对（可多个） |
| **User Data** | 任意二进制（0–130KB） |
| **Check** | 前 4 块拼接的 MD5（128 bit） |

错误码：1xxx(Head) / 2xxx(SubHead) / 3xxx(Context) / 4xxx(Check) / 5xxx(Internal) / 6xxx(Auth) / 7xxx(CFM)

### 文档索引

| 文档 | 内容 |
|------|------|
| [core-docs.md](core-docs.md) | Rust 核心库：FFI API、消息格式、状态机、MD5、Token |
| [cfm-docs.md](cfm-docs.md) | CFM 模块：文件托管、FFI API、存储管理 |
| [server-docs.md](server-docs.md) | Go 服务端：架构、存储、幂等、CFM、路由分发 |
| [client-docs.md](client-docs.md) | Go CLI 客户端：命令结构、FFI 调用、网络通信 |

### 构建

#### 一键构建（推荐）

```powershell
# Windows
.\scripts\build-all.ps1 -Clean

# 产物输出到 target/
#   smp_core.dll      核心库 DLL
#   cfm.dll           CFM 模块 DLL
#   smp-server.exe    服务端可执行文件
#   smp.exe           CLI 客户端可执行文件
```

#### 手动构建

```bash
# 1. Rust 核心库
cd core && cargo build --release

# 2. Rust CFM 模块
cd cfm && cargo build --release

# 3. Go 服务端
cd server && go build -o smp-server.exe ./cmd/smp-server/

# 4. Go 客户端
cd client && go build -o smp.exe ./cmd/smp/
```

#### 跨平台构建（GitHub Actions）

项目配置了 GitHub Actions 自动构建，支持：
- **操作系统**: Windows, macOS, Linux
- **架构**: x86-64, x86, arm64

### 构建要求

| 组件 | 版本 |
|------|------|
| Rust | 1.70+ |
| Go | 1.21+ |
| C 编译器 | GCC/Clang（Windows cgo 需要 LLVM-MinGW） |

#### Windows (LLVM-MinGW)

```bash
cd core
cargo build --release --target x86_64-pc-windows-gnullvm

cd server
set CGO_ENABLED=1
set CC=x86_64-w64-mingw32-gcc
go build -o smp-server.exe ./cmd/smp-server/
```

### 服务端参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--listen` | `0.0.0.0:9932` | TCP 监听地址 |
| `--retention` | `10` | 消息保留时间（分钟） |
| `--cfm-path` | `./cfm-storage` | CFM 存储目录 |
| `--cfm-max` | `100` | CFM 最大文件大小（MB） |
| `--cfm-interval` | `10` | CFM 清理间隔（分钟） |
| `--verbose` | `false` | 详细日志 |
| `--config` | — | 配置文件路径（YAML） |

### CLI 命令

| 命令 | 说明 |
|------|------|
| `smp push [-u] <smp@server_ip> <smp@username>` | 推送消息，消息体从 stdin 读入。可用 flag：`--force`、`--http`、`--ssh`、`--tcp`、`--smp`、`--context <id>` |
| `smp pull <smp@server_ip> <smp@username>` | 拉取某用户的消息。可用 flag：`-u`、`--force`、`--http`、`--ssh`、`--tcp`、`--smp` |
| `smp create <smp@server_ip> <smp@username>` | 注册用户，返回绑定该用户的 token 并保存到 `~/.smp-token`。可用 flag：`--force`、`--http`、`--ssh`、`--tcp`、`--smp` |
| `smp list <smp@server_ip>` | 列出服务端所有用户。可用 flag：`--force`、`--http`、`--ssh`、`--tcp`、`--smp` |
| `smp watch-context` | 查询最近一条消息 ID。可用 flag：`--http`、`--ssh`、`--tcp`、`--smp` |
| `smp cfm install [--force]` | 安装 CFM DLL（`--force` 绕过缓存） |
| `smp cfm push <smp@server_ip> <filename>` | 上传文件到 CFM。可用 flag：`--force`、`--http`、`--ssh`、`--tcp`、`--smp` |

服务端地址写 `smp@<ip>`，用户写 `smp@<username>`。认证走 `SMP_TOKEN` 环境变量，
读不到则回退 `~/.smp-token`。不存在 `--server` / `--token` / `--timeout` 这类全局 flag。

`_token_generate`、`_token_list`、`_token_revoke` 只作为服务端路由存在
（`server/internal/handler/message.go`），客户端没有对应命令。

### 运行测试

```bash
cd core && cargo test --release
```

### 许可证

MIT
