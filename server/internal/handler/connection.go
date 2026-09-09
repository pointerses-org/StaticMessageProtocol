package handler

import (
	"log"
	"net"
	"time"
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

// HandleConnection processes messages from a single TCP connection.
func HandleConnection(conn net.Conn, handler *Handler, verbose bool) {
	defer conn.Close()

	remote := conn.RemoteAddr().String()
	if verbose {
		log.Printf("Conn: %s", remote)
	}

	handle := C.smp_parser_new()
	defer C.smp_parser_free(handle)

	buf := make([]byte, 262144)
	for {
		conn.SetReadDeadline(getDeadline())
		n, err := conn.Read(buf)
		if err != nil {
			if verbose {
				log.Printf("Read %s: %v", remote, err)
			}
			return
		}
		if n == 0 {
			continue
		}

		result := C.smp_parser_feed(handle, (*C.uint8_t)(unsafe.Pointer(&buf[0])), C.size_t(n))
		if result < 0 {
			SendError(conn, int32(result))
			return
		}

		state := C.smp_parser_state(handle)
		if state == C.SMP_STAGE_DONE {
			handler.ProcessParsedMessage(handle, conn)
			C.smp_parser_reset(handle)
		}
	}
}

func getDeadline() time.Time {
	return time.Now().Add(30 * time.Second)
}
