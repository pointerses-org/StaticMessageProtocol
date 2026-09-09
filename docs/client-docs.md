# SMP CLI Client — Developer Documentation

Go CLI 客户端实现 push/pull/cfm/token 命令，通过 cgo 调用 Rust 核心库组装和解析消息。

## 包结构

```
client/
├── cmd/smp/main.go              # 入口（薄层）
└── internal/cli/
    ├── ffi.go                   # FFI 封装（消息组装、Token 提取）
    ├── net.go                   # TCP 连接与通信
    ├── push.go                  # push 命令
    ├── pull.go                  # pull 命令
    ├── cfm.go                   # cfm 上传/下载
    ├── token.go                 # token 管理
    └── helpers.go               # 工具函数（ID 生成、地址获取、Token 提取）
```

## 命令结构

```
smp [global flags] <command> [command flags]

Global Flags:
  --server URL       服务器地址 (smp://host:port)
  --token TOKEN      认证 Token (或 SMP_TOKEN 环境变量)
  --timeout N        超时秒数 (默认 30)
  --verbose          详细输出

Commands:
  push <route>       推送消息
  pull               拉取消息
  cfm <file>         上传文件
  cfm <cfm_id>       下载文件
  token <action>     Token 管理 (generate|list|revoke)
```

## FFI 封装

### 消息组装

```go
func AssembleMessage(
    tokenTail string, msgID uint64, route string,
    userData []byte, contextPairs []ContextPair, extensions string,
) ([]byte, error)
```

调用 `smp_assemble_message` FFI 函数，组装完整 SMP 消息（含 MD5 Check）。

### Token 提取

```go
func ExtractTokenTail(fullToken string) string
```

调用 `smp_extract_token_tail` FFI 函数，从完整 Token（`smpt128-<32hex>`）提取最后 8 个 hex 字符。

## 网络通信

```go
func (c *Config) ConnectToServer() (net.Conn, error)
func SendAndReceive(conn net.Conn, msg []byte, timeout int) ([]byte, error)
```

- 解析 `smp://` 或 `smp+ssh://` URL
- 超时连接
- 写入消息，读取响应

## Push 命令

```
smp push <route> [--data text] [--file file] [--context ref:ts]... [--id N]
```

| 参数 | 说明 |
|------|------|
| `<route>` | 目标路由 |
| `--data TEXT` | 消息数据（文本） |
| `--file PATH` | 从文件读取数据 |
| `--context REF:TS` | Context 引用（十六进制 ID:十进制时间戳，逗号分隔） |
| `--id N` | 消息 ID（0=自动生成） |

**数据源优先级**: `--file` > `--data` > stdin

## Pull 命令

```
smp pull [--limit N] [--offset N] [--after TS] [--before TS] [--route R] [--id N] [--output table|json|raw]
```

构建查询字符串 `limit=N&offset=N&after=TS&before=TS&route=R&id=N`，发送到 `_pull` 路由。

**输出格式**:
- `table`: 表格显示（ID、路由、大小、数据）
- `json`: 原始 JSON 响应
- `raw`: 原始二进制输出

## CFM 命令

### 上传

```
smp cfm <file> [--expire 24h] [--public]
```

| 参数 | 说明 |
|------|------|
| `<file>` | 文件路径 |
| `--expire DUR` | 过期时间（`24h`=24小时，`30m`=30分钟） |
| `--public` | 公开访问 |

使用 `_cfm_upload` 路由，Message ID 设置 bit 63 标记为 CFM。

### 下载

```
smp cfm <cfm_id> [-o output]
```

| 参数 | 说明 |
|------|------|
| `<cfm_id>` | CFM ID（十六进制或十进制） |
| `-o OUTPUT` | 输出文件路径（不指定则输出到 stdout） |

使用 `_cfm_download` 路由，查询字符串为 `cfm_id=0x...`。

## Token 命令

```
smp token generate     # 生成新 Token
smp token list         # 列出所有 Token
smp token revoke <tail> # 吊销 Token
```

分别使用 `_token_generate`、`_token_list`、`_token_revoke` 路由。

## cgo FFI 绑定

```go
/*
    #cgo CFLAGS: -I${SRCDIR}/../../../core/include
    #cgo LDFLAGS: -L${SRCDIR}/../../../core/target/release -lsmp_core
    typedef __builtin_va_list __gnuc_va_list;
    #include <stdlib.h>
    #include "smp.h"
*/
import "C"
```

## 构建

```bash
cd client
export CGO_ENABLED=1
export CC=x86_64-w64-mingw32-gcc  # Windows
export GOTMPDIR=$PWD/tmp
go build -o smp.exe ./cmd/smp/
```

## 关键设计

1. **全局配置**: `Config` 结构体持有 server/token/timeout/verbose，通过指针传递给各命令
2. **Token 提取**: 完整 Token 在客户端本地提取尾（8 hex），只发送尾到服务端
3. **Context 格式**: `refID:timestamp` 逗号分隔，refID 十六进制，timestamp 十进制
4. **CFM ID 解析**: 支持 `0x` 前缀（十六进制）和纯数字（十进制）
