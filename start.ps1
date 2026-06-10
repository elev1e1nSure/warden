# Run warden frontend and backend together.

[CmdletBinding()]
param(
	[int]$Port = 8765,
	[int]$StartupTimeoutSeconds = 60
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$OutputEncoding = [Console]::OutputEncoding = [Text.Encoding]::UTF8
[Console]::InputEncoding = [Text.Encoding]::UTF8
chcp.com 65001 | Out-Null

$env:PYTHONUTF8 = "1"
$env:PYTHONIOENCODING = "utf-8"

$scriptDir = $PSScriptRoot
if (-not $scriptDir) {
	$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
}

$backendDir = Join-Path $scriptDir "agent"
$frontendDir = Join-Path $scriptDir "go"
$runtimeDir = Join-Path $scriptDir ".warden"
$backendOutLog = Join-Path $runtimeDir "backend.out.log"
$backendErrLog = Join-Path $runtimeDir "backend.err.log"
$healthUrl = "http://localhost:$Port/health"

$originalDir = Get-Location
$backendProcess = $null
$startedBackend = $false

function Write-Status {
	param(
		[Parameter(Mandatory = $true)][string]$Name,
		[Parameter(Mandatory = $true)][string]$Message,
		[ConsoleColor]$Color = [ConsoleColor]::White
	)

	Write-Host "[$Name] " -ForegroundColor $Color -NoNewline
	Write-Host $Message
}

function Resolve-CommandPath {
	param([Parameter(Mandatory = $true)][string]$Name)

	$command = Get-Command $Name -CommandType Application -ErrorAction Stop | Select-Object -First 1
	return $command.Source
}

function Test-BackendHealth {
	try {
		$response = Invoke-WebRequest -UseBasicParsing -Uri $healthUrl -TimeoutSec 2
		return $response.StatusCode -eq 200 -and $response.Content.Trim() -eq "ok"
	} catch {
		return $false
	}
}

function Test-PortListening {
	$connection = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
	return $null -ne $connection
}

function Wait-BackendReady {
	param([Parameter(Mandatory = $true)][System.Diagnostics.Process]$Process)

	$deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
	while ((Get-Date) -lt $deadline) {
		if ($Process.HasExited) {
			throw "Backend exited early with code $($Process.ExitCode). See $backendErrLog"
		}

		if (Test-BackendHealth) {
			return
		}

		Start-Sleep -Milliseconds 500
	}

	throw "Backend did not become healthy in ${StartupTimeoutSeconds}s. See $backendErrLog"
}

function Stop-ProcessTree {
	param([Parameter(Mandatory = $true)][int]$ProcessId)

	$children = Get-CimInstance Win32_Process -Filter "ParentProcessId = $ProcessId" -ErrorAction SilentlyContinue
	foreach ($child in $children) {
		Stop-ProcessTree -ProcessId $child.ProcessId
	}

	Stop-Process -Id $ProcessId -Force -ErrorAction SilentlyContinue
}

try {
	if (-not (Test-Path $backendDir -PathType Container)) {
		throw "Backend directory not found: $backendDir"
	}
	if (-not (Test-Path $frontendDir -PathType Container)) {
		throw "Frontend directory not found: $frontendDir"
	}

	$python = Resolve-CommandPath "python"
	$go = Resolve-CommandPath "go"

	New-Item -ItemType Directory -Force -Path $runtimeDir | Out-Null
	Remove-Item -LiteralPath $backendOutLog, $backendErrLog -Force -ErrorAction SilentlyContinue

	Write-Host ""
	Write-Host "============================================================" -ForegroundColor Cyan
	Write-Host "  WARDEN - starting system" -ForegroundColor Cyan
	Write-Host "============================================================" -ForegroundColor Cyan
	Write-Host ""

	if (Test-BackendHealth) {
		Write-Status "BACKEND" "already healthy on $healthUrl" Yellow
	} elseif (Test-PortListening) {
		throw "Port $Port is busy, but $healthUrl is not healthy. Stop the old backend and run this script again."
	} else {
		Write-Status "BACKEND" "starting Python server..." Yellow
		$backendProcess = Start-Process `
			-FilePath $python `
			-ArgumentList @("server.py") `
			-WorkingDirectory $backendDir `
			-RedirectStandardOutput $backendOutLog `
			-RedirectStandardError $backendErrLog `
			-WindowStyle Hidden `
			-PassThru
		$startedBackend = $true
		Wait-BackendReady -Process $backendProcess
		Write-Status "BACKEND" "ready on $healthUrl" Green
	}

	Write-Status "FRONTEND" "starting Go client..." Cyan
	Write-Host ""
	Write-Host "System started. Press Ctrl+C to stop." -ForegroundColor Green
	Write-Host "Backend logs: $backendOutLog"
	Write-Host ""

	Set-Location $frontendDir
	& $go run .
	if ($LASTEXITCODE -ne 0) {
		throw "Frontend exited with code $LASTEXITCODE"
	}
} finally {
	Set-Location $originalDir

	if ($startedBackend -and $null -ne $backendProcess -and -not $backendProcess.HasExited) {
		Write-Status "BACKEND" "stopping Python server..." Yellow
		Stop-ProcessTree -ProcessId $backendProcess.Id
	}
}
