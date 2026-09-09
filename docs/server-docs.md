# SMP Server — Developer Documentation

Go 服务端实现 TCP 监听、消息存储、幂等缓存、CFM 文件托管、Token 管理。

## 包结构

```
server/
├── cmd/smp-server/main.go       # 入口（薄层）
└── internal/
    ├── config/config.go         # 配置解析
    ├── server/server.go         # 服务器生命周期
    ├── handler/
    │   ├── connection.go        # TCP 连接处理
    │   └── message.go           # 消息路由分发
    ├── store/
    │   ├── message.go           # 消息存储
    │   └── cfm.go               # CFM 文件存储
    ├── cache/idempotency.go     # 幂等缓存 (LRU 10K)
    └── auth/token.go            # Token 管理
```

## 架构

```
TCP Client → net.Listen → acceptLoop → handleConn
                                        ↓
                              SmpParser (Rust FFI)
                                        ↓
                              processParsedMessage
                                        ↓
                    ┌───────────┬──────────┬──────────┬──────────┐
                    │ _pull     │ _cfm_*   │ _token_* │  default  │
                    │ (查询)    │ (文件)   │ (Token)  │  (推送)   │
                    └───────────┴──────────┴──────────┴──────────┘
```

## 路由分发

| 路由 | 说明 | 请求格式 | 响应格式 |
|------|------|----------|----------|
| `_pull` | 查询消息 | `limit=N&offset=N&after=TS&before=TS&route=R&id=ID` | 二进制消息列表 |
| `_cfm_upload` | 上传 CFM | `expire=N&public=BOOL` (UserData 为文件元数据) | `cfm_id=0x...` |
| `_cfm_download` | 下载 CFM | `cfm_id=0x...` | 文件二进制数据 |
| `_token_generate` | 生成 Token | (空) | `smpt128-<32hex>` |
| `_token_list` | 列出 Token | (空) | `[tail1, tail2, ...]` |
| `_token_revoke` | 吊销 Token | Token 尾字符串 | `revoked` / `not_found` |
| 其他 | 推送消息 | (任意 UserData) | `ok:<msg_id>` |

## 消息存储

- **内存存储**: `map[route][]*StoredMessage`
- **保留时间**: 可配置（默认 10 分钟），定期清理
- **查询**: 支持 `limit`、`offset`、`after`、`before`、`route`、`id` 过滤
- **序列化**: 二进制格式（ID 8B + Route 长度前缀 + UserData 长度前缀 + Context 对）

## 幂等缓存

- **容量**: 10,000 条
- **键**: `"%x:%s"` (msgID hex + tokenTail)
- **淘汰**: LRU（最近最少使用）
- **行为**: 重复 ID 返回缓存响应，不重复执行业务逻辑

## CFM 文件存储

- **存储目录**: 可配置（默认 `./cfm-storage`）
- **文件命名**: `%016x.bin` (CFM ID 的十六进制表示)
- **权限**:
  - 上传者: 始终可访问
  - `--public=true`: 所有人可访问
  - `--public=false`: 仅上传者可访问（错误 -7003）
- **清理**: 定期扫描，删除过期文件
- **大小限制**: 可配置（默认 100 MB）

## Token 管理

- **格式**: `smpt128-<32 hex>`
- **存储**: 内存 map (tail → full token)
- **生成**: `math/rand` + `time.Now().UnixNano()` 种子（非密码学安全，文档中已注明）
- **操作**: 生成、查询、列出、吊销

## 配置

### 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--listen` | `0.0.0.0:9932` | TCP 监听地址 |
| `--retention` | `10` | 消息保留时间（分钟） |
| `--cfm-path` | `./cfm-storage` | CFM 存储目录 |
| `--cfm-max` | `100` | CFM 最大文件大小（MB） |
| `--cfm-interval` | `10` | CFM 清理间隔（分钟） |
| `--verbose` | `false` | 详细日志 |
| `--config` | (无) | 配置文件路径 |

### 配置文件格式

```
listen = 0.0.0.0:9932
retention = 10
cfm_path = ./cfm-storage
cfm_max_mb = 100
cfm_interval = 10
verbose = false
```

支持 `#` 注释和 `[section]` 段落（段落名被忽略）。

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

**关键**: `__gnuc_va_list` 的 `typedef` 必须在 `#include "smp.h"` 之前，否则 Clang/LLVM-MinGW 编译失败。

## 构建

```bash
cd server
export CGO_ENABLED=1
export CC=x86_64-w64-mingw32-gcc  # Windows
export GOTMPDIR=$PWD/tmp
go build -o smp-server.exe ./cmd/smp-server/
```

## 错误处理

| 场景 | 错误码 | 响应 |
|------|--------|------|
| 消息校验失败 | 负数错误码 | `error:<code>:<message>` |
| 存储失败 | -5002 | `error:5002:<detail>` |
| 超时 | -5001 | 连接关闭 |
