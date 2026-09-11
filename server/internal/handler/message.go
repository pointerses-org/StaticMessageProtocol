// Package handler provides connection and message processing.
package handler

import (
	"encoding/binary"
	"fmt"
	"strconv"
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

	// Command policy: base-permission / special-permission hold command globs
	// such as "smp *pull*", and conf-mode decides whether a match grants or
	// denies access. _create is exempt -- it is the unauthenticated bootstrap,
	// so gating it would deadlock the first user out of a token.
	if routeStr != "_create" {
		if !h.authorizeCommand(conn, cacheKey, routeStr, tokenFound, tokenUsername) {
			return
		}
	}

	// Process based on route
	var response []byte
	var err error

	switch routeStr {
	case "_pull":
		if !h.authorizeInbox(conn, cacheKey, tokenUsername,
			parseQuery(string(userData))["route"]) {
			return
		}
		response, err = h.handlePull(string(userData))
	case "_watch":
		// Newest message ID in the caller's own inbox. Answering globally would
		// leak other users' message IDs.
		if !h.authorizeInbox(conn, cacheKey, tokenUsername, "smp@"+tokenUsername) {
			return
		}
		response = []byte(fmt.Sprintf("last_id=0x%x", h.store.LatestID("smp@"+tokenUsername)))
	case "_cfm_upload":
		response, err = h.handleCFMUpload(msgID, userData, tokenUsername)
	case "_cfm_download":
		response, err = h.handleCFMDownload(userData, tokenUsername)
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
		// Every route that gets here is a user inbox, not an internal one -- the
		// internal routes are all named cases above.
		if !h.authorizeInbox(conn, cacheKey, tokenUsername, routeStr) {
			return
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

// routeCommand maps an SMP route to the CLI command it stands for. The config's
// base-permission / special-permission globs match against this string, so
// "smp *pull*" covers _pull and "smp *" covers everything.
func routeCommand(route string) string {
	switch route {
	case "_pull":
		return "smp pull"
	case "_create":
		return "smp create"
	case "_list":
		return "smp list"
	case "_watch":
		return "smp watch-context"
	case "_cfm_upload":
		return "smp cfm upload"
	case "_cfm_download":
		return "smp cfm download"
	case "_token_generate":
		return "smp token generate"
	case "_token_list":
		return "smp token list"
	case "_token_revoke":
		return "smp token revoke"
	default:
		// Anything else is a user inbox address such as smp@alice.
		return "smp push"
	}
}

// requireToken rejects callers whose token tail this server never issued and
// writes the rejection itself. Returns true when the caller is known.
func (h *Handler) requireToken(conn Writer, cacheKey string, tokenFound bool) bool {
	if tokenFound {
		return true
	}
	resp := errorResponse(1007, "token not found")
	h.cache.Put(cacheKey, resp)
	conn.Write(resp)
	return false
}

// authorizeCommand enforces the config's command policy for a route: a known
// token is required, then the mapped command must be allowed by
// base-permission / special-permission under the configured conf-mode.
func (h *Handler) authorizeCommand(conn Writer, cacheKey, route string,
	tokenFound bool, tokenUsername string) bool {
	if !h.requireToken(conn, cacheKey, tokenFound) {
		return false
	}
	cmd := routeCommand(route)
	allowed, why := h.cfg.CommandAllowed(tokenUsername, cmd)
	if allowed {
		return true
	}
	resp := errorResponse(6005, "command denied: "+cmd+" ("+why+")")
	h.cache.Put(cacheKey, resp)
	conn.Write(resp)
	return false
}

// authorizeInbox refuses cross-user inbox access. authorizeCommand has already
// validated the caller's token, so this only compares the target user against
// the one the token is bound to.
func (h *Handler) authorizeInbox(conn Writer, cacheKey, tokenUsername, inbox string) bool {
	if strings.HasPrefix(inbox, "smp@") && strings.TrimPrefix(inbox, "smp@") != tokenUsername {
		resp := errorResponse(1006, "token bound to different user")
		h.cache.Put(cacheKey, resp)
		conn.Write(resp)
		return false
	}
	return true
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

func (h *Handler) handleCFMUpload(msgID uint64, data []byte, uploader string) ([]byte, error) {
	params := parseQuery(string(data))
	minutes := 1440
	fmt.Sscanf(params["expire"], "%d", &minutes)
	public := params["public"] == "true"
	cfID := msgID | (1 << 63)
	_, err := h.cfm.Save(cfID, data, uploader, public, time.Duration(minutes)*time.Minute)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("cfm_id=0x%x", cfID)), nil
}

func (h *Handler) handleCFMDownload(data []byte, requester string) ([]byte, error) {
	params := parseQuery(string(data))
	// IDs travel as "0x<16 hex>". fmt.Sscanf with %d stops at the 'x' and leaves
	// cfID at 0, so every download missed with "CFM not found: 0".
	raw := strings.TrimSpace(params["cfm_id"])
	if strings.HasPrefix(raw, "0x") || strings.HasPrefix(raw, "0X") {
		raw = raw[2:]
	}
	cfID, err := strconv.ParseUint(raw, 16, 64)
	if err != nil {
		cfID, _ = strconv.ParseUint(raw, 10, 64)
	}
	return h.cfm.Load(cfID, requester)
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
	// smp_error_message NUL-terminates the buffer (core/src/ffi.rs), so use the
	// returned byte count. GoStringN(ptr, -1) is read as 0xFFFFFFFF on x86 and
	// attempts a 4 GiB allocation, which OOMs and kills the whole process.
	n := C.smp_error_message(C.int32_t(errCode), (*C.uint8_t)(unsafe.Pointer(&buf[0])), C.size_t(len(buf)))
	if n <= 0 {
		conn.Write([]byte(fmt.Sprintf("error:%d:unknown error", errCode)))
		return
	}
	conn.Write([]byte(fmt.Sprintf("error:%d:%s", errCode, string(buf[:n]))))
}

// errorResponse creates a standard error response.
func errorResponse(code int, detail string) []byte {
	return []byte(fmt.Sprintf("error:%d:%s", code, detail))
}
