# Run warden - frontend and backend simultaneously

$ErrorActionPreference = "Stop"

# Set UTF-8 encoding
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$backendDir = Join-Path $scriptDir "agent"
$frontendDir = Join-Path $scriptDir "go"

Write-Host ""
Write-Host "═══════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "  WARDEN - starting system" -ForegroundColor Cyan -NoNewline
Write-Host ""
Write-Host "═══════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host ""

# Start backend in background
Write-Host "[BACKEND]" -ForegroundColor Yellow -NoNewline
Write-Host " starting Python server..." -ForegroundColor White
$backendJob = Start-Job -ScriptBlock {
    param($dir)
    chcp 65001 | Out-Null
    [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
    $OutputEncoding = [System.Text.Encoding]::UTF8
    Set-Location $dir
    python server.py
} -ArgumentList $backendDir

# Wait for backend to start
Start-Sleep -Seconds 2

# Check if backend started successfully
if ($backendJob.State -eq "Failed") {
    Write-Host "[ERROR] Backend failed to start" -ForegroundColor Red
    $backendError = Receive-Job $backendJob -ErrorAction SilentlyContinue
    Write-Host $backendError -ForegroundColor Red
    exit 1
}

# Start frontend in foreground (Bubbletea needs terminal access)
Write-Host "[FRONTEND]" -ForegroundColor Cyan -NoNewline
Write-Host " starting Go client..." -ForegroundColor White
Write-Host ""
Write-Host "System started. Press Ctrl+C to stop." -ForegroundColor Green
Write-Host ""

# Start background job to stream backend logs
$logJob = Start-Job -ScriptBlock {
    param($job)
    while ($true) {
        $output = Receive-Job $job -ErrorAction SilentlyContinue
        if ($output) {
            foreach ($line in $output) {
                Write-Host "[BACKEND] " -ForegroundColor Yellow -NoNewline
                Write-Host $line -ForegroundColor White
            }
        }
        if ($job.State -ne "Running") {
            break
        }
        Start-Sleep -Milliseconds 100
    }
} -ArgumentList $backendJob

# Start frontend in foreground
$originalDir = Get-Location
Set-Location $frontendDir
chcp 65001 | Out-Null
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
go run .

# Return to original directory
Set-Location $originalDir

# Cleanup after frontend exits
Stop-Job $logJob -ErrorAction SilentlyContinue
Remove-Job $logJob -Force -ErrorAction SilentlyContinue
Stop-Job $backendJob -ErrorAction SilentlyContinue
Remove-Job $backendJob -Force -ErrorAction SilentlyContinue
