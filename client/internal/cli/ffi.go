package cli

import (
    "fmt"
    "strings"
    "unsafe"
)

/*
    #cgo CFLAGS: -I${SRCDIR}/../../../core/include
    #cgo LDFLAGS: -L${SRCDIR}/../../../core/target/release -lsmp_core
    typedef __builtin_va_list __gnuc_va_list;
    #include <stdlib.h>
    #include "smp.h"
*/
import "C"

// ContextPair represents a context reference.
type ContextPair struct {
    RefID     uint64
    Timestamp uint32
}

// AssembleMessage builds a complete SMP message.
func AssembleMessage(
    tokenTail string, msgID uint64, route string,
    userData []byte, contextPairs []ContextPair, extensions string,
) ([]byte, error) {
    version := C.CString("smp/0.1b")
    defer C.free(unsafe.Pointer(version))

    localIP := localAddr()
    clientAddr := C.CString(localIP)
    defer C.free(unsafe.Pointer(clientAddr))

    tailBytes := []byte(tokenTail)
    routeBytes := []byte(route)
    extBytes := []byte(extensions)

    var refs *C.uint64_t
    var tsPtr *C.uint32_t
    ctxCount := C.size_t(0)

    if len(contextPairs) > 0 {
        ctxCount = C.size_t(len(contextPairs))
        refsSlice := make([]uint64, ctxCount)
        tsSlice := make([]uint32, ctxCount)
        for i, cp := range contextPairs {
            refsSlice[i] = cp.RefID
            tsSlice[i] = cp.Timestamp
        }
        refs = (*C.uint64_t)(unsafe.Pointer(&refsSlice[0]))
        tsPtr = (*C.uint32_t)(unsafe.Pointer(&tsSlice[0]))
        _ = refsSlice
        _ = tsSlice
    }

    var dataPtr *C.uint8_t
    if len(userData) > 0 {
        dataPtr = (*C.uint8_t)(unsafe.Pointer(&userData[0]))
    }

    var outLen C.size_t
    ptr := C.smp_assemble_message(
        (*C.uint8_t)(unsafe.Pointer(&tailBytes[0])), C.size_t(len(tailBytes)),
        C.uint64_t(msgID),
        (*C.uint8_t)(unsafe.Pointer(version)), C.size_t(len("smp/0.1b")),
        (*C.uint8_t)(unsafe.Pointer(clientAddr)), C.size_t(len(localIP)),
        (*C.uint8_t)(unsafe.Pointer(&routeBytes[0])), C.size_t(len(routeBytes)),
        (*C.uint8_t)(unsafe.Pointer(&extBytes[0])), C.size_t(len(extBytes)),
        ctxCount, refs, tsPtr,
        dataPtr, C.size_t(len(userData)),
        &outLen,
    )

    if ptr == nil {
        return nil, fmt.Errorf("smp_assemble_message failed")
    }
    defer C.smp_free_buffer(ptr, outLen)

    return C.GoBytes(unsafe.Pointer(ptr), C.int(outLen)), nil
}

// ExtractTokenTail extracts the tail from a full token.
func ExtractTokenTail(fullToken string) string {
    if fullToken == "" {
        return ""
    }
    if !strings.HasPrefix(fullToken, "smpt128-") && !strings.HasPrefix(fullToken, "smpt256-") {
        return fullToken
    }
    tail := make([]byte, 16)
    tokenBytes := []byte(fullToken)
    tailLen := int(C.smp_extract_token_tail(
        (*C.uint8_t)(unsafe.Pointer(&tokenBytes[0])), C.size_t(len(tokenBytes)),
        (*C.uint8_t)(unsafe.Pointer(&tail[0])), C.size_t(len(tail)),
    ))
    if tailLen > 0 {
        return string(tail[:tailLen])
    }
    return ""
}
