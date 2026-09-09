//! CFM (Cloud File Message) module for SMP protocol.
//!
//! Provides C FFI exports for file storage, retrieval, and management.
//! Files are stored on the local filesystem with metadata in memory.

use std::collections::HashMap;
use std::os::raw::c_void;
use std::path::PathBuf;
use std::sync::Mutex;
use std::time::{SystemTime, UNIX_EPOCH};

/// Error codes for CFM operations.
pub const ERR_CFM_OK: i32 = 0;
pub const ERR_CFM_NOT_FOUND: i32 = -7001;
pub const ERR_CFM_EXPIRED: i32 = -7002;
pub const ERR_CFM_NO_PERMISSION: i32 = -7003;
pub const ERR_CFM_TOO_LARGE: i32 = -7004;
pub const ERR_CFM_STORE_FULL: i32 = -7005;
pub const ERR_CFM_FORMAT_INVALID: i32 = -7006;
pub const ERR_CFM_INTERNAL: i32 = -7099;

/// File entry metadata.
struct FileEntry {
    id: u64,
    file_path: PathBuf,
    uploader: String,
    public: bool,
    expires_at: u64, // Unix timestamp
    size: u64,
}

/// CFM store managing file storage.
struct CFMStore {
    entries: HashMap<u64, FileEntry>,
    base_dir: PathBuf,
    max_size: u64,
}

impl CFMStore {
    /// Create a new CFM store.
    fn new(base_dir: &str, max_mb: u64) -> Self {
        let base = PathBuf::from(base_dir);
        std::fs::create_dir_all(&base).ok();
        CFMStore {
            entries: HashMap::new(),
            base_dir: base,
            max_size: max_mb * 1024 * 1024,
        }
    }

    /// Save a file and return its entry.
    fn save(&mut self, cf_id: u64, data: &[u8], uploader: &str, public: bool, expire_minutes: u64) -> i32 {
        if data.len() as u64 > self.max_size {
            return ERR_CFM_TOO_LARGE;
        }

        let file_path = self.base_dir.join(format!("{:016x}.bin", cf_id));
        if let Err(_) = std::fs::write(&file_path, data) {
            return ERR_CFM_INTERNAL;
        }

        let expires_at = current_timestamp() + expire_minutes * 60;
        let entry = FileEntry {
            id: cf_id,
            file_path: file_path.clone(),
            uploader: uploader.to_string(),
            public,
            expires_at,
            size: data.len() as u64,
        };
        self.entries.insert(cf_id, entry);
        ERR_CFM_OK
    }

    /// Load a file by CFM ID.
    fn load(&mut self, cf_id: u64, requester: &str, out_buf: &mut Vec<u8>) -> i32 {
        out_buf.clear();

        let entry = match self.entries.get_mut(&cf_id) {
            Some(e) => e,
            None => return ERR_CFM_NOT_FOUND,
        };

        if current_timestamp() > entry.expires_at {
            let path = entry.file_path.clone();
            std::fs::remove_file(&path).ok();
            self.entries.remove(&cf_id);
            return ERR_CFM_EXPIRED;
        }

        if !entry.public && entry.uploader != requester {
            return ERR_CFM_NO_PERMISSION;
        }

        match std::fs::read(&entry.file_path) {
            Ok(data) => {
                out_buf.extend_from_slice(&data);
                ERR_CFM_OK
            }
            Err(_) => ERR_CFM_NOT_FOUND,
        }
    }

    /// Check if a file exists.
    fn exists(&self, cf_id: u64) -> bool {
        self.entries.contains_key(&cf_id)
    }

    /// Get file size.
    fn size(&self, cf_id: u64) -> Option<u64> {
        self.entries.get(&cf_id).map(|e| e.size)
    }

    /// Get file metadata.
    fn metadata(&self, cf_id: u64) -> Option<FileEntry> {
        self.entries.get(&cf_id).map(|e| FileEntry {
            id: e.id,
            file_path: e.file_path.clone(),
            uploader: e.uploader.clone(),
            public: e.public,
            expires_at: e.expires_at,
            size: e.size,
        })
    }

    /// List all file IDs.
    fn list_ids(&self) -> Vec<u64> {
        let mut ids: Vec<u64> = self.entries.keys().copied().collect();
        ids.sort();
        ids
    }

    /// Cleanup expired files.
    fn cleanup(&mut self) -> usize {
        let now = current_timestamp();
        let expired: Vec<u64> = self.entries.iter()
            .filter(|(_, e)| now > e.expires_at)
            .map(|(id, _)| *id)
            .collect();

        let mut removed = 0;
        for id in &expired {
            if let Some(entry) = self.entries.get(id) {
                std::fs::remove_file(&entry.file_path).ok();
                self.entries.remove(id);
                removed += 1;
            }
        }
        removed
    }

    /// Get total storage usage.
    fn usage(&self) -> u64 {
        self.entries.values().map(|e| e.size).sum()
    }
}

fn current_timestamp() -> u64 {
    SystemTime::now().duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0)
}

// ============================================================
// FFI Exports
// ============================================================

/// Global CFM store.
static STORE: Mutex<Option<CFMStore>> = Mutex::new(None);

/// Initialize the CFM store.
///
/// # Safety
/// - `base_dir` must be a valid C string
/// - `max_mb` must be > 0
#[no_mangle]
pub extern "C" fn cfm_init(base_dir: *const c_void, max_mb: u64) -> i32 {
    unsafe {
        if base_dir.is_null() {
            return ERR_CFM_INTERNAL;
        }
        let base_str = std::ffi::CStr::from_ptr(base_dir as *const std::os::raw::c_char);
        let base = match base_str.to_str() {
            Ok(s) => s,
            Err(_) => return ERR_CFM_INTERNAL,
        };
        let store = CFMStore::new(base, if max_mb == 0 { 100 } else { max_mb });
        let mut guard = STORE.lock().unwrap();
        *guard = Some(store);
        ERR_CFM_OK
    }
}

/// Save a file to CFM store.
///
/// # Safety
/// - `data` must be valid for `data_len` bytes
#[no_mangle]
pub extern "C" fn cfm_save(
    cf_id: u64,
    data: *const u8,
    data_len: usize,
    uploader: *const c_void,
    public: bool,
    expire_minutes: u64,
) -> i32 {
    unsafe {
        if data.is_null() && data_len > 0 {
            return ERR_CFM_INTERNAL;
        }
        let data_slice = std::slice::from_raw_parts(data, data_len);

        let uploader_str = if uploader.is_null() {
            ""
        } else {
            let c_str = std::ffi::CStr::from_ptr(uploader as *const std::os::raw::c_char);
            c_str.to_str().unwrap_or("")
        };

        let mut guard = STORE.lock().unwrap();
        let store = match &mut *guard {
            Some(s) => s,
            None => return ERR_CFM_INTERNAL,
        };
        store.save(cf_id, data_slice, uploader_str, public, expire_minutes)
    }
}

/// Load a file from CFM store.
///
/// # Safety
/// - `out_buf` must be valid for `out_buf_size` bytes
/// - Returns actual bytes written in `out_len`
#[no_mangle]
pub extern "C" fn cfm_load(
    cf_id: u64,
    requester: *const c_void,
    out_buf: *mut u8,
    out_buf_size: usize,
    out_len: *mut usize,
) -> i32 {
    unsafe {
        if out_buf.is_null() || out_buf_size == 0 {
            return ERR_CFM_INTERNAL;
        }
        if !out_len.is_null() {
            *out_len = 0;
        }

        let requester_str = if requester.is_null() {
            ""
        } else {
            let c_str = std::ffi::CStr::from_ptr(requester as *const std::os::raw::c_char);
            c_str.to_str().unwrap_or("")
        };

        let mut guard = STORE.lock().unwrap();
        let store = match &mut *guard {
            Some(s) => s,
            None => return ERR_CFM_INTERNAL,
        };

        let mut buf = Vec::new();
        let result = store.load(cf_id, requester_str, &mut buf);
        if result != ERR_CFM_OK {
            return result;
        }

        let copy_len = std::cmp::min(buf.len(), out_buf_size);
        std::ptr::copy_nonoverlapping(buf.as_ptr(), out_buf, copy_len);
        if !out_len.is_null() {
            *out_len = copy_len;
        }
        ERR_CFM_OK
    }
}

/// Check if a CFM file exists.
#[no_mangle]
pub extern "C" fn cfm_exists(cf_id: u64) -> bool {
    let guard = STORE.lock().unwrap();
    match &*guard {
        Some(store) => store.exists(cf_id),
        None => false,
    }
}

/// Get the size of a CFM file.
#[no_mangle]
pub extern "C" fn cfm_size(cf_id: u64) -> i64 {
    let guard = STORE.lock().unwrap();
    match &*guard {
        Some(store) => match store.size(cf_id) {
            Some(s) => s as i64,
            None => -1,
        },
        None => -1,
    }
}

/// Get the uploader of a CFM file.
#[no_mangle]
pub extern "C" fn cfm_get_uploader(cf_id: u64, out_buf: *mut u8, out_buf_size: usize) -> i32 {
    unsafe {
        if out_buf.is_null() || out_buf_size == 0 {
            return 0;
        }
        let guard = STORE.lock().unwrap();
        match &*guard {
            Some(store) => {
                if let Some(meta) = store.metadata(cf_id) {
                    let bytes = meta.uploader.as_bytes();
                    let copy_len = std::cmp::min(bytes.len(), out_buf_size - 1);
                    std::ptr::copy_nonoverlapping(bytes.as_ptr(), out_buf, copy_len);
                    *out_buf.add(copy_len) = 0;
                    copy_len as i32
                } else {
                    0
                }
            }
            None => 0,
        }
    }
}

/// Get the expiry timestamp of a CFM file.
#[no_mangle]
pub extern "C" fn cfm_get_expiry(cf_id: u64) -> u64 {
    let guard = STORE.lock().unwrap();
    match &*guard {
        Some(store) => store.metadata(cf_id).map(|m| m.expires_at).unwrap_or(0),
        None => 0,
    }
}

/// Get whether a CFM file is public.
#[no_mangle]
pub extern "C" fn cfm_is_public(cf_id: u64) -> bool {
    let guard = STORE.lock().unwrap();
    match &*guard {
        Some(store) => store.metadata(cf_id).map(|m| m.public).unwrap_or(false),
        None => false,
    }
}

/// List all CFM file IDs.
#[no_mangle]
pub extern "C" fn cfm_list(out_ids: *mut u64, max_count: usize, out_count: *mut usize) -> i32 {
    unsafe {
        if out_ids.is_null() || max_count == 0 {
            return ERR_CFM_INTERNAL;
        }
        if !out_count.is_null() {
            *out_count = 0;
        }

        let guard = STORE.lock().unwrap();
        match &*guard {
            Some(store) => {
                let ids = store.list_ids();
                let copy_count = std::cmp::min(ids.len(), max_count);
                std::ptr::copy_nonoverlapping(ids.as_ptr(), out_ids, copy_count);
                if !out_count.is_null() {
                    *out_count = copy_count;
                }
                ERR_CFM_OK
            }
            None => ERR_CFM_INTERNAL,
        }
    }
}

/// Cleanup expired CFM files.
#[no_mangle]
pub extern "C" fn cfm_cleanup() -> i32 {
    let mut guard = STORE.lock().unwrap();
    match &mut *guard {
        Some(store) => store.cleanup() as i32,
        None => 0,
    }
}

/// Get total storage usage in bytes.
#[no_mangle]
pub extern "C" fn cfm_usage() -> u64 {
    let guard = STORE.lock().unwrap();
    match &*guard {
        Some(store) => store.usage(),
        None => 0,
    }
}

/// Get error message for a CFM error code.
#[no_mangle]
pub extern "C" fn cfm_error_message(code: i32, out_buf: *mut u8, out_buf_size: usize) -> i32 {
    unsafe {
        if out_buf.is_null() || out_buf_size == 0 {
            return 0;
        }
        let msg = match code {
            ERR_CFM_OK => "OK",
            ERR_CFM_NOT_FOUND => "CFM file not found",
            ERR_CFM_EXPIRED => "CFM file has expired",
            ERR_CFM_NO_PERMISSION => "No permission to access CFM file",
            ERR_CFM_TOO_LARGE => "CFM file exceeds size limit",
            ERR_CFM_STORE_FULL => "CFM storage is full",
            ERR_CFM_FORMAT_INVALID => "CFM file format not supported",
            _ => "Unknown CFM error",
        };
        let msg_bytes = msg.as_bytes();
        let copy_len = std::cmp::min(msg_bytes.len(), out_buf_size - 1);
        std::ptr::copy_nonoverlapping(msg_bytes.as_ptr(), out_buf, copy_len);
        *out_buf.add(copy_len) = 0;
        copy_len as i32
    }
}
