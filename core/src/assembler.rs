//! Message assembly: builds complete SMP messages from components.

use crate::error::*;
use crate::md5::md5_bytes;
use crate::types::*;

/// Assemble a complete SMP message from its components.
/// Returns a Vec<u8> containing the serialized message (including Check).
pub fn assemble_message(
    token_tail: &[u8],
    msg_id: u64,
    version: &[u8],
    client_addr: &[u8],
    route: &[u8],
    extensions: &[u8],
    context_pairs: &[(u64, u32)],
    user_data: &[u8],
) -> Vec<u8> {
    let mut msg = Vec::with_capacity(256);

    // --- Head ---
    msg.extend_from_slice(&(token_tail.len() as u16).to_be_bytes());
    msg.extend_from_slice(token_tail);
    msg.extend_from_slice(&(user_data.len() as u32).to_be_bytes());
    msg.extend_from_slice(&msg_id.to_be_bytes());

    // --- SubHead ---
    msg.extend_from_slice(&(version.len() as u16).to_be_bytes());
    msg.extend_from_slice(version);
    msg.extend_from_slice(&(client_addr.len() as u16).to_be_bytes());
    msg.extend_from_slice(client_addr);
    msg.extend_from_slice(&(route.len() as u16).to_be_bytes());
    msg.extend_from_slice(route);
    msg.extend_from_slice(&(extensions.len() as u16).to_be_bytes());
    msg.extend_from_slice(extensions);

    // --- Context ---
    msg.extend_from_slice(&(context_pairs.len() as u16).to_be_bytes());
    for &(ref_id, ts) in context_pairs {
        msg.extend_from_slice(&ref_id.to_be_bytes());
        msg.extend_from_slice(&ts.to_be_bytes());
    }

    // --- User Data ---
    msg.extend_from_slice(user_data);

    // --- Check (MD5 of Head + SubHead + Context + User Data) ---
    let check = md5_bytes(&msg);
    msg.extend_from_slice(&check);

    msg
}

/// Validate the Check block of a complete SMP message.
/// Returns ERR_OK if valid, or a negative error code.
pub fn validate_check(data: &[u8]) -> i32 {
    if data.len() < CHECK_SIZE {
        return ERR_CHECK_LENGTH_ILLEGAL;
    }
    let payload_len = data.len() - CHECK_SIZE;
    let expected = md5_bytes(&data[..payload_len]);
    let received = &data[payload_len..];
    if expected != *received {
        return ERR_CHECK_MISMATCH;
    }
    ERR_OK
}

/// Parse SubHead Extensions from a `key=value\n`-formatted byte string.
/// Unknown keys are silently ignored. Empty lines are skipped.
/// Malformed lines (no `=`) are ignored.
pub fn parse_extensions(raw: &[u8]) -> Vec<(Vec<u8>, Vec<u8>)> {
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
