#Requires -Version 5.1
$ErrorActionPreference = "Stop"
$root = $PSScriptRoot

# ── helpers ──────────────────────────────────────────────────────────────

function Tag($text, $color) {
    Write-Host "[$text]" -NoNewline -ForegroundColor $color
    Write-Host " " -NoNewline
}

function Ok($msg)  { Tag " OK " Green; Write-Host $msg }
function Info($msg){ Tag " .. " DarkGray; Write-Host $msg }
function Err($msg) { Tag "FAIL" Red; Write-Host $msg }

function Spinner($durationSec, $label) {
    $chars = '/','-','\','|'
    $end = [DateTime]::Now.AddSeconds($durationSec)
    $i = 0
    while ([DateTime]::Now -lt $end) {
        Write-Host "`r  $($chars[$i % 4]) $label" -NoNewline -ForegroundColor Cyan
        Start-Sleep -Milliseconds 80
        $i++
    }
    Write-Host "`r    $label" -ForegroundColor DarkGray
}

# ── banner ───────────────────────────────────────────────────────────────

Clear-Host
Write-Host ""
Write-Host "    __      __                _           " -ForegroundColor Cyan
Write-Host "    \ \    / /               | |          " -ForegroundColor Cyan
Write-Host "     \ \  / /__ _ _ __ _ __  | | ___ _ __ " -ForegroundColor Cyan
Write-Host "      \ \/ / _ \ '__| '_ \ | |/ _ \ '__|" -ForegroundColor Cyan
Write-Host "       \  /  __/ |  | | | || |  __/ |   " -ForegroundColor Cyan
Write-Host "        \/ \___|_|  |_| |_|_/ |\___|_|   " -ForegroundColor Cyan
Write-Host "                           |__/           " -ForegroundColor Cyan
Write-Host ""
Write-Host "        Build script  |  Windows edition" -ForegroundColor DarkGray
Write-Host ""
Write-Host "────────────────────────────────────────────" -ForegroundColor DarkGray
Write-Host ""

# ── deps ─────────────────────────────────────────────────────────────────

Info "checking dependencies..."

$go = Get-Command go -ErrorAction SilentlyContinue
if (-not $go) { Err "Go is not installed or not in PATH"; exit 1 }
Ok "Go          $($(& go version).Split(' ')[2])"

$py = Get-Command python -ErrorAction SilentlyContinue
if (-not $py) { Err "Python is not installed or not in PATH"; exit 1 }
Ok "Python      $($(& python --version 2>&1))"

Write-Host ""

# ── build ────────────────────────────────────────────────────────────────

Info "building warden.exe..."
$start = Get-Date

Push-Location "$root\go\cmd\warden"
try {
    $out = go build -o "$root\warden.exe" . 2>&1
    if ($LASTEXITCODE -ne 0) { throw $out }
} catch {
    Write-Host ""
    Err "build failed"
    Write-Host ""
    Write-Host $_.Exception.Message -ForegroundColor Red
    exit 1
} finally {
    Pop-Location
}

$elapsed = [math]::Round(((Get-Date) - $start).TotalSeconds, 2)

Write-Host ""
Ok "warden.exe built in ${elapsed}s"
Write-Host "    path: $root\warden.exe" -ForegroundColor DarkGray
Write-Host ""
Write-Host "────────────────────────────────────────────" -ForegroundColor DarkGray
Write-Host ""
