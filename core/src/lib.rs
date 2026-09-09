//! SMP Core — Static Message Protocol Core Library
//!
//! Implements the 5-block message structure (Head, SubHead, Context, User Data, Check),
//! parsing state machine, message assembly, MD5 checksum, token utilities, and C FFI exports.

pub mod error;
pub mod types;
pub mod md5;
pub mod token;
pub mod parser;
pub mod assembler;
pub mod ffi;

// Re-export key items
pub use error::*;
pub use types::*;
pub use parser::SmpParser;
pub use assembler::{assemble_message, validate_check};
pub use md5::md5_bytes;
pub use token::{extract_token_tail, is_valid_token, generate_token_128};
