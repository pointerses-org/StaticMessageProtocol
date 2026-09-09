# build-all.ps1 — Automated build for all SMP project components
#
# Usage: .\scripts\build-all.ps1 [-Clean] [-Profile <debug|release>]
#
# Build order:
#   1. Core (Rust)  → smp_core.dll  (stays in place for Go linking)
#   2. CFM  (Rust)  → cfm.dll       (stays in place)
#   3. Server (Go)  → smp-server.exe (needs smp_core.dll)
#   4. Client (Go)  → smp.exe        (needs smp_core.dll)
#   5. Copy all artifacts to target/

param(
    [switch]$Clean,
    [ValidateSet("debug", "release")]
    [string]$Profile = "release"
)

# ============================================================
# Paths — hardcoded project root, avoid $MyInvocation issues
$ROOT       = "D:\StaticMessageProtocol"
$TARGET     = "$ROOT\target"

$CORE_DIR   = "$ROOT\core"
$CFM_DIR    = "$ROOT\cfm"
$SERVER_DIR = "$ROOT\server"
$CLIENT_DIR = "$ROOT\client"

$RUST_BIN = "D:\llvm-mingw-20260826-msvcrt-i686\bin"
$GO_BIN   = "D:\go\bin"
$CC       = "x86_64-w64-mingw32-gcc"
$CXX      = "x86_64-w64-mingw32-g++"

$cargoProfile = if ($Profile -eq "release") { "--release" } else { "" }
$cargoTarget  = "--target x86_64-pc-windows-gnullvm"
$goTags       = if ($Profile -eq "release") { "" } else { "-tags debug" }

# Build artifact paths (original locations)
$coreSrc    = "$CORE_DIR\target\x86_64-pc-windows-gnullvm\release\smp_core.dll"
$coreRel    = "$CORE_DIR\target\release\smp_core.dll"
$cfmSrc     = "$CFM_DIR\target\x86_64-pc-windows-gnullvm\release\cfm.dll"
$serverSrc  = "$SERVER_DIR\smp-server.exe"
$clientSrc  = "$CLIENT_DIR\smp.exe"

# Final output paths
$coreOut    = "$TARGET\smp_core.dll"
$cfmOut     = "$TARGET\cfm.dll"
$serverOut  = "$TARGET\smp-server.exe"
$clientOut  = "$TARGET\smp.exe"

$serverTmp  = "$SERVER_DIR\tmp"
$clientTmp  = "$CLIENT_DIR\tmp"

# ============================================================
# Utilities
# ============================================================
function Step($label, $cmd) {
    Write-Host ""
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "  $label" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "> $cmd" -ForegroundColor DarkGray
    & cmd /c $cmd 2>&1 | ForEach-Object { Write-Host "  $_" -ForegroundColor Gray }
    $LASTEXITCODE -eq 0
}

# ============================================================
# Cleanup
# ============================================================
if ($Clean) {
    Write-Host ""
    Write-Host "========================================" -ForegroundColor Yellow
    Write-Host "  Cleaning" -ForegroundColor Yellow
    Write-Host "========================================" -ForegroundColor Yellow
    foreach ($p in @($TARGET,
                      "$CORE_DIR\target\x86_64-pc-windows-gnullvm\release",
                      "$CORE_DIR\target\release",
                      "$CFM_DIR\target\x86_64-pc-windows-gnullvm\release",
                      $serverSrc, $clientSrc, $serverTmp, $clientTmp)) {
        if (Test-Path $p) {
            Write-Host "  - $p"
            Remove-Item $p -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
    Write-Host "  [Cleaned]" -ForegroundColor Green
}

# ============================================================
# Environment
# ============================================================
$env:PATH = "$RUST_BIN;$GO_BIN;$env:PATH"
$env:CGO_ENABLED = "1"
$env:CC = $CC
$env:CXX = $CXX

New-Item -ItemType Directory -Path $TARGET    -Force | Out-Null
New-Item -ItemType Directory -Path $serverTmp -Force | Out-Null
New-Item -ItemType Directory -Path $clientTmp -Force | Out-Null

# ============================================================
# Build
# ============================================================
Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  SMP Build All" -ForegroundColor Cyan
Write-Host "  Profile: $Profile" -ForegroundColor Cyan
Write-Host "  Output:  target\" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# 1. Core (Rust)
Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  1/5 — Core (Rust)" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
$env:GOTMPDIR = $serverTmp
if (-not (Step "cargo build" "cd $CORE_DIR && cargo build $cargoProfile $cargoTarget")) {
    Write-Host "FAILED" -ForegroundColor Red; exit 1
}
# Copy DLL to core/target/release/ for Go cgo linking
New-Item -ItemType Directory -Path "$CORE_DIR\target\release" -Force | Out-Null
Copy-Item $coreSrc "$CORE_DIR\target\release\smp_core.dll" -Force

# 2. CFM (Rust)
Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  2/5 — CFM (Rust)" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
if (-not (Step "cargo build" "cd $CFM_DIR && cargo build $cargoProfile $cargoTarget")) {
    Write-Host "FAILED" -ForegroundColor Red; exit 1
}

# 3. Server (Go)
Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  3/5 — Server (Go)" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
$env:GOTMPDIR = $serverTmp
if (-not (Step "go build" "cd $SERVER_DIR && go build -o smp-server.exe $goTags ./cmd/smp-server/")) {
    Write-Host "FAILED" -ForegroundColor Red; exit 1
}

# 4. Client (Go)
Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  4/5 — Client (Go)" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
$env:GOTMPDIR = $clientTmp
if (-not (Step "go build" "cd $CLIENT_DIR && go build -o smp.exe $goTags ./cmd/smp/")) {
    Write-Host "FAILED" -ForegroundColor Red; exit 1
}

# 5. 复制所有产物到 target/
Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  5/5 — Collect to target/" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

$T = "$ROOT\target"
if (Test-Path $coreSrc) { Copy-Item $coreSrc "$T\smp_core.dll" -Force; $sz=(Get-Item "$T\smp_core.dll").Length; Write-Host ("  Copied: smp_core.dll -> target/ ({0:N1} KB)" -f ($sz/1024)) -ForegroundColor Green }
if (Test-Path $cfmSrc)  { Copy-Item $cfmSrc "$T\cfm.dll" -Force; $sz=(Get-Item "$T\cfm.dll").Length; Write-Host ("  Copied: cfm.dll -> target/ ({0:N1} KB)" -f ($sz/1024)) -ForegroundColor Green }
if (Test-Path $serverSrc) { Copy-Item $serverSrc "$T\smp-server.exe" -Force; $sz=(Get-Item "$T\smp-server.exe").Length; Write-Host ("  Copied: smp-server.exe -> target/ ({0:N1} KB)" -f ($sz/1024)) -ForegroundColor Green }
if (Test-Path $clientSrc) { Copy-Item $clientSrc "$T\smp.exe" -Force; $sz=(Get-Item "$T\smp.exe").Length; Write-Host ("  Copied: smp.exe -> target/ ({0:N1} KB)" -f ($sz/1024)) -ForegroundColor Green }

# Verify
Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Verify" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

$ok = 0
foreach ($f in @("$T\smp_core.dll", "$T\cfm.dll", "$T\smp-server.exe", "$T\smp.exe")) {
    if (Test-Path $f) {
        Write-Host "  [OK] $(Split-Path $f -Leaf)" -ForegroundColor Green
        $ok++
    } else {
        Write-Host "  [MISSING] $(Split-Path $f -Leaf)" -ForegroundColor Red
    }
}

if ($ok -lt 4) { Write-Host "  $ok/4 artifacts!" -ForegroundColor Red; exit 1 }

# Summary
Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "  Build Complete!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host "  Output: $T" -ForegroundColor White
Write-Host ""
foreach ($f in @("$T\smp_core.dll", "$T\cfm.dll", "$T\smp-server.exe", "$T\smp.exe")) {
    if (Test-Path $f) {
        $sz = [math]::Round((Get-Item $f).Length / 1024, 1)
        Write-Host ("  {0,-18} {1,8} KB" -f (Split-Path $f -Leaf), $sz) -ForegroundColor White
    }
}
Write-Host ""
Write-Host "  Config: server\config.yml" -ForegroundColor DarkGray
Write-Host "  Docs:   docs\core-docs.md, docs\server-docs.md, docs\client-docs.md" -ForegroundColor DarkGray
Write-Host ""
