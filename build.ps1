#Requires -Version 5.1
$ErrorActionPreference = "Stop"
$root = $PSScriptRoot

function Ok($msg)  { Write-Host "[ OK ] $msg" -ForegroundColor Green }
function Info($msg){ Write-Host "[ .. ] $msg" -ForegroundColor DarkGray }
function Err($msg) { Write-Host "[FAIL] $msg" -ForegroundColor Red; exit 1 }

Write-Host "warden build script" -ForegroundColor Cyan

Info "checking dependencies..."

if (-not (Get-Command go -ErrorAction SilentlyContinue)) { Err "Go is not installed or not in PATH" }
Ok "Go          $($(& go version).Split(' ')[2])"

if (-not (Get-Command python -ErrorAction SilentlyContinue)) { Err "Python is not installed or not in PATH" }
Ok "Python      $($(& python --version 2>&1))"

Info "building warden.exe..."
$start = Get-Date

Push-Location "$root\go\cmd\warden"
try {
    $out = go build -o "$root\warden.exe" . 2>&1
    if ($LASTEXITCODE -ne 0) { throw $out }
} catch {
    Err "build failed`n$($_.Exception.Message)"
} finally {
    Pop-Location
}

$elapsed = [math]::Round(((Get-Date) - $start).TotalSeconds, 2)
Ok "warden.exe built in ${elapsed}s"
Write-Host "path: $root\warden.exe" -ForegroundColor DarkGray
