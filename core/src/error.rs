//! Error codes for SMP protocol.

// OK
pub const ERR_OK: i32 = 0;

// 1xxx: Head errors
pub const ERR_TOKEN_FAILED: i32 = -1001;
pub const ERR_TOKEN_BLACKLIST: i32 = -1002;
pub const ERR_HEAD_LIMIT: i32 = -1003;
pub const ERR_ID_INVALID: i32 = -1004;
pub const ERR_PREFIX_INVALID: i32 = -1005;
pub const ERR_TOKEN_USER_MISMATCH: i32 = -1006;  // Token bound to different user
pub const ERR_TOKEN_NOT_FOUND: i32 = -1007;      // Token tail not found
pub const ERR_TOKEN_REVOKED: i32 = -1008;        // Token has been revoked

// 2xxx: SubHead errors
pub const ERR_VERSION_INCOMPATIBLE: i32 = -2001;
pub const ERR_ROUTE_UNREACHABLE: i32 = -2002;
pub const ERR_SUBHEAD_LIMIT: i32 = -2003;
pub const ERR_ROUTE_INVALID: i32 = -2004;        // Route format invalid

// 3xxx: Context errors
pub const ERR_REF_NOT_FOUND: i32 = -3001;
pub const ERR_CONTEXT_FORMAT: i32 = -3003;
pub const ERR_CONTEXT_EXPIRED: i32 = -3004;      // Context reference expired

// 4xxx: Check errors
pub const ERR_CHECK_MISMATCH: i32 = -4001;
pub const ERR_CHECK_LENGTH: i32 = -4002;
pub const ERR_CHECK_LENGTH_ILLEGAL: i32 = -4003;

// 5xxx: Internal errors
pub const ERR_TIMEOUT: i32 = -5001;
pub const ERR_STORAGE_FAIL: i32 = -5002;
pub const ERR_SERVICE_UNAVAILABLE: i32 = -5003;
pub const ERR_INTERNAL_ERROR: i32 = -5004;       // Generic internal error
pub const ERR_RATE_LIMITED: i32 = -5005;         // Rate limit exceeded

// 6xxx: Authentication errors
pub const ERR_ACCESS_DENIED: i32 = -6001;        // User has no permission
pub const ERR_USERNAME_REQUIRED: i32 = -6002;    // Username not provided
pub const ERR_USER_EXISTS: i32 = -6003;          // User already exists
pub const ERR_USER_NOT_FOUND: i32 = -6004;       // User not found
pub const ERR_PERMISSION_DENIED: i32 = -6005;    // Insufficient permissions

// 7xxx: CFM errors
pub const ERR_CFM_NOT_FOUND: i32 = -7001;
pub const ERR_CFM_EXPIRED: i32 = -7002;
pub const ERR_CFM_NO_PERMISSION: i32 = -7003;
pub const ERR_CFM_TOO_LARGE: i32 = -7004;
pub const ERR_CFM_STORE_FULL: i32 = -7005;       // Storage is full
pub const ERR_CFM_FORMAT_INVALID: i32 = -7006;   // File format not supported

/// Get human-readable error message for a given error code.
pub fn error_message(code: i32) -> &'static str {
    match code {
        ERR_OK => "OK",
        ERR_TOKEN_FAILED => "Token validation failed",
        ERR_TOKEN_BLACKLIST => "Token is blacklisted",
        ERR_HEAD_LIMIT => "Head block size exceeded",
        ERR_ID_INVALID => "Invalid message ID",
        ERR_PREFIX_INVALID => "Invalid token prefix or malformed input",
        ERR_TOKEN_USER_MISMATCH => "Token bound to different user",
        ERR_TOKEN_NOT_FOUND => "Token not found",
        ERR_TOKEN_REVOKED => "Token has been revoked",
        ERR_VERSION_INCOMPATIBLE => "Incompatible protocol version",
        ERR_ROUTE_UNREACHABLE => "Route is unreachable or empty",
        ERR_SUBHEAD_LIMIT => "SubHead block size exceeded",
        ERR_ROUTE_INVALID => "Route format invalid",
        ERR_REF_NOT_FOUND => "Reference ID not found",
        ERR_CONTEXT_FORMAT => "Invalid context format",
        ERR_CONTEXT_EXPIRED => "Context reference expired",
        ERR_CHECK_MISMATCH => "Check (MD5) mismatch",
        ERR_CHECK_LENGTH => "Check block length mismatch",
        ERR_CHECK_LENGTH_ILLEGAL => "Check block length is illegal",
        ERR_TIMEOUT => "Operation timeout",
        ERR_STORAGE_FAIL => "Storage operation failed",
        ERR_SERVICE_UNAVAILABLE => "Service unavailable",
        ERR_INTERNAL_ERROR => "Internal server error",
        ERR_RATE_LIMITED => "Rate limit exceeded",
        ERR_ACCESS_DENIED => "Access denied",
        ERR_USERNAME_REQUIRED => "Username required",
        ERR_USER_EXISTS => "User already exists",
        ERR_USER_NOT_FOUND => "User not found",
        ERR_PERMISSION_DENIED => "Insufficient permissions",
        ERR_CFM_NOT_FOUND => "CFM file not found",
        ERR_CFM_EXPIRED => "CFM file has expired",
        ERR_CFM_NO_PERMISSION => "No permission to access CFM file",
        ERR_CFM_TOO_LARGE => "CFM file exceeds size limit",
        ERR_CFM_STORE_FULL => "CFM storage is full",
        ERR_CFM_FORMAT_INVALID => "CFM file format not supported",
        _ => "Unknown error",
    }
}
