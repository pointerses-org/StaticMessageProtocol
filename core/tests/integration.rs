//! Integration tests for SMP core library.

#[cfg(test)]
mod tests {
    use smp_core::*;
    use smp_core::ffi;

    // --- MD5 tests ---

    #[test]
    fn test_md5_empty() {
        let hash = md5_bytes(b"");
        let expected = [0xd4, 0x1d, 0x8c, 0xd9, 0x8f, 0x00, 0xb2, 0x04,
                         0xe9, 0x80, 0x09, 0x98, 0xec, 0xf8, 0x42, 0x7e];
        assert_eq!(hash, expected);
    }

    #[test]
    fn test_md5_hello() {
        let hash = md5_bytes(b"hello");
        let expected = [0x5d, 0x41, 0x40, 0x2a, 0xbc, 0x4b, 0x2a, 0x76,
                         0xb9, 0x71, 0x9d, 0x91, 0x10, 0x17, 0xc5, 0x92];
        assert_eq!(hash, expected);
    }

    #[test]
    fn test_md5_quick_brown_fox() {
        let hash = md5_bytes(b"The quick brown fox jumps over the lazy dog");
        let expected = [0x9e, 0x10, 0x7d, 0x9d, 0x37, 0x2b, 0xb6, 0x82,
                         0x6b, 0xd8, 0x1d, 0x35, 0x42, 0xa4, 0x19, 0xd6];
        assert_eq!(hash, expected);
    }

    // --- Token tests ---

    #[test]
    fn test_extract_token_tail_128() {
        let token = b"smpt128-0123456789abcdef0123456789abcdef";
        let tail = extract_token_tail(token).unwrap();
        assert_eq!(tail.as_slice(), b"89abcdef");
    }

    #[test]
    fn test_extract_token_tail_256() {
        let token = b"smpt256-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef";
        let tail = extract_token_tail(token).unwrap();
        assert_eq!(tail.as_slice(), b"89abcdef");
    }

    #[test]
    fn test_extract_token_tail_invalid() {
        assert!(extract_token_tail(b"invalid-token").is_none());
        assert!(extract_token_tail(b"smpt128-short").is_none());
        assert!(extract_token_tail(b"").is_none());
    }

    #[test]
    fn test_is_valid_token() {
        assert!(is_valid_token(b"smpt128-0123456789abcdef0123456789abcdef"));
        assert!(is_valid_token(b"smpt256-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"));
        assert!(is_valid_token(b"smpt128-0123456789ABCDEF0123456789abcdef"));
        assert!(!is_valid_token(b"invalid"));
        assert!(!is_valid_token(b"smpt128-tooshort"));
        assert!(!is_valid_token(b"smpt128-0123456789abcdef0123456789abcdef0"));
    }

    #[test]
    fn test_generate_token_128() {
        let token = generate_token_128();
        assert!(is_valid_token(token.as_bytes()));
        assert_eq!(token.len(), 8 + 32);
    }

    // --- Extensions parsing ---

    #[test]
    fn test_parse_extensions_basic() {
        let ext = b"key1=val1\nkey2=val2\n";
        let msg = assemble_message(b"abcdef12", 1, PROTOCOL_VERSION.as_bytes(),
            b"127.0.0.1", b"test", ext, &[], b"data");
        assert_eq!(validate_check(&msg), ERR_OK);
    }

    // --- Message assembly ---

    #[test]
    fn test_assemble_message_basic() {
        let msg = assemble_message(
            b"abcdef12", 0x1234, PROTOCOL_VERSION.as_bytes(),
            b"192.168.1.1", b"smp@alice", b"", &[], b"Hello, SMP!",
        );
        assert_eq!(validate_check(&msg), ERR_OK);
        assert_eq!(msg[0], 0x00);
        assert_eq!(msg[1], 0x08);
        assert_eq!(&msg[2..10], b"abcdef12");
    }

    #[test]
    fn test_assemble_message_with_context() {
        let ctx = vec![(0x111u64, 1700000000u32), (0x222u64, 1700000001u32)];
        let msg = assemble_message(
            b"abcdef12", 0xABCDEF, PROTOCOL_VERSION.as_bytes(),
            b"10.0.0.1", b"smp@bob", b"priority=high\n", &ctx, b"Test data",
        );
        assert_eq!(validate_check(&msg), ERR_OK);
    }

    #[test]
    fn test_assemble_message_empty_user_data() {
        let msg = assemble_message(
            b"abcdef12", 1, PROTOCOL_VERSION.as_bytes(),
            b"127.0.0.1", b"_push", b"", &[], b"",
        );
        assert_eq!(validate_check(&msg), ERR_OK);
    }

    // --- Check validation ---

    #[test]
    fn test_validate_check_ok() {
        let payload = vec![0u8; 100];
        let hash = md5_bytes(&payload);
        let mut full = payload;
        full.extend_from_slice(&hash);
        assert_eq!(validate_check(&full), ERR_OK);
    }

    #[test]
    fn test_validate_check_mismatch() {
        let payload = vec![0u8; 100];
        let mut hash = md5_bytes(&payload);
        hash[0] ^= 0xFF;
        let mut full = payload;
        full.extend_from_slice(&hash);
        assert_eq!(validate_check(&full), ERR_CHECK_MISMATCH);
    }

    #[test]
    fn test_validate_check_too_short() {
        assert_eq!(validate_check(&[0u8; 10]), ERR_CHECK_LENGTH_ILLEGAL);
    }

    // --- Parser state machine ---

    #[test]
    fn test_parser_basic_message() {
        let msg = assemble_message(
            b"abcdef12", 0x0000000000000001, PROTOCOL_VERSION.as_bytes(),
            b"127.0.0.1", b"smp@alice", b"", &[], b"test payload",
        );
        let mut parser = SmpParser::new();
        assert_eq!(parser.feed(&msg), ERR_OK);
        assert_eq!(parser.state(), STAGE_DONE);
    }

    #[test]
    fn test_parser_incremental_feed() {
        let msg = assemble_message(
            b"abcdef12", 2, PROTOCOL_VERSION.as_bytes(),
            b"10.0.0.1", b"smp@bob", b"key=value\n",
            &[(0x100u64, 1700000000u32)], b"incremental test",
        );
        let mut parser = SmpParser::new();
        for i in 0..msg.len() {
            parser.feed(&msg[i..i + 1]);
        }
        assert_eq!(parser.state(), STAGE_DONE);
    }

    #[test]
    fn test_parser_bad_token_length() {
        let mut msg = Vec::new();
        msg.extend_from_slice(&0u16.to_be_bytes());
        msg.extend_from_slice(&0u32.to_be_bytes());
        msg.extend_from_slice(&1u64.to_be_bytes());
        let version_len = PROTOCOL_VERSION.len();
        msg.extend_from_slice(&(version_len as u16).to_be_bytes());
        msg.extend_from_slice(PROTOCOL_VERSION.as_bytes());
        msg.extend_from_slice(&0u16.to_be_bytes());
        msg.extend_from_slice(&0u16.to_be_bytes());
        msg.extend_from_slice(&0u16.to_be_bytes());
        msg.extend_from_slice(&0u16.to_be_bytes());
        let check = md5_bytes(&msg);
        msg.extend_from_slice(&check);
        let mut parser = SmpParser::new();
        assert_eq!(parser.feed(&msg), ERR_TOKEN_FAILED);
    }

    #[test]
    fn test_parser_bad_version() {
        let bad_version = b"smp/0.99";
        let mut msg = Vec::new();
        msg.extend_from_slice(&8u16.to_be_bytes());
        msg.extend_from_slice(b"abcdef12");
        msg.extend_from_slice(&4u32.to_be_bytes());
        msg.extend_from_slice(&1u64.to_be_bytes());
        msg.extend_from_slice(&(bad_version.len() as u16).to_be_bytes());
        msg.extend_from_slice(bad_version);
        msg.extend_from_slice(&7u16.to_be_bytes());
        msg.extend_from_slice(b"127.0.0.1");
        msg.extend_from_slice(&9u16.to_be_bytes());
        msg.extend_from_slice(b"smp@test");
        msg.extend_from_slice(&0u16.to_be_bytes());
        msg.extend_from_slice(&0u16.to_be_bytes());
        msg.extend_from_slice(b"test");
        let check = md5_bytes(&msg);
        msg.extend_from_slice(&check);
        let mut parser = SmpParser::new();
        assert_eq!(parser.feed(&msg), ERR_VERSION_INCOMPATIBLE);
    }

    #[test]
    fn test_parser_bad_check() {
        let msg = assemble_message(
            b"abcdef12", 1, PROTOCOL_VERSION.as_bytes(),
            b"127.0.0.1", b"smp@test", b"", &[], b"test",
        );
        let mut corrupted = msg.clone();
        let last_byte = corrupted.len() - 1;
        corrupted[last_byte] ^= 0xFF;
        let mut parser = SmpParser::new();
        assert_eq!(parser.feed(&corrupted), ERR_CHECK_MISMATCH);
    }

    #[test]
    fn test_parser_context_refs() {
        let ctx = vec![
            (0x101u64, 1700000000u32),
            (0x8000000000000202u64, 1700000001u32),
            (0x303u64, 1700000002u32),
        ];
        let msg = assemble_message(
            b"abcdef12", 2, PROTOCOL_VERSION.as_bytes(),
            b"127.0.0.1", b"smp@test", b"", &ctx, b"context test",
        );
        let mut parser = SmpParser::new();
        assert_eq!(parser.feed(&msg), ERR_OK);
        let ctx_data = parser.context.as_ref().unwrap();
        assert_eq!(ctx_data.pairs.len(), 3);
        assert_eq!(ctx_data.pairs[1].0, 0x8000000000000202);
    }

    #[test]
    fn test_parser_reset() {
        let msg1 = assemble_message(
            b"abcdef12", 1, PROTOCOL_VERSION.as_bytes(),
            b"127.0.0.1", b"smp@test", b"", &[], b"first",
        );
        let msg2 = assemble_message(
            b"abcdef12", 2, PROTOCOL_VERSION.as_bytes(),
            b"127.0.0.1", b"smp@test", b"", &[], b"second",
        );
        let mut parser = SmpParser::new();
        parser.feed(&msg1);
        assert_eq!(parser.state(), STAGE_DONE);
        parser.reset();
        assert_eq!(parser.state(), STAGE_HEAD);
        parser.feed(&msg2);
        assert_eq!(parser.state(), STAGE_DONE);
    }

    // --- FFI tests ---

    #[test]
    fn test_ffi_parser_lifecycle() {
        let handle = ffi::smp_parser_new();
        assert!(!handle.is_null());
        ffi::smp_parser_free(handle);
    }

    #[test]
    fn test_ffi_assemble_and_free() {
        let mut out_len: usize = 0;
        let ptr = ffi::smp_assemble_message(
            b"abcdef12".as_ptr(), 8, 1,
            PROTOCOL_VERSION.as_ptr(), PROTOCOL_VERSION.len(),
            b"127.0.0.1".as_ptr(), 7,
            b"smp@test".as_ptr(), 8,
            b"".as_ptr(), 0,
            0, std::ptr::null(), std::ptr::null(),
            b"FFI test".as_ptr(), 8,
            &mut out_len,
        );
        assert!(!ptr.is_null());
        assert!(out_len > 0);
        let slice = unsafe { std::slice::from_raw_parts(ptr, out_len) };
        assert_eq!(validate_check(slice), ERR_OK);
        ffi::smp_free_buffer(ptr, out_len);
    }

    #[test]
    fn test_ffi_md5() {
        let data = b"hello world";
        let mut hash = [0u8; 16];
        ffi::smp_md5(data.as_ptr(), data.len(), hash.as_mut_ptr());
        assert_eq!(hash, md5_bytes(data));
    }

    #[test]
    fn test_ffi_is_cfm_id() {
        assert!(!ffi::smp_is_cfm_id(0x1234));
        assert!(ffi::smp_is_cfm_id(1u64 << 63));
        assert!(ffi::smp_is_cfm_id(0xFFFFFFFFFFFFFFFF));
    }

    #[test]
    fn test_ffi_error_message() {
        let mut buf = [0u8; 128];
        let len = ffi::smp_error_message(ERR_CHECK_MISMATCH, buf.as_mut_ptr(), buf.len());
        assert!(len > 0);
        assert!(std::str::from_utf8(&buf[..len as usize]).unwrap().contains("MD5"));
    }

    // --- Edge cases ---

    #[test]
    fn test_assemble_message_many_context_pairs() {
        let ctx: Vec<(u64, u32)> = (1..=100)
            .map(|i| (i as u64, 1700000000u32 + i as u32))
            .collect();
        let msg = assemble_message(
            b"abcdef12", 1, PROTOCOL_VERSION.as_bytes(),
            b"127.0.0.1", b"smp@ctx", b"", &ctx, b"many refs",
        );
        assert_eq!(validate_check(&msg), ERR_OK);
        let mut parser = SmpParser::new();
        parser.feed(&msg);
        assert_eq!(parser.context.as_ref().unwrap().pairs.len(), 100);
    }

    #[test]
    fn test_extensions_ignored_on_unknown_keys() {
        let ext = b"unknown1=value1\nunknown2=value2\nfuture=key\n";
        let msg = assemble_message(
            b"abcdef12", 1, PROTOCOL_VERSION.as_bytes(),
            b"127.0.0.1", b"smp@test", ext, &[], b"ext test",
        );
        let mut parser = SmpParser::new();
        assert_eq!(parser.feed(&msg), ERR_OK);
    }

    #[test]
    fn test_cfm_id_detection() {
        assert!(ffi::smp_is_cfm_id(1u64 << 63));
        assert!(!ffi::smp_is_cfm_id(1));
        assert!(!ffi::smp_is_cfm_id(0));
    }
}
