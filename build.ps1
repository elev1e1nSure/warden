$ErrorActionPreference = "Stop"
$root = $PSScriptRoot

Write-Host "building warden..." -ForegroundColor DarkGray

Push-Location "$root\go\cmd\warden"
try {
    go build -o "$root\warden.exe" .
} finally {
    Pop-Location
}

Write-Host "done -> warden.exe" -ForegroundColor Green
