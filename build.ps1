#Requires -Version 5.1
$ErrorActionPreference = "Stop"
$root = $PSScriptRoot

$esc = [char]27
$useAnsi = -not [Console]::IsOutputRedirected
$c = @{
    green = "$esc[38;2;138;184;154m"
    blue  = "$esc[38;2;56;189;248m"
    red   = "$esc[38;2;255;68;68m"
    dim   = "$esc[38;2;102;102;102m"
    white = "$esc[37m"
    reset = "$esc[0m"
}

function Paint($text, $color) {
    if ($useAnsi) { return "$($c[$color])$text$($c.reset)" }
    return $text
}

function Line($mark, $msg, $color) {
    if ($useAnsi) {
        Write-Host "$(Paint $mark $color) $(Paint $msg white)"
    } else {
        Write-Host "$mark $msg"
    }
}

function Ok($msg)   { Line "[ok]" $msg "green" }
function Info($msg) { Line "  " $msg "dim" }
function Run($msg)  { Line "[>>]" $msg "blue" }
function Err($msg)  { Line "[!!]" $msg "red"; exit 1 }

if ($useAnsi) {
    Write-Host "$(Paint 'warden' green) $(Paint 'build' dim)"
} else {
    Write-Host "warden build"
}

Info "checking dependencies..."

if (-not (Get-Command go -ErrorAction SilentlyContinue)) { Err "Go is not installed or not in PATH" }
Ok "Go          $($(& go version).Split(' ')[2])"

if (-not (Get-Command python -ErrorAction SilentlyContinue)) { Err "Python is not installed or not in PATH" }
Ok "Python      $($(& python --version 2>&1))"

Run "building warden.exe..."
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
$exePath = "$root\warden.exe"
Ok "warden.exe built in ${elapsed}s"
if ($useAnsi) {
    Write-Host "$(Paint 'path:' dim) $(Paint $exePath white)"
} else {
    Write-Host "path: $exePath"
}
