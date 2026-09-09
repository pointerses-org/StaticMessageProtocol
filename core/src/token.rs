//! Token utilities: validation, tail extraction, generation.

use crate::types::TOKEN_TAIL_LEN;

/// Extract the token tail (last 8 hex chars) from a full token string.
///
/// Full token formats:
///   `smpt128-<32 hex chars>`  → tail = last 8 of the 32 hex chars
///   `smpt256-<64 hex chars>`  → tail = last 8 of the 64 hex chars
///
/// Returns `None` if the token is malformed.
pub fn extract_token_tail(full_token: &[u8]) -> Option<Vec<u8>> {
    let token = std::str::from_utf8(full_token).ok()?;
    let hex_part = token
        .strip_prefix("smpt128-")
        .or_else(|| token.strip_prefix("smpt256-"))?;
    if hex_part.len() < TOKEN_TAIL_LEN {
        return None;
    }
    Some(hex_part[hex_part.len() - TOKEN_TAIL_LEN..].as_bytes().to_vec())
}

/// Validate a full token string.
///
/// Accepts:
///   - `smpt128-` + exactly 32 hex chars
///   - `smpt256-` + exactly 64 hex chars
pub fn is_valid_token(token: &[u8]) -> bool {
    let s = match std::str::from_utf8(token) {
        Ok(s) => s,
        Err(_) => return false,
    };
    let (prefix, expected_len) = if let Some(h) = s.strip_prefix("smpt128-") {
        (h, 32usize)
    } else if let Some(h) = s.strip_prefix("smpt256-") {
        (h, 64)
    } else {
        return false;
    };
    prefix.len() == expected_len && prefix.chars().all(|c| c.is_ascii_hexdigit())
}

/// Generate a random 128-bit token in `smpt128-<32hex>` format.
/// Uses a simple PRNG (not cryptographically secure — for dev/testing only).
pub fn generate_token_128() -> String {
    use std::time::{SystemTime, UNIX_EPOCH};
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    let mut state = (nanos as u64) ^ 0xDEADBEEFCAFEBABE;
    let mut hex = String::with_capacity(32);
    for _ in 0..16 {
        state ^= state << 13;
        state ^= state >> 7;
        state ^= state << 17;
        hex.push_str(&format!("{:02x}", (state & 0xFF) as u8));
    }
    format!("smpt128-{}", hex)
}
