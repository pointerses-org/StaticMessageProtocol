package cli

import (
    "fmt"
    "os"
    "strconv"
    "strings"
)

// Error codes matching the Rust core.
const (
    ErrOK = 0

    // 1xxx: Head errors
    ErrTokenFailed        = 1001
    ErrTokenBlacklist     = 1002
    ErrHeadLimit          = 1003
    ErrIDInvalid          = 1004
    ErrPrefixInvalid      = 1005
    ErrTokenUserMismatch  = 1006
    ErrTokenNotFound      = 1007
    ErrTokenRevoked       = 1008

    // 2xxx: SubHead errors
    ErrVersionIncompatible = 2001
    ErrRouteUnreachable    = 2002
    ErrSubheadLimit        = 2003
    ErrRouteInvalid        = 2004

    // 3xxx: Context errors
    ErrRefNotFound     = 3001
    ErrContextFormat   = 3003
    ErrContextExpired  = 3004

    // 4xxx: Check errors
    ErrCheckMismatch      = 4001
    ErrCheckLength        = 4002
    ErrCheckLengthIllegal = 4003

    // 5xxx: Internal errors
    ErrTimeout       = 5001
    ErrStorageFail   = 5002
    ErrServiceUnavailable = 5003
    ErrInternalError = 5004
    ErrRateLimited   = 5005

    // 6xxx: Authentication errors
    ErrAccessDenied       = 6001
    ErrUsernameRequired   = 6002
    ErrUserExists         = 6003
    ErrUserNotFound       = 6004
    ErrPermissionDenied   = 6005

    // 7xxx: CFM errors
    ErrCFMNotFound      = 7001
    ErrCFMExpired       = 7002
    ErrCFMNoPermission  = 7003
    ErrCFMTooLarge      = 7004
    ErrCFMStoreFull     = 7005
    ErrCFMFormatInvalid = 7006
)

// ErrorMessage returns a human-readable error message for a code.
func ErrorMessage(code int) string {
    switch code {
    case ErrOK:
        return "OK"
    case ErrTokenFailed:
        return "Token validation failed"
    case ErrTokenBlacklist:
        return "Token is blacklisted"
    case ErrHeadLimit:
        return "Head block size exceeded"
    case ErrIDInvalid:
        return "Invalid message ID"
    case ErrPrefixInvalid:
        return "Invalid token prefix or malformed input"
    case ErrTokenUserMismatch:
        return "Token bound to different user"
    case ErrTokenNotFound:
        return "Token not found"
    case ErrTokenRevoked:
        return "Token has been revoked"
    case ErrVersionIncompatible:
        return "Incompatible protocol version"
    case ErrRouteUnreachable:
        return "Route is unreachable or empty"
    case ErrSubheadLimit:
        return "SubHead block size exceeded"
    case ErrRouteInvalid:
        return "Route format invalid"
    case ErrRefNotFound:
        return "Reference ID not found"
    case ErrContextFormat:
        return "Invalid context format"
    case ErrContextExpired:
        return "Context reference expired"
    case ErrCheckMismatch:
        return "Check (MD5) mismatch"
    case ErrCheckLength:
        return "Check block length mismatch"
    case ErrCheckLengthIllegal:
        return "Check block length is illegal"
    case ErrTimeout:
        return "Operation timeout"
    case ErrStorageFail:
        return "Storage operation failed"
    case ErrServiceUnavailable:
        return "Service unavailable"
    case ErrInternalError:
        return "Internal server error"
    case ErrRateLimited:
        return "Rate limit exceeded"
    case ErrAccessDenied:
        return "Access denied"
    case ErrUsernameRequired:
        return "Username required"
    case ErrUserExists:
        return "User already exists"
    case ErrUserNotFound:
        return "User not found"
    case ErrPermissionDenied:
        return "Insufficient permissions"
    case ErrCFMNotFound:
        return "CFM file not found"
    case ErrCFMExpired:
        return "CFM file has expired"
    case ErrCFMNoPermission:
        return "No permission to access CFM file"
    case ErrCFMTooLarge:
        return "CFM file exceeds size limit"
    case ErrCFMStoreFull:
        return "CFM storage is full"
    case ErrCFMFormatInvalid:
        return "CFM file format not supported"
    default:
        return fmt.Sprintf("Unknown error (%d)", code)
    }
}

// ParseResponse parses a server response and returns (isError, code, message).
func ParseResponse(response []byte) (bool, int, string) {
    resp := string(response)
    if strings.HasPrefix(resp, "error:") {
        parts := strings.SplitN(resp, ":", 3)
        if len(parts) >= 2 {
            code, err := strconv.Atoi(parts[1])
            if err == nil {
                detail := ""
                if len(parts) >= 3 {
                    detail = parts[2]
                }
                return true, code, detail
            }
        }
        return true, 0, resp
    }
    return false, 0, resp
}

// DisplayResponse displays a server response with error handling.
func DisplayResponse(response []byte) {
    isError, code, detail := ParseResponse(response)
    if isError {
        msg := ErrorMessage(code)
        if detail != "" {
            fmt.Fprintf(os.Stderr, "Error %d: %s (%s)\n", code, msg, detail)
        } else {
            fmt.Fprintf(os.Stderr, "Error %d: %s\n", code, msg)
        }
    } else {
        fmt.Println(string(response))
    }
}
