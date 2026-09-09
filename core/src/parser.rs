//! SMP message parser — 5-stage streaming state machine.

use crate::error::*;
use crate::md5::md5_bytes;
use crate::types::*;

/// Streaming SMP message parser with per-stage callback support.
pub struct SmpParser {
    pub stage: u32,
    pub buffer: Vec<u8>,
    pub head: Option<HeadData>,
    pub subhead: Option<SubHeadData>,
    pub context: Option<ContextData>,
    pub user_data: Vec<u8>,
    expected_user_data_len: usize,
    error_code: i32,
    callbacks: Callbacks,
}

impl SmpParser {
    pub fn new() -> Self {
        SmpParser {
            stage: STAGE_HEAD,
            buffer: Vec::with_capacity(4096),
            head: None,
            subhead: None,
            context: None,
            user_data: Vec::new(),
            expected_user_data_len: 0,
            error_code: ERR_OK,
            callbacks: Callbacks::empty(),
        }
    }

    pub fn set_callbacks(&mut self, cbs: Callbacks) {
        self.callbacks = cbs;
    }

    /// Feed bytes into the parser. Returns ERR_OK if parsing continues,
    /// or a negative error code if a stage failed.
    pub fn feed(&mut self, data: &[u8]) -> i32 {
        if self.stage == STAGE_ERROR {
            return self.error_code;
        }
        if self.stage == STAGE_DONE {
            return ERR_OK;
        }
        self.buffer.extend_from_slice(data);
        self.try_parse()
    }

    pub fn state(&self) -> u32 {
        self.stage
    }

    pub fn error(&self) -> i32 {
        self.error_code
    }

    pub fn reset(&mut self) {
        self.stage = STAGE_HEAD;
        self.buffer.clear();
        self.head = None;
        self.subhead = None;
        self.context = None;
        self.user_data.clear();
        self.expected_user_data_len = 0;
        self.error_code = ERR_OK;
    }

    fn try_parse(&mut self) -> i32 {
        loop {
            let result = match self.stage {
                STAGE_HEAD => self.parse_head(),
                STAGE_SUBHEAD => self.parse_subhead(),
                STAGE_CONTEXT => self.parse_context(),
                STAGE_USER_DATA => self.parse_user_data(),
                STAGE_CHECK => self.parse_check(),
                _ => ParseResult::Complete,
            };
            match result {
                ParseResult::Complete => match self.stage {
                    STAGE_HEAD => self.stage = STAGE_SUBHEAD,
                    STAGE_SUBHEAD => self.stage = STAGE_CONTEXT,
                    STAGE_CONTEXT => self.stage = STAGE_USER_DATA,
                    STAGE_USER_DATA => self.stage = STAGE_CHECK,
                    STAGE_CHECK => {
                        self.stage = STAGE_DONE;
                        return ERR_OK;
                    }
                    _ => return ERR_OK,
                },
                ParseResult::NeedMore => return ERR_OK,
                ParseResult::Error(code) => {
                    self.stage = STAGE_ERROR;
                    self.error_code = code;
                    return code;
                }
            }
        }
    }

    // ---- Stage parsers ----

    fn parse_head(&mut self) -> ParseResult {
        let buf = &self.buffer;
        if buf.len() < 2 {
            return ParseResult::NeedMore;
        }
        let token_len = u16::from_be_bytes([buf[0], buf[1]]) as usize;

        if token_len == 0 {
            return ParseResult::Error(ERR_TOKEN_FAILED);
        }
        if token_len > TOKEN_TAIL_LEN {
            return ParseResult::Error(ERR_HEAD_LIMIT);
        }

        let total_needed = 2 + token_len + 4 + MSG_ID_SIZE;
        if buf.len() < total_needed {
            return ParseResult::NeedMore;
        }

        let token_tail = buf[2..2 + token_len].to_vec();
        let user_data_len = u32::from_be_bytes(
            buf[2 + token_len..2 + token_len + 4].try_into().unwrap(),
        ) as usize;
        let msg_id = u64::from_be_bytes(
            buf[2 + token_len + 4..2 + token_len + 4 + MSG_ID_SIZE].try_into().unwrap(),
        );

        if user_data_len > MAX_USER_DATA_SIZE {
            return ParseResult::Error(ERR_HEAD_LIMIT);
        }
        if msg_id == 0 {
            return ParseResult::Error(ERR_ID_INVALID);
        }

        let end_offset = total_needed;

        if let Some(cb) = self.callbacks.token_cb {
            let result = cb(self.callbacks.ctx, token_tail.as_ptr(), token_tail.len());
            if result < 0 {
                return ParseResult::Error(result);
            }
        }

        self.expected_user_data_len = user_data_len;
        self.head = Some(HeadData {
            token_tail,
            user_data_len: user_data_len as u32,
            msg_id,
            end_offset,
        });

        ParseResult::Complete
    }

    fn parse_subhead(&mut self) -> ParseResult {
        let buf = &self.buffer;
        let head_end = self.head.as_ref().map(|h| h.end_offset).unwrap_or(0);
        let read_pos = |off: usize| -> usize { head_end + off };
        let mut off = 0usize;

        // Version
        if buf.len() < read_pos(off + 2) {
            return ParseResult::NeedMore;
        }
        let version_len = u16::from_be_bytes([buf[read_pos(off)], buf[read_pos(off + 1)]]) as usize;
        off += 2;
        if buf.len() < read_pos(off + version_len) {
            return ParseResult::NeedMore;
        }
        let version = buf[read_pos(off)..read_pos(off + version_len)].to_vec();
        off += version_len;

        // Client Address
        if buf.len() < read_pos(off + 2) {
            return ParseResult::NeedMore;
        }
        let addr_len = u16::from_be_bytes([buf[read_pos(off)], buf[read_pos(off + 1)]]) as usize;
        off += 2;
        if buf.len() < read_pos(off + addr_len) {
            return ParseResult::NeedMore;
        }
        let client_addr = buf[read_pos(off)..read_pos(off + addr_len)].to_vec();
        off += addr_len;

        // Route
        if buf.len() < read_pos(off + 2) {
            return ParseResult::NeedMore;
        }
        let route_len = u16::from_be_bytes([buf[read_pos(off)], buf[read_pos(off + 1)]]) as usize;
        off += 2;
        if buf.len() < read_pos(off + route_len) {
            return ParseResult::NeedMore;
        }
        let route = buf[read_pos(off)..read_pos(off + route_len)].to_vec();
        off += route_len;

        // Extensions
        if buf.len() < read_pos(off + 2) {
            return ParseResult::NeedMore;
        }
        let ext_len = u16::from_be_bytes([buf[read_pos(off)], buf[read_pos(off + 1)]]) as usize;
        off += 2;
        if buf.len() < read_pos(off + ext_len) {
            return ParseResult::NeedMore;
        }
        let extensions_raw = buf[read_pos(off)..read_pos(off + ext_len)].to_vec();
        off += ext_len;

        let subhead_size = off;
        if subhead_size > MAX_SUBHEAD_SIZE {
            return ParseResult::Error(ERR_SUBHEAD_LIMIT);
        }
        if version.as_slice() != PROTOCOL_VERSION.as_bytes() {
            return ParseResult::Error(ERR_VERSION_INCOMPATIBLE);
        }
        if route.is_empty() {
            return ParseResult::Error(ERR_ROUTE_UNREACHABLE);
        }

        let extensions = parse_extensions(&extensions_raw);

        if let Some(cb) = self.callbacks.route_cb {
            let result = cb(self.callbacks.ctx, route.as_ptr(), route.len());
            if result < 0 {
                return ParseResult::Error(result);
            }
        }

        self.subhead = Some(SubHeadData {
            version,
            client_addr,
            route,
            extensions_raw,
            extensions,
            end_offset: head_end + subhead_size,
        });

        ParseResult::Complete
    }

    fn parse_context(&mut self) -> ParseResult {
        let buf = &self.buffer;
        let subhead_end = self.subhead.as_ref().map(|s| s.end_offset).unwrap_or(0);
        let read_pos = |off: usize| -> usize { subhead_end + off };

        if buf.len() < read_pos(2) {
            return ParseResult::NeedMore;
        }

        let pair_count = u16::from_be_bytes([buf[read_pos(0)], buf[read_pos(1)]]) as usize;
        let pairs_data_size = pair_count * (REF_ID_SIZE + TIMESTAMP_SIZE);
        let total_needed = 2 + pairs_data_size;

        if buf.len() < read_pos(total_needed) {
            return ParseResult::NeedMore;
        }

        let mut pairs = Vec::with_capacity(pair_count);
        let mut off = 2usize;

        for _ in 0..pair_count {
            let ref_id = u64::from_be_bytes(
                buf[read_pos(off)..read_pos(off + REF_ID_SIZE)].try_into().unwrap(),
            );
            off += REF_ID_SIZE;
            let timestamp = u32::from_be_bytes(
                buf[read_pos(off)..read_pos(off + TIMESTAMP_SIZE)].try_into().unwrap(),
            );
            off += TIMESTAMP_SIZE;
            pairs.push((ref_id, timestamp));
        }

        if let Some(cb) = self.callbacks.ref_cb {
            for &(ref_id, ts) in &pairs {
                let result = cb(self.callbacks.ctx, ref_id, ts);
                if result < 0 {
                    return ParseResult::Error(result);
                }
            }
        }

        self.context = Some(ContextData {
            pairs,
            end_offset: subhead_end + total_needed,
        });

        ParseResult::Complete
    }

    fn parse_user_data(&mut self) -> ParseResult {
        let buf = &self.buffer;
        let context_end = self.context.as_ref().map(|c| c.end_offset).unwrap_or(0);
        let needed = self.expected_user_data_len;

        if buf.len() < context_end + needed {
            return ParseResult::NeedMore;
        }

        self.user_data = buf[context_end..context_end + needed].to_vec();
        ParseResult::Complete
    }

    fn parse_check(&mut self) -> ParseResult {
        let buf = &self.buffer;
        let context_end = self.context.as_ref().map(|c| c.end_offset).unwrap_or(0);
        let user_data_end = context_end + self.expected_user_data_len;

        if buf.len() < user_data_end + CHECK_SIZE {
            return ParseResult::NeedMore;
        }

        let expected = md5_bytes(&buf[..user_data_end]);
        let received = &buf[user_data_end..user_data_end + CHECK_SIZE];

        if expected != *received {
            return ParseResult::Error(ERR_CHECK_MISMATCH);
        }

        let head = self.head.as_ref().expect("head must be parsed");
        let subhead = self.subhead.as_ref().expect("subhead must be parsed");
        let context = self.context.as_ref().expect("context must be parsed");

        if let Some(cb) = self.callbacks.message_cb {
            let refs_vec: Vec<u64> = context.pairs.iter().map(|(r, _)| *r).collect();
            let ts_vec: Vec<u32> = context.pairs.iter().map(|(_, t)| *t).collect();

            let result = cb(
                self.callbacks.ctx,
                head.msg_id,
                subhead.route.as_ptr(),
                subhead.route.len(),
                self.user_data.as_ptr(),
                self.user_data.len(),
                context.pairs.len(),
                refs_vec.as_ptr(),
                ts_vec.as_ptr(),
            );
            if result < 0 {
                return ParseResult::Error(result);
            }
        }

        ParseResult::Complete
    }
}

impl Default for SmpParser {
    fn default() -> Self {
        Self::new()
    }
}

/// Internal parse result enum.
#[derive(Debug)]
enum ParseResult {
    Complete,
    NeedMore,
    Error(i32),
}

/// Parse SubHead Extensions from a `key=value\n`-formatted byte string.
fn parse_extensions(raw: &[u8]) -> Vec<(Vec<u8>, Vec<u8>)> {
    let mut pairs = Vec::new();
    for line in raw.split(|&b| b == b'\n') {
        if line.is_empty() {
            continue;
        }
        if let Some(eq_pos) = line.iter().position(|&b| b == b'=') {
            let key = line[..eq_pos].to_vec();
            let value = line[eq_pos + 1..].to_vec();
            pairs.push((key, value));
        }
    }
    pairs
}
