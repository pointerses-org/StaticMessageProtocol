/*
 * cfm.h — CFM (Cloud File Message) C FFI Header
 *
 * Part of SMP (Static Message Protocol)
 */

#ifndef CFM_H
#define CFM_H

#include <stdint.h>
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Error codes */
#define ERR_CFM_OK               0
#define ERR_CFM_NOT_FOUND      -7001
#define ERR_CFM_EXPIRED        -7002
#define ERR_CFM_NO_PERMISSION  -7003
#define ERR_CFM_TOO_LARGE      -7004
#define ERR_CFM_STORE_FULL     -7005
#define ERR_CFM_FORMAT_INVALID -7006
#define ERR_CFM_INTERNAL       -7099

/* Initialize the CFM store.
 * base_dir: file system path for storage (e.g. "./cfm-storage")
 * max_mb: maximum file size in MB (0 = 100 MB default)
 * Returns: ERR_CFM_OK on success
 */
int32_t cfm_init(const char* base_dir, uint64_t max_mb);

/* Save a file to CFM store.
 * cf_id: unique file identifier (use bit 63 = 1 for CFM)
 * data: file data buffer
 * data_len: length of data
 * uploader: uploader username (can be NULL)
 * public: if true, anyone can download
 * expire_minutes: expiry duration in minutes
 * Returns: ERR_CFM_OK on success
 */
int32_t cfm_save(uint64_t cf_id, const uint8_t* data, size_t data_len,
                 const char* uploader, bool public, uint64_t expire_minutes);

/* Load a file from CFM store.
 * cf_id: file identifier
 * requester: requesting username (can be NULL for public files)
 * out_buf: output buffer
 * out_buf_size: size of output buffer
 * out_len: [out] actual bytes written
 * Returns: ERR_CFM_OK on success
 */
int32_t cfm_load(uint64_t cf_id, const char* requester,
                 uint8_t* out_buf, size_t out_buf_size, size_t* out_len);

/* Check if a CFM file exists. */
bool cfm_exists(uint64_t cf_id);

/* Get the size of a CFM file in bytes. Returns -1 if not found. */
int64_t cfm_size(uint64_t cf_id);

/* Get the uploader of a CFM file.
 * out_buf: output buffer (null-terminated)
 * out_buf_size: size of output buffer
 * Returns: length of string written, or 0 if not found
 */
int32_t cfm_get_uploader(uint64_t cf_id, char* out_buf, size_t out_buf_size);

/* Get the expiry timestamp (Unix seconds) of a CFM file. Returns 0 if not found. */
uint64_t cfm_get_expiry(uint64_t cf_id);

/* Check if a CFM file is public. */
bool cfm_is_public(uint64_t cf_id);

/* List all CFM file IDs.
 * out_ids: output buffer for IDs
 * max_count: maximum number of IDs to return
 * out_count: [out] actual number of IDs returned
 * Returns: ERR_CFM_OK on success
 */
int32_t cfm_list(uint64_t* out_ids, size_t max_count, size_t* out_count);

/* Cleanup expired CFM files. Returns number of files removed. */
int32_t cfm_cleanup(void);

/* Get total storage usage in bytes. */
uint64_t cfm_usage(void);

/* Get error message for a CFM error code.
 * out_buf: output buffer (null-terminated)
 * out_buf_size: size of output buffer
 * Returns: length of message written
 */
int32_t cfm_error_message(int32_t code, char* out_buf, size_t out_buf_size);

#ifdef __cplusplus
}
#endif

#endif /* CFM_H */
