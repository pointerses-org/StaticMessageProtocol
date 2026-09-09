# SMP Core Library — Developer Documentation

## en-uk

The Rust core library implements all underlying logic for the SMP protocol: message parsing, assembly, validation, and FFI exports.

### Module Structure

```
core/src/
├── lib.rs          # Module declarations and re-exports
├── error.rs        # Error code definitions
├── types.rs        # Public types and protocol constants
├── md5.rs          # MD5 hashing
├── token.rs        # Token utilities (validation, extraction, generation)
├── parser.rs       # 5-stage streaming parsing state machine
├── assembler.rs    # Message assembly and validation
└── ffi.rs          # C FFI exports
```

### Message Format

SMP messages consist of 5 sequential blocks. All strings use 2-byte BE length prefix (no `\0` delimiter):

```
+-----------+------------+-----------+------------+--------------+
|   Head    |  SubHead   |  Context  |  User Data |  Check       |
| (≤130KB)  |  (≤64KB)   |  (pairs)  |  (dynamic) |  (128 bits)  |
+-----------+------------+-----------+------------+--------------+
```

#### Head

```
[Token Tail Len (2B BE)] [Token Tail (UTF-8, ≤8 bytes)]
[User Data Len (4B BE)]
[Message ID (8B BE)]
```

- **Token Tail**: Last 8 hex characters of the full token (`smpt128-<32hex>` or `smpt256-<64hex>`)
- **Message ID**: 8-byte BE integer, **bit 63 (MSB) = 1** indicates CFM ID

| Error Code | Meaning |
|------------|---------|
| -1001 | Token validation failed |
| -1002 | Token blacklisted |
| -1003 | Head block size exceeded |
| -1004 | Message ID is zero |
| -1005 | Invalid token prefix |

#### SubHead

```
[Version Len (2B BE)] [Version]
[Client Addr Len (2B BE)] [Client Addr]
[Route Len (2B BE)] [Route]
[Extensions Len (2B BE)] [Extensions]
```

- **Version**: Fixed `smp/0.1b`, incompatible → -2001
- **Client Addr**: Client IP, ignored if missing or malformed
- **Route**: Message route, empty → -2002
- **Extensions**: `key=value\n` format, **unknown keys silently ignored**

| Error Code | Meaning |
|------------|---------|
| -2001 | Incompatible version |
| -2002 | Route unreachable or empty |
| -2003 | SubHead block size exceeded |

#### Context

```
[Pair Count (2B BE)]
[Ref ID (8B BE)] [Timestamp (4B BE)]  × Pair Count
```

- **Ref ID**: bit 63 = 1 → CFM reference, bit 63 = 0 → normal message reference
- **Timestamp**: 4-byte BE Unix timestamp (seconds)
- Validation only checks ID existence, not time window

| Error Code | Meaning |
|------------|---------|
| -3001 | Reference ID not found |
| -3003 | Context format invalid |

#### User Data

- Arbitrary binary payload, length declared in Head (0–130 KB)
- Follows directly after the Context block

#### Check

- MD5 of the first 4 blocks (Head + SubHead + Context + User Data) concatenated
- Fixed 128 bits (16 bytes)

| Error Code | Meaning |
|------------|---------|
| -4001 | MD5 checksum mismatch |
| -4002 | Check block length mismatch |
| -4003 | Check block length invalid |

### State Machine

```
HEAD → SUBHEAD → CONTEXT → USER_DATA → CHECK → DONE
  │         │          │           │          │
  └─────────┴──────────┴───────────┴──────────┘
                           ↓
                        ERROR
```

Each stage supports interception callbacks:

| Stage | Callback Type | Purpose |
|-------|---------------|---------|
| HEAD | `TokenCallback` | Token whitelist/blacklist validation |
| SUBHEAD | `RouteCallback` | Route validation |
| CONTEXT | `RefCallback` | Reference ID validation |
| CHECK | `MessageCallback` | Full message processing |

### FFI API

#### Parser

| Function | Description |
|----------|-------------|
| `smp_parser_new()` | Create parser instance |
| `smp_parser_free(handle)` | Free parser |
| `smp_parser_feed(handle, data, len)` | Feed data, returns error code |
| `smp_parser_state(handle)` | Get current stage |
| `smp_parser_error(handle)` | Get error code |
| `smp_parser_reset(handle)` | Reset parser |
| `smp_parser_set_callbacks(handle, cbs)` | Set callbacks |

#### Field Extraction

| Function | Description |
|----------|-------------|
| `smp_parser_get_msg_id(handle)` | Get Message ID |
| `smp_parser_get_token_tail(handle, buf, size)` | Get token tail |
| `smp_parser_get_route(handle, buf, size)` | Get route |
| `smp_parser_get_user_data(handle, buf, size)` | Get user data |
| `smp_parser_get_context(handle, refs, ts, max)` | Get context pairs |
| `smp_parser_get_context_count(handle)` | Get context pair count |

#### Message Assembly

| Function | Description |
|----------|-------------|
| `smp_assemble_message(...)` | Assemble full message, returns `*mut u8` |
| `smp_free_buffer(ptr, len)` | Free assembled message memory |
| `smp_validate_check(data, len)` | Validate Check block |

#### Utilities

| Function | Description |
|----------|-------------|
| `smp_md5(data, len, out)` | Compute MD5 |
| `smp_extract_token_tail(token, len, out, size)` | Extract token tail |
| `smp_validate_token(token, len)` | Validate token |
| `smp_is_cfm_id(msg_id)` | Check if CFM ID |
| `smp_error_message(code, buf, size)` | Get error message text |

### Error Code Reference

| Range | Category | Main Error Codes |
|-------|----------|-----------------|
| 1xxx | Head | -1001 Token failed, -1002 Blacklist, -1003 Exceeded, -1004 Invalid ID, -1005 Invalid prefix |
| 2xxx | SubHead | -2001 Version incompatible, -2002 Route unreachable, -2003 Exceeded |
| 3xxx | Context | -3001 Ref not found, -3003 Format invalid |
| 4xxx | Check | -4001 MD5 mismatch, -4002 Length mismatch, -4003 Length invalid |
| 5xxx | Internal | -5001 Timeout, -5002 Storage failed, -5003 Service unavailable |
| 7xxx | CFM | -7001 Not found, -7002 Expired, -7003 No permission, -7004 Too large |

### Building

```bash
# Linux/macOS
cargo build --release

# Windows (LLVM-MinGW, Clang-based)
cargo build --release --target x86_64-pc-windows-gnullvm
# Copy DLL to release directory
copy target/x86_64-pc-windows-gnullvm/release/smp_core.dll target/release/

# Test (requires GNU toolchain)
cargo test --release
```

### Key Design Decisions

1. **`gnullvm` target**: On Windows with Clang/LLVM-MinGW, the Rust core must be compiled with the `gnullvm` target to avoid `-lgcc_eh` linker dependency
2. **`__gnuc_va_list`**: cgo preprocessor requires `typedef __builtin_va_list __gnuc_va_list;` to work with Clang's `smp.h`
3. **MD5 via `md5` crate v0.8**: `md5::compute(data).0` directly returns `[u8; 16]`
4. **Callbacks pass state via `ctx` pointer**: Each connection can have independent context data

---

## zh-cn

Rust 核心库实现 SMP 协议的所有底层逻辑：消息解析、组装、校验、FFI 导出。

### 模块结构

```
core/src/
├── lib.rs          # 模块声明与重导出
├── error.rs        # 错误码定义
├── types.rs        # 公共类型与协议常量
├── md5.rs          # MD5 哈希
├── token.rs        # Token 工具（校验、提取、生成）
├── parser.rs       # 5 阶段流式解析状态机
├── assembler.rs    # 消息组装与校验
└── ffi.rs          # C FFI 导出
```

### 消息格式

SMP 消息由 5 个顺序块组成，所有字符串使用 2 字节 BE 长度前缀（无 `\0` 分隔符）：

```
+-----------+------------+-----------+------------+--------------+
|   Head    |  SubHead   |  Context  |  User Data |  Check       |
| (≤130KB)  |  (≤64KB)   |  (pairs)  |  (dynamic) |  (128 bits)  |
+-----------+------------+-----------+------------+--------------+
```

#### Head

```
[Token Tail Len (2B BE)] [Token Tail (UTF-8, ≤8 bytes)]
[User Data Len (4B BE)]
[Message ID (8B BE)]
```

- **Token Tail**: 完整 Token 的最后 8 个 hex 字符（`smpt128-<32hex>` 或 `smpt256-<64hex>`）
- **Message ID**: 8 字节 BE 整数，**bit 63 (MSB) = 1** 表示 CFM ID

| 错误码 | 含义 |
|--------|------|
| -1001  | Token 校验失败 |
| -1002  | Token 被拉黑 |
| -1003  | Head 块超限 |
| -1004  | Message ID 为零 |
| -1005  | Token 前缀无效 |

#### SubHead

```
[Version Len (2B BE)] [Version]
[Client Addr Len (2B BE)] [Client Addr]
[Route Len (2B BE)] [Route]
[Extensions Len (2B BE)] [Extensions]
```

- **Version**: 固定 `smp/0.1b`，不兼容 → -2001
- **Client Addr**: 客户端 IP，缺失或格式错误则**忽略**
- **Route**: 消息路由，空则 → -2002
- **Extensions**: `key=value\n` 格式，**未知键静默忽略**

| 错误码 | 含义 |
|--------|------|
| -2001  | 版本不兼容 |
| -2002  | 路由不可达或为空 |
| -2003  | SubHead 块超限 |

#### Context

```
[Pair Count (2B BE)]
[Ref ID (8B BE)] [Timestamp (4B BE)]  × Pair Count
```

- **Ref ID**: bit 63 = 1 → CFM 引用，bit 63 = 0 → 普通消息引用
- **Timestamp**: 4 字节 BE Unix 时间戳（秒）
- 校验仅检查 ID 是否存在，不检查时间窗口

| 错误码 | 含义 |
|--------|------|
| -3001  | 引用 ID 不存在 |
| -3003  | Context 格式非法 |

#### User Data

- 任意二进制载荷，长度在 Head 中声明（0–130 KB）
- 直接跟在 Context 块之后

#### Check

- 前 4 块（Head + SubHead + Context + User Data）拼接后的 MD5
- 固定 128 bits (16 bytes)

| 错误码 | 含义 |
|--------|------|
| -4001  | MD5 校验不匹配 |
| -4002  | Check 块长度不匹配 |
| -4003  | Check 块长度非法 |

### 状态机

```
HEAD → SUBHEAD → CONTEXT → USER_DATA → CHECK → DONE
  │         │          │           │          │
  └─────────┴──────────┴───────────┴──────────┘
                           ↓
                        ERROR
```

每阶段支持拦截回调：

| 阶段 | 回调类型 | 用途 |
|------|----------|------|
| HEAD | `TokenCallback` | Token 白名单/黑名单校验 |
| SUBHEAD | `RouteCallback` | 路由校验 |
| CONTEXT | `RefCallback` | 引用 ID 校验 |
| CHECK | `MessageCallback` | 完整消息处理 |

### FFI API

#### 解析器

| 函数 | 说明 |
|------|------|
| `smp_parser_new()` | 创建解析器实例 |
| `smp_parser_free(handle)` | 释放解析器 |
| `smp_parser_feed(handle, data, len)` | 喂入数据，返回错误码 |
| `smp_parser_state(handle)` | 获取当前阶段 |
| `smp_parser_error(handle)` | 获取错误码 |
| `smp_parser_reset(handle)` | 重置解析器 |
| `smp_parser_set_callbacks(handle, cbs)` | 设置回调 |

#### 字段提取

| 函数 | 说明 |
|------|------|
| `smp_parser_get_msg_id(handle)` | 获取 Message ID |
| `smp_parser_get_token_tail(handle, buf, size)` | 获取 Token 尾 |
| `smp_parser_get_route(handle, buf, size)` | 获取路由 |
| `smp_parser_get_user_data(handle, buf, size)` | 获取用户数据 |
| `smp_parser_get_context(handle, refs, ts, max)` | 获取 Context 对 |
| `smp_parser_get_context_count(handle)` | 获取 Context 对数量 |

#### 消息组装

| 函数 | 说明 |
|------|------|
| `smp_assemble_message(...)` | 组装完整消息，返回 `*mut u8` |
| `smp_free_buffer(ptr, len)` | 释放组装的消息内存 |
| `smp_validate_check(data, len)` | 校验 Check 块 |

#### 工具

| 函数 | 说明 |
|------|------|
| `smp_md5(data, len, out)` | 计算 MD5 |
| `smp_extract_token_tail(token, len, out, size)` | 提取 Token 尾 |
| `smp_validate_token(token, len)` | 校验 Token |
| `smp_is_cfm_id(msg_id)` | 判断是否 CFM ID |
| `smp_error_message(code, buf, size)` | 获取错误消息文本 |

### 错误码参考

| 范围 | 类别 | 主要错误码 |
|------|------|-----------|
| 1xxx | Head | -1001 Token 失败, -1002 黑名单, -1003 超限, -1004 无效 ID, -1005 无效前缀 |
| 2xxx | SubHead | -2001 版本不兼容, -2002 路由不可达, -2003 超限 |
| 3xxx | Context | -3001 引用不存在, -3003 格式非法 |
| 4xxx | Check | -4001 MD5 不匹配, -4002 长度不匹配, -4003 长度非法 |
| 5xxx | 内部 | -5001 超时, -5002 存储失败, -5003 服务不可用 |
| 7xxx | CFM | -7001 未找到, -7002 已过期, -7003 无权限, -7004 过大 |

### 构建

```bash
# Linux/macOS
cargo build --release

# Windows (LLVM-MinGW, Clang-based)
cargo build --release --target x86_64-pc-windows-gnullvm
# 复制 DLL 到 release 目录
copy target/x86_64-pc-windows-gnullvm/release/smp_core.dll target/release/

# 测试 (需要 GNU 工具链)
cargo test --release
```

### 关键设计决策

1. **`gnullvm` 目标**: Windows 下使用 Clang/LLVM-MinGW 时，Rust 核心必须用 `gnullvm` 目标编译，避免 `-lgcc_eh` 链接器依赖
2. **`__gnuc_va_list`**: cgo 预处理器需要 `typedef __builtin_va_list __gnuc_va_list;` 才能与 Clang 的 `smp.h` 配合
3. **MD5 使用 `md5` crate v0.8**: `md5::compute(data).0` 直接返回 `[u8; 16]`
4. **回调通过 `ctx` 指针传递状态**: 每个连接可以有独立的上下文数据
