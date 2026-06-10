# Запуск warden - фронтенд и бэкенд одновременно

$ErrorActionPreference = "Stop"

# Установка UTF-8 кодировки
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$backendDir = Join-Path $scriptDir "agent"
$frontendDir = Join-Path $scriptDir "go"

Write-Host ""
Write-Host "═══════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "  WARDEN - запуск системы" -ForegroundColor Cyan -NoNewline
Write-Host ""
Write-Host "═══════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host ""

# Запуск бэкенда
Write-Host "[BACKEND]" -ForegroundColor Yellow -NoNewline
Write-Host " запуск Python сервера..." -ForegroundColor White
$backendJob = Start-Job -ScriptBlock {
    param($dir)
    Set-Location $dir
    python server.py
} -ArgumentList $backendDir

# Небольшая пауза для старта бэкенда
Start-Sleep -Seconds 2

# Запуск фронтенда
Write-Host "[FRONTEND]" -ForegroundColor Cyan -NoNewline
Write-Host " запуск Go клиента..." -ForegroundColor White
$frontendJob = Start-Job -ScriptBlock {
    param($dir)
    Set-Location $dir
    go run .
} -ArgumentList $frontendDir

Write-Host ""
Write-Host "Система запущена. Нажмите Ctrl+C для остановки." -ForegroundColor Green
Write-Host ""

# Вывод логов в реальном времени
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
        Write-Host "[ERROR] Один из процессов завершился с ошибкой" -ForegroundColor Red
        break
    }

    if ($backendJob.State -eq "Completed" -or $frontendJob.State -eq "Completed") {
        Write-Host "[INFO] Один из процессов завершился" -ForegroundColor DarkGray
        break
    }

    Start-Sleep -Milliseconds 100
}

# Очистка
Remove-Job $backendJob -Force -ErrorAction SilentlyContinue
Remove-Job $frontendJob -Force -ErrorAction SilentlyContinue
