// Package handler provides connection and message processing.
package handler

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"smp-server/internal/auth"
	"smp-server/internal/cache"
	"smp-server/internal/config"
	"smp-server/internal/store"

	/*
		#cgo CFLAGS: -I${SRCDIR}/../../../core/include
		#cgo LDFLAGS: -L${SRCDIR}/../../../core/target/release -lsmp_core
		typedef __builtin_va_list __gnuc_va_list;
		#include <stdlib.h>
		#include "smp.h"
	*/
	"C"
)

// Handler processes parsed messages.
type Handler struct {
	cfg       config.Config
	store     *store.MessageStore
	cfm       *store.CFMStore
	cache     *cache.IdempotencyCache
	tokens    *auth.TokenStore
	accounts  *store.AccountStore
}

// NewHandler creates a message handler.
func NewHandler(cfg config.Config, msgStore *store.MessageStore, cfmStore *store.CFMStore,
	idemCache *cache.IdempotencyCache, tokenStore *auth.TokenStore, accountStore *store.AccountStore) *Handler {
	return &Handler{
		cfg:       cfg,
		store:     msgStore,
		cfm:       cfmStore,
		cache:     idemCache,
		tokens:    tokenStore,
		accounts:  accountStore,
	}
}

// ProcessParsedMessage extracts fields from the parsed parser and handles the message.
func (h *Handler) ProcessParsedMessage(handle *C.SmpParser, conn Writer) {
	// Extract fields
	msgID := uint64(C.smp_parser_get_msg_id(handle))

	tokenBuf := make([]byte, 16)
	tokenLen := int(C.smp_parser_get_token_tail(handle, (*C.uint8_t)(unsafe.Pointer(&tokenBuf[0])), C.size_t(len(tokenBuf))))
	tokenTail := string(tokenBuf[:tokenLen])

	routeBuf := make([]byte, 512)
	routeLen := int(C.smp_parser_get_route(handle, (*C.uint8_t)(unsafe.Pointer(&routeBuf[0])), C.size_t(len(routeBuf))))
	routeStr := string(routeBuf[:routeLen])

	userDataBuf := make([]byte, 131072)
	userDataLen := int(C.smp_parser_get_user_data(handle, (*C.uint8_t)(unsafe.Pointer(&userDataBuf[0])), C.size_t(len(userDataBuf))))
	userData := userDataBuf[:userDataLen]

	ctxCount := int(C.smp_parser_get_context_count(handle))
	var ctxRefs []uint64
	var ctxTsArr []uint32
	if ctxCount > 0 {
		ctxRefs = make([]uint64, ctxCount)
		ctxTsArr = make([]uint32, ctxCount)
		C.smp_parser_get_context(handle,
			(*C.uint64_t)(unsafe.Pointer(&ctxRefs[0])),
			(*C.uint32_t)(unsafe.Pointer(&ctxTsArr[0])),
			C.size_t(ctxCount))
	}

	// Look up token to get bound username (if any)
	_, tokenUsername, tokenFound := h.tokens.Lookup(tokenTail)

	// Check idempotency
	cacheKey := h.cache.Key(msgID, []byte(tokenTail))
	if cached, ok := h.cache.Get(cacheKey); ok {
		conn.Write(cached)
		return
	}

	// Process based on route
	var response []byte
	var err error

	switch routeStr {
	case "_pull":
		response, err = h.handlePull(string(userData))
	case "_cfm_upload":
		response, err = h.handleCFMUpload(msgID, userData)
	case "_cfm_download":
		response, err = h.handleCFMDownload(userData)
	case "_token_generate":
		params := parseQuery(string(userData))
		username := strings.TrimSpace(params["user"])
		if username == "" {
			response = errorResponse(6002, "username required")
		} else {
			// Check access control
			if !h.cfg.HasAccess(username) {
				response = errorResponse(6001, "access denied")
			} else {
				tok := h.tokens.Generate(username)
				response = []byte(tok)
			}
		}
	case "_token_list":
		tails := h.tokens.List()
		response = []byte(fmt.Sprintf("%v", tails))
	case "_token_revoke":
		tail := strings.TrimSpace(string(userData))
		if h.tokens.Revoke(tail) {
			response = []byte("revoked")
		} else {
			response = errorResponse(1007, "token not found")
		}
	case "_create":
		params := parseQuery(string(userData))
		username := strings.TrimSpace(params["user"])
		if username == "" {
			response = errorResponse(6002, "username required")
		} else {
			// Check access control
			if !h.cfg.HasAccess(username) {
				response = errorResponse(6001, "access denied")
			} else {
				// Check if user already exists
				if _, exists := h.accounts.Get(username); exists {
					response = errorResponse(6003, "user already exists")
				} else {
					// Generate token bound to this username
					tok := h.tokens.Generate(username)
					perms := h.cfg.GetPermissions(username)
					acc := h.accounts.Create(username, perms)
					response = []byte(fmt.Sprintf("ok:%s:token=%s:permissions=%v",
						acc.Username, tok, acc.Permissions))
				}
			}
		}
	case "_list":
		accounts := h.accounts.List()
		var buf []byte
		buf = binary.BigEndian.AppendUint16(buf, uint16(len(accounts)))
		for _, acc := range accounts {
			name := []byte(acc.Username)
			buf = binary.BigEndian.AppendUint16(buf, uint16(len(name)))
			buf = append(buf, name...)
			buf = binary.BigEndian.AppendUint16(buf, uint16(len(acc.Permissions)))
			for _, p := range acc.Permissions {
				pb := []byte(p)
				buf = binary.BigEndian.AppendUint16(buf, uint16(len(pb)))
				buf = append(buf, pb...)
			}
		}
		response = buf
	default:
		// Validate token-user binding for push messages
		if tokenFound && strings.HasPrefix(routeStr, "smp@") {
			routeUser := strings.TrimPrefix(routeStr, "smp@")
			if routeUser != tokenUsername {
				response = errorResponse(1006, "token bound to different user")
				h.cache.Put(cacheKey, response)
				conn.Write(response)
				return
			}
		}

		ctxPairs := make([]store.ContextPair, ctxCount)
		for i := 0; i < ctxCount; i++ {
			ctxPairs[i] = store.ContextPair{RefID: ctxRefs[i], Timestamp: ctxTsArr[i]}
		}
		msg := &store.StoredMessage{
			ID: msgID, Route: routeStr, UserData: userData,
			Context: ctxPairs, TokenTail: tokenTail, Timestamp: time.Now(),
		}
		h.store.Store(msg)
		response = []byte(fmt.Sprintf("ok:%x", msgID))
	}

	if err != nil {
		response = []byte(fmt.Sprintf("error:5002:%s", err.Error()))
	}

	h.cache.Put(cacheKey, response)
	conn.Write(response)
}

func (h *Handler) handlePull(query string) ([]byte, error) {
	params := parseQuery(query)
	limit := 10
	fmt.Sscanf(params["limit"], "%d", &limit)
	offset := 0
	fmt.Sscanf(params["offset"], "%d", &offset)
	var after, before uint32
	fmt.Sscanf(params["after"], "%d", &after)
	fmt.Sscanf(params["before"], "%d", &before)
	var msgID uint64
	fmt.Sscanf(params["id"], "%d", &msgID)

	msgs := h.store.Query(params["route"], limit, offset, after, before, msgID)

	var buf []byte
	buf = binary.BigEndian.AppendUint16(buf, uint16(len(msgs)))
	for _, msg := range msgs {
		msgBuf := serializeMessage(msg)
		buf = binary.BigEndian.AppendUint32(buf, uint32(len(msgBuf)))
		buf = append(buf, msgBuf...)
	}
	return buf, nil
}

func (h *Handler) handleCFMUpload(msgID uint64, data []byte) ([]byte, error) {
	params := parseQuery(string(data))
	minutes := 1440
	fmt.Sscanf(params["expire"], "%d", &minutes)
	public := params["public"] == "true"
	cfID := msgID | (1 << 63)
	_, err := h.cfm.Save(cfID, data, "", public, time.Duration(minutes)*time.Minute)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("cfm_id=0x%x", cfID)), nil
}

func (h *Handler) handleCFMDownload(data []byte) ([]byte, error) {
	params := parseQuery(string(data))
	var cfID uint64
	fmt.Sscanf(params["cfm_id"], "%d", &cfID)
	return h.cfm.Load(cfID, "")
}

func serializeMessage(msg *store.StoredMessage) []byte {
	var buf []byte
	buf = binary.BigEndian.AppendUint64(buf, msg.ID)
	rb := []byte(msg.Route)
	buf = binary.BigEndian.AppendUint16(buf, uint16(len(rb)))
	buf = append(buf, rb...)
	buf = binary.BigEndian.AppendUint32(buf, uint32(len(msg.UserData)))
	buf = append(buf, msg.UserData...)
	buf = binary.BigEndian.AppendUint16(buf, uint16(len(msg.Context)))
	for _, ctx := range msg.Context {
		buf = binary.BigEndian.AppendUint64(buf, ctx.RefID)
		buf = binary.BigEndian.AppendUint32(buf, ctx.Timestamp)
	}
	return buf
}

func parseQuery(q string) map[string]string {
	params := make(map[string]string)
	for _, pair := range strings.Split(q, "&") {
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			params[parts[0]] = parts[1]
		}
	}
	return params
}

// Writer is the interface for writing responses.
type Writer interface {
	Write([]byte) (int, error)
}

// SendError writes an error response using the standard format.
func SendError(conn Writer, errCode int32) {
	var buf [256]byte
	C.smp_error_message(C.int32_t(errCode), (*C.uint8_t)(unsafe.Pointer(&buf[0])), C.size_t(len(buf)))
	msg := C.GoStringN((*C.char)(unsafe.Pointer(&buf[0])), -1)
	conn.Write([]byte(fmt.Sprintf("error:%d:%s", errCode, msg)))
}

// errorResponse creates a standard error response.
func errorResponse(code int, detail string) []byte {
	return []byte(fmt.Sprintf("error:%d:%s", code, detail))
}
