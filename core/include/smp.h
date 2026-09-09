/**
 * SMP Core - C FFI Header
 *
 * This header declares the C-compatible functions exported by the Rust
 * smp-core library. Go code uses cgo to call these functions.
 *
 * Platform outputs:
 *   Windows: smp_core.dll
 *   Linux:   libsmp_core.so
 *   macOS:   libsmp_core.dylib
 */

#ifndef SMP_CORE_H
#define SMP_CORE_H

#include <stdint.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/* =====================================================================
 * Opaque Handle
 * ===================================================================== */

/** Opaque parser handle. */
typedef struct SmpParser SmpParser;

/* =====================================================================
 * Stage Constants
 * ===================================================================== */

#define SMP_STAGE_HEAD      0
#define SMP_STAGE_SUBHEAD   1
#define SMP_STAGE_CONTEXT   2
#define SMP_STAGE_USER_DATA 3
#define SMP_STAGE_CHECK     4
#define SMP_STAGE_DONE      5
#define SMP_STAGE_ERROR     6

/* =====================================================================
 * Error Codes (negative = error, 0 = OK)
 * ===================================================================== */

#define SMP_ERR_OK                    0
#define SMP_ERR_TOKEN_FAILED         -1001
#define SMP_ERR_TOKEN_BLACKLIST      -1002
#define SMP_ERR_HEAD_LIMIT           -1003
#define SMP_ERR_ID_INVALID           -1004
#define SMP_ERR_PREFIX_INVALID       -1005
#define SMP_ERR_VERSION_INCOMPATIBLE -2001
#define SMP_ERR_ROUTE_UNREACHABLE    -2002
#define SMP_ERR_SUBHEAD_LIMIT        -2003
#define SMP_ERR_REF_NOT_FOUND        -3001
#define SMP_ERR_CONTEXT_FORMAT       -3003
#define SMP_ERR_CHECK_MISMATCH       -4001
#define SMP_ERR_CHECK_LENGTH         -4002
#define SMP_ERR_CHECK_LENGTH_ILLEGAL -4003
#define SMP_ERR_TIMEOUT              -5001
#define SMP_ERR_STORAGE_FAIL         -5002
#define SMP_ERR_SERVICE_UNAVAILABLE  -5003
#define SMP_ERR_CFM_NOT_FOUND        -7001
#define SMP_ERR_CFM_EXPIRED          -7002
#define SMP_ERR_CFM_NO_PERMISSION    -7003
#define SMP_ERR_CFM_TOO_LARGE        -7004

/* =====================================================================
 * Callback Types
 * ===================================================================== */

/**
 * Token validation callback — called after Head is parsed.
 * @param ctx       User-provided context pointer
 * @param token_tail  Pointer to token tail bytes (last 8 hex chars)
 * @param token_len   Length of token tail
 * @return 0 to accept, negative error code to reject
 */
typedef int32_t (*smp_token_cb)(void* ctx, const uint8_t* token_tail, size_t token_len);

/**
 * Route validation callback — called after SubHead is parsed.
 * @param ctx   User-provided context pointer
 * @param route  Pointer to route bytes
 * @param route_len  Length of route
 * @return 0 to accept, negative error code to reject
 */
typedef int32_t (*smp_route_cb)(void* ctx, const uint8_t* route, size_t route_len);

/**
 * Reference validation callback — called after Context is parsed.
 * @param ctx   User-provided context pointer
 * @param ref_id  Full 64-bit reference ID (bit 63 may be CFM flag)
 * @param timestamp  Unix timestamp (seconds)
 * @return 0 if reference exists, negative error code if not
 */
typedef int32_t (*smp_ref_cb)(void* ctx, uint64_t ref_id, uint32_t timestamp);

/**
 * Message received callback — called when the full message is validated.
 * @param ctx   User-provided context pointer
 * @param msg_id  Message ID (64-bit, bit 63 may be CFM flag)
 * @param route  Pointer to route bytes
 * @param route_len  Length of route
 * @param user_data  Pointer to user data bytes
 * @param user_data_len  Length of user data
 * @param context_count  Number of context pairs
 * @param refs  Pointer to array of context reference IDs
 * @param timestamps  Pointer to array of context timestamps
 * @return 0 to accept, negative error code to reject
 */
typedef int32_t (*smp_message_cb)(
    void* ctx,
    uint64_t msg_id,
    const uint8_t* route, size_t route_len,
    const uint8_t* user_data, size_t user_data_len,
    size_t context_count,
    const uint64_t* refs,
    const uint32_t* timestamps
);

/* =====================================================================
 * Callbacks Bundle
 * ===================================================================== */

/** Callbacks bundle, passed to smp_parser_set_callbacks. */
typedef struct {
    smp_token_cb   token_cb;
    smp_route_cb   route_cb;
    smp_ref_cb     ref_cb;
    smp_message_cb message_cb;
    void*          ctx;
} smp_callbacks_t;

/* =====================================================================
 * Parser Functions
 * ===================================================================== */

/** Create a new parser instance. */
SmpParser* smp_parser_new(void);

/** Free a parser instance. */
void smp_parser_free(SmpParser* handle);

/**
 * Feed bytes to the parser.
 * @return SMP_ERR_OK if parsing continues, or negative error code.
 */
int32_t smp_parser_feed(SmpParser* handle, const uint8_t* data, size_t len);

/** Get the current parser stage. */
uint32_t smp_parser_state(SmpParser* handle);

/** Get the last error code. */
int32_t smp_parser_error(SmpParser* handle);

/** Reset the parser to its initial state. */
void smp_parser_reset(SmpParser* handle);

/** Set callbacks for stage interception. */
void smp_parser_set_callbacks(SmpParser* handle, const smp_callbacks_t* cbs);

/* =====================================================================
 * Message Assembly
 * ===================================================================== */

/**
 * Assemble a complete SMP message.
 * @param token_tail   Token tail bytes (last 8 hex chars)
 * @param token_len    Length of token tail
 * @param msg_id       Message ID (64-bit, bit 63 may be CFM flag)
 * @param version      Protocol version string (e.g. "smp/0.1b")
 * @param version_len  Length of version
 * @param client_addr  Client address string
 * @param client_addr_len  Length of client address
 * @param route        Route string
 * @param route_len    Length of route
 * @param extensions   Extensions string (key=value\n format)
 * @param extensions_len  Length of extensions
 * @param context_count  Number of context pairs
 * @param refs         Array of context reference IDs
 * @param timestamps   Array of context timestamps
 * @param user_data    User data bytes
 * @param user_data_len  Length of user data
 * @param out_len      [out] Length of assembled message
 * @return Pointer to allocated buffer (use smp_free_buffer to free), or NULL on error.
 */
uint8_t* smp_assemble_message(
    const uint8_t* token_tail, size_t token_len,
    uint64_t msg_id,
    const uint8_t* version, size_t version_len,
    const uint8_t* client_addr, size_t client_addr_len,
    const uint8_t* route, size_t route_len,
    const uint8_t* extensions, size_t extensions_len,
    size_t context_count,
    const uint64_t* refs,
    const uint32_t* timestamps,
    const uint8_t* user_data, size_t user_data_len,
    size_t* out_len
);

/** Free a buffer allocated by smp_assemble_message. */
void smp_free_buffer(uint8_t* buf, size_t len);

/* =====================================================================
 * MD5 & Check Validation
 * ===================================================================== */

/** Compute MD5 hash of data. Writes 16 bytes to out. */
void smp_md5(const uint8_t* data, size_t len, uint8_t out[16]);

/**
 * Validate the Check block of a complete SMP message.
 * @return SMP_ERR_OK if valid, or negative error code.
 */
int32_t smp_validate_check(const uint8_t* data, size_t len);

/* =====================================================================
 * Token Utilities
 * ===================================================================== */

/**
 * Extract the token tail from a full token string.
 * @param token      Full token string (e.g. "smpt128-...")
 * @param token_len  Length of token
 * @param out_buf    [out] Buffer to receive the token tail (at least 8 bytes)
 * @param out_buf_size  Size of out_buf
 * @return Length of token tail written, or -1 if malformed.
 */
int32_t smp_extract_token_tail(
    const uint8_t* token, size_t token_len,
    uint8_t* out_buf, size_t out_buf_size
);

/**
 * Validate a full token string.
 * @return SMP_ERR_OK if valid, or SMP_ERR_PREFIX_INVALID.
 */
int32_t smp_validate_token(const uint8_t* token, size_t token_len);

/** Check if a message ID is a CFM ID (bit 63 set). */
int smp_is_cfm_id(uint64_t msg_id);

/**
 * Get the error message string for a given error code.
 * Writes a null-terminated string to out_buf.
 * @return Number of bytes written (excluding null terminator).
 */
int32_t smp_error_message(int32_t code, uint8_t* out_buf, size_t out_buf_size);

/* =====================================================================
 * Parser Field Extraction (after STAGE_DONE)
 * ===================================================================== */

/** Get the message ID from a parsed parser. */
uint64_t smp_parser_get_msg_id(SmpParser* handle);

/** Get the token tail from a parsed parser. Returns length written. */
int32_t smp_parser_get_token_tail(SmpParser* handle, uint8_t* out_buf, size_t out_buf_size);

/** Get the route from a parsed parser. Returns length written. */
int32_t smp_parser_get_route(SmpParser* handle, uint8_t* out_buf, size_t out_buf_size);

/** Get the user data from a parsed parser. Returns length written. */
int32_t smp_parser_get_user_data(SmpParser* handle, uint8_t* out_buf, size_t out_buf_size);

/** Get context pairs from a parsed parser. Returns number of pairs written. */
int32_t smp_parser_get_context(SmpParser* handle, uint64_t* out_refs, uint32_t* out_ts, size_t max_pairs);

/** Get context pair count from a parsed parser. */
int32_t smp_parser_get_context_count(SmpParser* handle);

#ifdef __cplusplus
}
#endif

#endif /* SMP_CORE_H */
