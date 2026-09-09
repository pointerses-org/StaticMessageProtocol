# CFM Module — Developer Documentation

CFM (Cloud File Message) 模块提供大文件托管功能，作为独立 Rust 库编译为 `cfm.dll`。

## 架构

```
cfm/
├── Cargo.toml              # 独立 Rust crate
├── .cargo/config.toml      # gnullvm target
├── src/lib.rs              # CFM 实现 + FFI 导出
└── include/cfm.h           # C 头文件
```

## FFI API

### 初始化

| 函数 | 说明 |
|------|------|
| `cfm_init(base_dir, max_mb)` | 初始化存储目录，设置最大文件大小 |

### 文件操作

| 函数 | 说明 |
|------|------|
| `cfm_save(cf_id, data, len, uploader, public, expire)` | 上传文件 |
| `cfm_load(cf_id, requester, out_buf, size, out_len)` | 下载文件 |
| `cfm_exists(cf_id)` | 检查文件是否存在 |
| `cfm_size(cf_id)` | 获取文件大小 |
| `cfm_cleanup()` | 清理过期文件 |
| `cfm_usage()` | 获取存储用量 |

### 元数据

| 函数 | 说明 |
|------|------|
| `cfm_get_uploader(cf_id, buf, size)` | 获取上传者 |
| `cfm_get_expiry(cf_id)` | 获取过期时间（Unix 秒） |
| `cfm_is_public(cf_id)` | 检查是否公开 |
| `cfm_list(out_ids, max, out_count)` | 列出所有文件 ID |

### 错误处理

| 函数 | 说明 |
|------|------|
| `cfm_error_message(code, buf, size)` | 获取错误消息 |

## 错误码

| 错误码 | 含义 |
|--------|------|
| `ERR_CFM_OK` (0) | 成功 |
| `ERR_CFM_NOT_FOUND` (-7001) | 文件不存在 |
| `ERR_CFM_EXPIRED` (-7002) | 文件已过期 |
| `ERR_CFM_NO_PERMISSION` (-7003) | 无权限访问 |
| `ERR_CFM_TOO_LARGE` (-7004) | 文件超过大小限制 |
| `ERR_CFM_STORE_FULL` (-7005) | 存储已满 |
| `ERR_CFM_FORMAT_INVALID` (-7006) | 文件格式不支持 |
| `ERR_CFM_INTERNAL` (-7099) | 内部错误 |

## CFM ID 格式

CFM 文件使用特殊的 Message ID：
- **bit 63 (MSB) = 1** 表示 CFM 引用
- 格式：`0x8000000000000001`、`0x8000000000000002` 等

## 存储

- **文件系统存储**：文件以 `%016x.bin` 格式存储在指定目录
- **内存元数据**：`HashMap<u64, FileEntry>` 存储文件信息
- **过期清理**：定期调用 `cfm_cleanup()` 删除过期文件
- **权限控制**：
  - `public = true`：所有人可下载
  - `public = false`：仅上传者或管理员可下载

## 构建

```bash
# 直接构建
cd cfm
cargo build --release

# 使用 build-all.ps1
.\scripts\build-all.ps1 -Clean
# 产物自动输出到 target/cfm.dll
```

## 与客户端集成

客户端 `smp cfm` 命令通过 `cfm.dll` 实现文件操作：

```
smp cfm install [--force]    # 安装/更新 cfm.dll
smp cfm push <file>          # 上传文件
smp cfm <cfm_id> -o out.bin  # 下载文件
```
