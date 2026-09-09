//! C-compatible FFI exports for the SMP core library.

use crate::error::*;
use crate::md5::md5_bytes;
use crate::parser::SmpParser;
use crate::types::*;

// ---- Parser Lifecycle ----

#[no_mangle]
pub extern "C" fn smp_parser_new() -> *mut SmpParser {
    Box::into_raw(Box::new(SmpParser::new()))
}

#[no_mangle]
pub extern "C" fn smp_parser_free(handle: *mut SmpParser) {
    if !handle.is_null() {
        unsafe { drop(Box::from_raw(handle)); }
    }
}

#[no_mangle]
pub extern "C" fn smp_parser_feed(handle: *mut SmpParser, data: *const u8, len: usize) -> i32 {
    if handle.is_null() || data.is_null() {
        return ERR_PREFIX_INVALID;
    }
    let parser = unsafe { &mut *handle };
    let slice = unsafe { std::slice::from_raw_parts(data, len) };
    parser.feed(slice)
}

#[no_mangle]
pub extern "C" fn smp_parser_state(handle: *mut SmpParser) -> u32 {
    if handle.is_null() { return STAGE_ERROR; }
    unsafe { &*handle }.state()
}

#[no_mangle]
pub extern "C" fn smp_parser_error(handle: *mut SmpParser) -> i32 {
    if handle.is_null() { return ERR_OK; }
    unsafe { &*handle }.error()
}

#[no_mangle]
pub extern "C" fn smp_parser_reset(handle: *mut SmpParser) {
    if handle.is_null() { return; }
    unsafe { &mut *handle }.reset();
}

#[no_mangle]
pub extern "C" fn smp_parser_set_callbacks(handle: *mut SmpParser, cbs: *const Callbacks) {
    if handle.is_null() || cbs.is_null() { return; }
    unsafe {
        let cbs = &*cbs;
        (&mut *handle).set_callbacks(Callbacks {
            token_cb: cbs.token_cb,
            route_cb: cbs.route_cb,
            ref_cb: cbs.ref_cb,
            message_cb: cbs.message_cb,
            ctx: cbs.ctx,
        });
    }
}

// ---- Message Assembly ----

#[no_mangle]
pub extern "C" fn smp_assemble_message(
    token_tail: *const u8, token_len: usize,
    msg_id: u64,
    version: *const u8, version_len: usize,
    client_addr: *const u8, client_addr_len: usize,
    route: *const u8, route_len: usize,
    extensions: *const u8, extensions_len: usize,
    context_count: usize,
    refs: *const u64,
    timestamps: *const u32,
    user_data: *const u8, user_data_len: usize,
    out_len: *mut usize,
) -> *mut u8 {
    if (token_tail.is_null() && token_len > 0)
        || (version.is_null() && version_len > 0)
        || (client_addr.is_null() && client_addr_len > 0)
        || (route.is_null() && route_len > 0)
        || (extensions.is_null() && extensions_len > 0)
        || (user_data.is_null() && user_data_len > 0)
    {
        return std::ptr::null_mut();
    }

    let token_tail_slice = if token_len > 0 {
        unsafe { std::slice::from_raw_parts(token_tail, token_len) }
    } else { &[] };
    let version_slice = if version_len > 0 {
        unsafe { std::slice::from_raw_parts(version, version_len) }
    } else { &[] };
    let client_addr_slice = if client_addr_len > 0 {
        unsafe { std::slice::from_raw_parts(client_addr, client_addr_len) }
    } else { &[] };
    let route_slice = if route_len > 0 {
        unsafe { std::slice::from_raw_parts(route, route_len) }
    } else { &[] };
    let extensions_slice = if extensions_len > 0 {
        unsafe { std::slice::from_raw_parts(extensions, extensions_len) }
    } else { &[] };
    let user_data_slice = if user_data_len > 0 {
        unsafe { std::slice::from_raw_parts(user_data, user_data_len) }
    } else { &[] };

    if msg_id == 0 { return std::ptr::null_mut(); }

    let mut context_pairs = Vec::with_capacity(context_count);
    if context_count > 0 {
        if refs.is_null() || timestamps.is_null() { return std::ptr::null_mut(); }
        for i in 0..context_count {
            let ref_id = unsafe { *refs.add(i) };
            let ts = unsafe { *timestamps.add(i) };
            context_pairs.push((ref_id, ts));
        }
    }

    let message = crate::assembler::assemble_message(
        token_tail_slice, msg_id, version_slice, client_addr_slice,
        route_slice, extensions_slice, &context_pairs, user_data_slice,
    );

    if !out_len.is_null() {
        unsafe { *out_len = message.len(); }
    }

    let mut out = Vec::with_capacity(message.len());
    out.extend_from_slice(&message);
    let ptr = out.as_mut_ptr();
    std::mem::forget(out);
    ptr
}

#[no_mangle]
pub extern "C" fn smp_free_buffer(buf: *mut u8, len: usize) {
    if buf.is_null() || len == 0 { return; }
    unsafe {
        let slice = std::slice::from_raw_parts_mut(buf, len);
        let vec = Vec::from_raw_parts(slice.as_mut_ptr(), len, len);
        drop(vec);
    }
}

// ---- MD5 & Check Validation ----

#[no_mangle]
pub extern "C" fn smp_md5(data: *const u8, len: usize, out: *mut u8) {
    if data.is_null() || out.is_null() || len == 0 { return; }
    let slice = unsafe { std::slice::from_raw_parts(data, len) };
    let hash = md5_bytes(slice);
    unsafe { std::ptr::copy_nonoverlapping(hash.as_ptr(), out, CHECK_SIZE); }
}

#[no_mangle]
pub extern "C" fn smp_validate_check(data: *const u8, len: usize) -> i32 {
    if data.is_null() || len < CHECK_SIZE { return ERR_CHECK_LENGTH_ILLEGAL; }
    let slice = unsafe { std::slice::from_raw_parts(data, len) };
    crate::assembler::validate_check(slice)
}

// ---- Token Utilities ----

#[no_mangle]
pub extern "C" fn smp_extract_token_tail(
    token: *const u8, token_len: usize,
    out_buf: *mut u8, out_buf_size: usize,
) -> i32 {
    if token.is_null() || out_buf.is_null() || out_buf_size < TOKEN_TAIL_LEN { return -1; }
    let token_slice = unsafe { std::slice::from_raw_parts(token, token_len) };
    match crate::token::extract_token_tail(token_slice) {
        Some(tail) => {
            unsafe { std::ptr::copy_nonoverlapping(tail.as_ptr(), out_buf, tail.len()); }
            tail.len() as i32
        }
        None => -1,
    }
}

#[no_mangle]
pub extern "C" fn smp_validate_token(token: *const u8, token_len: usize) -> i32 {
    if token.is_null() || token_len == 0 { return ERR_PREFIX_INVALID; }
    let token_slice = unsafe { std::slice::from_raw_parts(token, token_len) };
    if crate::token::is_valid_token(token_slice) { ERR_OK } else { ERR_PREFIX_INVALID }
}

#[no_mangle]
pub extern "C" fn smp_is_cfm_id(msg_id: u64) -> bool {
    msg_id & CFM_ID_FLAG != 0
}

#[no_mangle]
pub extern "C" fn smp_error_message(code: i32, out_buf: *mut u8, out_buf_size: usize) -> i32 {
    if out_buf.is_null() || out_buf_size == 0 { return 0; }
    let msg = error_message(code);
    let msg_bytes = msg.as_bytes();
    let copy_len = std::cmp::min(msg_bytes.len(), out_buf_size - 1);
    unsafe {
        std::ptr::copy_nonoverlapping(msg_bytes.as_ptr(), out_buf, copy_len);
        *out_buf.add(copy_len) = 0;
    }
    copy_len as i32
}

// ---- Parser Field Extraction ----

#[no_mangle]
pub extern "C" fn smp_parser_get_msg_id(handle: *mut SmpParser) -> u64 {
    if handle.is_null() { return 0; }
    let parser = unsafe { &*handle };
    parser.head.as_ref().map(|h| h.msg_id).unwrap_or(0)
}

#[no_mangle]
pub extern "C" fn smp_parser_get_token_tail(
    handle: *mut SmpParser, out_buf: *mut u8, out_buf_size: usize,
) -> i32 {
    if handle.is_null() || out_buf.is_null() || out_buf_size == 0 { return 0; }
    let parser = unsafe { &*handle };
    if let Some(head) = &parser.head {
        let copy_len = std::cmp::min(head.token_tail.len(), out_buf_size);
        unsafe { std::ptr::copy_nonoverlapping(head.token_tail.as_ptr(), out_buf, copy_len); }
        copy_len as i32
    } else { 0 }
}

#[no_mangle]
pub extern "C" fn smp_parser_get_route(
    handle: *mut SmpParser, out_buf: *mut u8, out_buf_size: usize,
) -> i32 {
    if handle.is_null() || out_buf.is_null() || out_buf_size == 0 { return 0; }
    let parser = unsafe { &*handle };
    if let Some(sub) = &parser.subhead {
        let copy_len = std::cmp::min(sub.route.len(), out_buf_size);
        unsafe { std::ptr::copy_nonoverlapping(sub.route.as_ptr(), out_buf, copy_len); }
        copy_len as i32
    } else { 0 }
}

#[no_mangle]
pub extern "C" fn smp_parser_get_user_data(
    handle: *mut SmpParser, out_buf: *mut u8, out_buf_size: usize,
) -> i32 {
    if handle.is_null() || out_buf.is_null() || out_buf_size == 0 { return 0; }
    let parser = unsafe { &*handle };
    let copy_len = std::cmp::min(parser.user_data.len(), out_buf_size);
    if copy_len > 0 {
        unsafe { std::ptr::copy_nonoverlapping(parser.user_data.as_ptr(), out_buf, copy_len); }
    }
    copy_len as i32
}

#[no_mangle]
pub extern "C" fn smp_parser_get_context(
    handle: *mut SmpParser, out_refs: *mut u64, out_ts: *mut u32, max_pairs: usize,
) -> i32 {
    if handle.is_null() || out_refs.is_null() || out_ts.is_null() || max_pairs == 0 { return 0; }
    let parser = unsafe { &*handle };
    if let Some(ctx) = &parser.context {
        let count = std::cmp::min(ctx.pairs.len(), max_pairs);
        for i in 0..count {
            unsafe {
                *out_refs.add(i) = ctx.pairs[i].0;
                *out_ts.add(i) = ctx.pairs[i].1;
            }
        }
        count as i32
    } else { 0 }
}

#[no_mangle]
pub extern "C" fn smp_parser_get_context_count(handle: *mut SmpParser) -> i32 {
    if handle.is_null() { return 0; }
    let parser = unsafe { &*handle };
    parser.context.as_ref().map(|c| c.pairs.len() as i32).unwrap_or(0)
}
