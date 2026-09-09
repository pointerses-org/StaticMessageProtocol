# SMP — Static Message Protocol

轻量级消息协议，Rust 核心库 + Go 服务端 + Go CLI 客户端。消息存储在服务器上，通过 `push` / `pull` 显式操作，不自动推送。

## 快速开始

```bash
# 1. 构建
.\scripts\build-all.ps1 -Clean          # 一键构建全部产物到 target/

# 2. 启动服务端
./target/smp-server.exe --listen 0.0.0.0:9932 --verbose

# 3. 生成 Token
./target/smp.exe token generate

# 4. 推送消息
./target/smp.exe push myroute --data "Hello, SMP!"

# 5. 拉取消息
./target/smp.exe pull --limit 20 --route myroute

# 6. 大文件托管 (CFM)
./target/smp.exe cfm largefile.zip --expire 48h --public
./target/smp.exe cfm 0x8000000000000001 -o output.zip
```

## 项目结构

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

## 协议概要

消息由 5 个顺序块组成：`Head → SubHead → Context → User Data → Check (MD5)`

| 块 | 内容 |
|----|------|
| **Head** | Token 尾（8 hex）、User Data 长度、Message ID（8B, bit63=CFM） |
| **SubHead** | 版本 `smp/0.1b`、客户端地址、路由、Extensions（key=value） |
| **Context** | 引用 ID + 时间戳对（可多个） |
| **User Data** | 任意二进制（0–130KB） |
| **Check** | 前 4 块拼接的 MD5（128 bit） |

错误码：1xxx(Head) / 2xxx(SubHead) / 3xxx(Context) / 4xxx(Check) / 5xxx(Internal) / 6xxx(Auth) / 7xxx(CFM)

## 文档索引

| 文档 | 内容 |
|------|------|
| [core-docs.md](core-docs.md) | Rust 核心库：FFI API、消息格式、状态机、MD5、Token |
| [cfm-docs.md](cfm-docs.md) | CFM 模块：文件托管、FFI API、存储管理 |
| [server-docs.md](server-docs.md) | Go 服务端：架构、存储、幂等、CFM、路由分发 |
| [client-docs.md](client-docs.md) | Go CLI 客户端：命令结构、FFI 调用、网络通信 |

## 构建

### 一键构建（推荐）

```powershell
# Windows
.\scripts\build-all.ps1 -Clean

# 产物输出到 target/
#   smp_core.dll      核心库 DLL
#   cfm.dll           CFM 模块 DLL
#   smp-server.exe    服务端可执行文件
#   smp.exe           CLI 客户端可执行文件
```

### 手动构建

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

### 跨平台构建（GitHub Actions）

项目配置了 GitHub Actions 自动构建，支持：
- **操作系统**: Windows, macOS, Linux
- **架构**: x86-64, x86, arm64

## 构建要求

| 组件 | 版本 |
|------|------|
| Rust | 1.70+ |
| Go | 1.21+ |
| C 编译器 | GCC/Clang（Windows cgo 需要 LLVM-MinGW） |

### Windows (LLVM-MinGW)

```bash
cd core
cargo build --release --target x86_64-pc-windows-gnullvm

cd server
set CGO_ENABLED=1
set CC=x86_64-w64-mingw32-gcc
go build -o smp-server.exe ./cmd/smp-server/
```

## 服务端参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--listen` | `0.0.0.0:9932` | TCP 监听地址 |
| `--retention` | `10` | 消息保留时间（分钟） |
| `--cfm-path` | `./cfm-storage` | CFM 存储目录 |
| `--cfm-max` | `100` | CFM 最大文件大小（MB） |
| `--cfm-interval` | `10` | CFM 清理间隔（分钟） |
| `--verbose` | `false` | 详细日志 |
| `--config` | — | 配置文件路径（YAML） |

## CLI 命令

| 命令 | 说明 |
|------|------|
| `smp push <route>` | 推送消息，支持 `--data`、`--file`、`--context`、`--id` |
| `smp pull` | 拉取消息，支持 `--limit`、`--offset`、`--route`、`--output` |
| `smp cfm <file>` | 上传文件，支持 `--expire`、`--public` |
| `smp cfm <id>` | 下载文件，支持 `-o` |
| `smp token generate` | 生成 Token |
| `smp token list` | 列出所有 Token |
| `smp token revoke <tail>` | 吊销 Token |

全局参数：`--server`、`--token`、`--timeout`、`--verbose`

## 运行测试

```bash
cd core && cargo test --release
```

## 许可证

MIT
