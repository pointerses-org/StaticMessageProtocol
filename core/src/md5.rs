//! MD5 hashing implementation.

use md5::compute as md5_compute;

/// Compute MD5 hash of a byte slice. Returns 16-byte digest.
#[inline]
pub fn md5_bytes(data: &[u8]) -> [u8; 16] {
    md5_compute(data).0
}
