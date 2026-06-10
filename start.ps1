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

# Start backend
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

# Small pause for backend startup
Start-Sleep -Seconds 2

# Start frontend
Write-Host "[FRONTEND]" -ForegroundColor Cyan -NoNewline
Write-Host " starting Go client..." -ForegroundColor White
$frontendJob = Start-Job -ScriptBlock {
    param($dir)
    chcp 65001 | Out-Null
    [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
    $OutputEncoding = [System.Text.Encoding]::UTF8
    Set-Location $dir
    go run .
} -ArgumentList $frontendDir

Write-Host ""
Write-Host "System started. Press Ctrl+C to stop." -ForegroundColor Green
Write-Host ""

# Output logs in real time
while ($true) {
    $backendOutput = Receive-Job $backendJob -ErrorAction SilentlyContinue
    if ($backendOutput) {
        foreach ($line in $backendOutput) {
            Write-Host "[BACKEND] " -ForegroundColor Yellow -NoNewline
            Write-Host $line -ForegroundColor White
        }
    }

    $frontendOutput = Receive-Job $frontendJob -ErrorAction SilentlyContinue
    if ($frontendOutput) {
        foreach ($line in $frontendOutput) {
            Write-Host "[FRONTEND] " -ForegroundColor Cyan -NoNewline
            Write-Host $line -ForegroundColor White
        }
    }

    if ($backendJob.State -eq "Failed" -or $frontendJob.State -eq "Failed") {
        Write-Host "[ERROR] One of the processes failed" -ForegroundColor Red
        break
    }

    if ($backendJob.State -eq "Completed" -or $frontendJob.State -eq "Completed") {
        Write-Host "[INFO] One of the processes completed" -ForegroundColor DarkGray
        break
    }

    Start-Sleep -Milliseconds 100
}

# Cleanup
Remove-Job $backendJob -Force -ErrorAction SilentlyContinue
Remove-Job $frontendJob -Force -ErrorAction SilentlyContinue
