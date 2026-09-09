//! Common types and protocol constants.

use std::os::raw::c_void;

// Protocol constants
pub const PROTOCOL_VERSION: &str = "smp/0.1b";
pub const MAX_HEAD_SIZE: usize = 130 * 1024;
pub const MAX_SUBHEAD_SIZE: usize = 64 * 1024;
pub const MAX_USER_DATA_SIZE: usize = 130 * 1024;
pub const CHECK_SIZE: usize = 16;
pub const TOKEN_TAIL_LEN: usize = 8;
pub const MSG_ID_SIZE: usize = 8;
pub const REF_ID_SIZE: usize = 8;
pub const TIMESTAMP_SIZE: usize = 4;
pub const CFM_ID_FLAG: u64 = 1u64 << 63;

// Stage constants
pub const STAGE_HEAD: u32 = 0;
pub const STAGE_SUBHEAD: u32 = 1;
pub const STAGE_CONTEXT: u32 = 2;
pub const STAGE_USER_DATA: u32 = 3;
pub const STAGE_CHECK: u32 = 4;
pub const STAGE_DONE: u32 = 5;
pub const STAGE_ERROR: u32 = 6;

// Callback types
pub type TokenCallback = extern "C" fn(
    _ctx: *mut c_void,
    token_tail: *const u8,
    token_len: usize,
) -> i32;

pub type RouteCallback = extern "C" fn(
    _ctx: *mut c_void,
    route: *const u8,
    route_len: usize,
) -> i32;

pub type RefCallback = extern "C" fn(
    _ctx: *mut c_void,
    ref_id: u64,
    timestamp: u32,
) -> i32;

pub type MessageCallback = extern "C" fn(
    _ctx: *mut c_void,
    msg_id: u64,
    route: *const u8,
    route_len: usize,
    user_data: *const u8,
    user_data_len: usize,
    context_count: usize,
    refs: *const u64,
    timestamps: *const u32,
) -> i32;

#[repr(C)]
pub struct Callbacks {
    pub token_cb: Option<TokenCallback>,
    pub route_cb: Option<RouteCallback>,
    pub ref_cb: Option<RefCallback>,
    pub message_cb: Option<MessageCallback>,
    pub ctx: *mut c_void,
}

impl Callbacks {
    pub fn empty() -> Self {
        Callbacks {
            token_cb: None,
            route_cb: None,
            ref_cb: None,
            message_cb: None,
            ctx: std::ptr::null_mut(),
        }
    }
}

/// Parsed Head block data.
pub struct HeadData {
    pub token_tail: Vec<u8>,
    pub user_data_len: u32,
    pub msg_id: u64,
    pub end_offset: usize,
}

/// Parsed SubHead block data.
pub struct SubHeadData {
    pub version: Vec<u8>,
    pub client_addr: Vec<u8>,
    pub route: Vec<u8>,
    pub extensions_raw: Vec<u8>,
    pub extensions: Vec<(Vec<u8>, Vec<u8>)>,
    pub end_offset: usize,
}

/// Parsed Context block data.
pub struct ContextData {
    pub pairs: Vec<(u64, u32)>,
    pub end_offset: usize,
}
