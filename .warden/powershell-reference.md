# Shell & Tools Reference — Windows / PowerShell 7

Reference for terminal interaction on Windows. Target: PowerShell 7.6 (LTS) running as `pwsh`.
Examples use inline comments so intent is clear without guessing.

---

## Risk Markers

Commands in this reference are tagged with risk levels:

- **SAFE** — read-only, no side effects
- **CONFIRM** — modifies state; requires user approval in leashed mode
- **BLOCKED** — dangerous or system-level; blocked by `agent/safety/` in leashed mode

## PowerShell Basics

### Version & Setup

```powershell
$PSVersionTable.PSVersion          # check PS version
pwsh                               # launch PowerShell 7
pwsh -Command "Get-Process"        # run single command and exit
pwsh -File "script.ps1"            # run a script file
pwsh -NoProfile -Command "..."     # skip profile (faster, clean env)

# Install / update PS7 via winget
winget install --id Microsoft.PowerShell --source winget
winget upgrade --id Microsoft.PowerShell
```

### Variables & Types

```powershell
$name = "Alice"                    # string
$num  = 42                         # int
$flag = $true                      # bool ($true / $false)
$nothing = $null                   # null

[int]$count    = "5"               # typed — auto-converts
[string]$s     = 123               # "123"
[bool]$active  = 1                 # $true

# String interpolation
"Hello $name"                      # "Hello Alice"
"Path: $($env:USERPROFILE)\docs"   # expressions need $()

# Multiline string (here-string) — closing "@ must be at start of line
$text = @"
Line one
Line two — $name is here
"@

# Arrays
$arr = @(1, 2, 3)
$arr += 4                          # append
$arr[0]                            # first element
$arr[-1]                           # last element
$arr.Count                         # length

# Hashtables (like dicts)
$h = @{ name = "Alice"; age = 30 }
$h["name"]                         # "Alice"
$h.name                            # same
$h["city"] = "Moscow"              # add key
$h.Remove("age")                   # delete key
$h.Keys                            # all keys
$h.Values                          # all values
```

### Modern Operators (PS7+)

```powershell
# Ternary — short if/else
$label = ($score -gt 50) ? "pass" : "fail"
$msg   = ($null -eq $user) ? "anonymous" : $user.Name

# Null-coalescing — use right side if left is $null
$val    = $env:MY_VAR ?? "default"
$config = $userConfig ?? $defaultConfig

# Null-coalescing assignment — assign only if currently $null
$val ??= "fallback"                # equivalent to: if ($null -eq $val) { $val = "fallback" }

# Pipeline chain operators
npm install && npm run build       # right runs only if left succeeded (exit 0)
git pull || Write-Host "pull failed"   # right runs only if left failed (exit != 0)
npm install && npm test || Write-Error "pipeline failed"
```

### Comparison & Logic

```powershell
# Always use -eq not ==
$a -eq $b     # equal
$a -ne $b     # not equal
$a -gt $b     # greater than
$a -lt $b     # less than
$a -ge $b     # >=
$a -le $b     # <=

# String matching (case-insensitive by default)
"Hello" -eq "hello"               # $true
"Hello" -ceq "hello"              # $false (case-sensitive)
"report.log" -like "*.log"        # wildcard match
"error 404" -match "\d{3}"        # regex match
"abc" -in @("abc", "def")         # membership check
"abc" -notin @("xyz")             # negated membership
"hello world" -replace "world", "PS"  # string replace

# Logical
$a -and $b
$a -or $b
-not $a  ;  !$a                   # both work
```

### Control Flow

```powershell
# If/Else
if ($x -gt 10) {
    "big"
} elseif ($x -eq 10) {
    "ten"
} else {
    "small"
}

# Switch
switch ($status) {
    "ok"      { "all good" }
    "warning" { Write-Warning "heads up" }
    "error"   { Write-Error "broken" }
    default   { "unknown: $status" }
}

# Switch with regex
switch -Regex ($line) {
    "^ERROR"   { "error line" }
    "^WARN"    { "warning line" }
    "^\d{4}-"  { "timestamped line" }
}

# Loops
foreach ($item in $collection) { $item }
for ($i = 0; $i -lt 10; $i++) { $i }
while ($condition) { ... }
1..10 | ForEach-Object { $_ * 2 }  # pipeline loop; $_ = current item

# Range
1..5            # @(1,2,3,4,5)
'a'..'e'        # @('a','b','c','d','e')

# Loop control
break           # exit loop
continue        # skip to next iteration
```

### Functions

```powershell
function Get-Greeting {
    param(
        [string]$Name = "World",   # default value
        [int]$Times   = 1,
        [switch]$Loud              # boolean flag, passed as -Loud
    )
    $msg = "Hello, $Name"
    if ($Loud) { $msg = $msg.ToUpper() }
    1..$Times | ForEach-Object { $msg }
}

Get-Greeting -Name "Alice" -Times 3 -Loud
```

### Error Handling

```powershell
# -ErrorAction controls what happens on non-terminating errors
Get-Item "missing.txt" -ErrorAction SilentlyContinue    # ignore silently
Get-Item "missing.txt" -ErrorAction Stop                # throw exception

# Try/Catch
try {
    Get-Item "missing.txt" -ErrorAction Stop
} catch [System.IO.FileNotFoundException] {
    "File not found: $($_.Exception.Message)"
} catch {
    "Unexpected error: $($_.Exception.Message)"
} finally {
    "Always runs (cleanup)"
}

# Check success of last command
$?                                 # $true if last command succeeded
$LASTEXITCODE                      # exit code of last native executable (0 = success)

if ($LASTEXITCODE -ne 0) { Write-Error "Command failed with code $LASTEXITCODE" }

# Global preference
$ErrorActionPreference = "Stop"    # make all errors terminating in this scope
```

### Pipeline

```powershell
# Pipe output through multiple cmdlets
Get-Process | Where-Object { $_.CPU -gt 100 } | Sort-Object CPU -Descending | Select-Object -First 5

# $_ or $PSItem — current pipeline object
Get-ChildItem | Where-Object { $_.Extension -eq ".log" }

# Common pipeline cmdlets
... | Where-Object { $_.Name -like "app*" }    # filter; alias: ?
... | ForEach-Object { $_.Name.ToUpper() }     # transform; alias: %
... | Sort-Object Length -Descending           # sort
... | Select-Object Name, Id, CPU              # pick properties
... | Select-Object -First 10                  # take first N
... | Select-Object -ExpandProperty Name       # unwrap single property to plain values
... | Group-Object Company                     # group by property
... | Measure-Object Length -Sum -Average      # aggregate
... | Out-String                               # convert objects to readable text

# Splatting — clean way to pass many params
$params = @{
    Path        = "C:\logs"
    Filter      = "*.log"
    Recurse     = $true
    ErrorAction = "SilentlyContinue"
}
Get-ChildItem @params
```

### Output Streams

```powershell
Write-Output "goes into pipeline"      # capturable, used in scripts
Write-Host   "goes to screen only"     # NOT in pipeline — use for user messages
Write-Verbose "debug info"             # shown with -Verbose flag
Write-Warning "something looks off"
Write-Error   "something broke"
Write-Debug   "deep debug"             # shown with -Debug flag

# Redirect
command 2>&1                           # merge stderr into stdout
command > out.txt                      # stdout to file
command 2> err.txt                     # stderr to file
command *> all.txt                     # all streams to file
```

### Environment Variables

```powershell
$env:PATH
$env:USERPROFILE                       # C:\Users\username
$env:APPDATA                           # C:\Users\username\AppData\Roaming
$env:LOCALAPPDATA                      # C:\Users\username\AppData\Local
$env:TEMP
$env:COMPUTERNAME
$env:USERNAME

$env:MY_VAR = "value"                  # set for current session only

Get-ChildItem Env:                     # list all env vars
Get-ChildItem Env: | Where-Object Name -like "PATH*"

# Persistent — survives session
[System.Environment]::SetEnvironmentVariable("MY_VAR", "value", "User")    # CONFIRM — persistent env var
[System.Environment]::SetEnvironmentVariable("MY_VAR", "value", "Machine") # BLOCKED — system scope change
[System.Environment]::GetEnvironmentVariable("MY_VAR", "User")             # read persistent

# Reload PATH in current session after changes
$env:PATH = [System.Environment]::GetEnvironmentVariable("PATH","Machine") + ";" +
            [System.Environment]::GetEnvironmentVariable("PATH","User")
```

### Execution Policy & Profile

```powershell
Get-ExecutionPolicy                    # check current policy
Set-ExecutionPolicy RemoteSigned -Scope CurrentUser   # allow local scripts (no admin needed)
Set-ExecutionPolicy Bypass -Scope Process              # bypass for current session only

# Policies:
# Restricted     — no scripts (Windows default)
# RemoteSigned   — local scripts ok; downloaded scripts need signature
# Unrestricted   — everything ok
# Bypass         — nothing blocked (good for CI/automated runs)

$PROFILE                               # path to your profile script
Test-Path $PROFILE                     # check if profile exists
New-Item $PROFILE -Force               # create profile file
notepad $PROFILE                       # edit profile
```

---

## Filesystem

```powershell
# Navigate
Set-Location "C:\Projects"             # cd; alias: cd, sl
Set-Location ..                        # go up
Push-Location "C:\temp"                # pushd — save current, go to new
Pop-Location                           # popd — return to saved
Get-Location                           # pwd — current directory

# List
Get-ChildItem                          # ls / dir in current dir
Get-ChildItem "C:\Projects"
Get-ChildItem -Recurse                 # recursive
Get-ChildItem -Filter "*.log"          # by pattern
Get-ChildItem -File                    # files only
Get-ChildItem -Directory               # dirs only
Get-ChildItem -Hidden                  # include hidden
Get-ChildItem -Recurse -Filter "*.ps1" | Select-Object FullName

# Paths
Test-Path "C:\some\path"               # $true / $false
Resolve-Path ".\relative\path"         # get absolute path
Split-Path "C:\dir\file.txt"           # returns "C:\dir"
Split-Path "C:\dir\file.txt" -Leaf     # returns "file.txt"
Split-Path "C:\dir\file.txt" -Extension  # returns ".txt" (PS7+)
Join-Path "C:\dir" "sub" "file.txt"    # safe join: "C:\dir\sub\file.txt"
[System.IO.Path]::GetTempPath()        # temp directory

# Create
New-Item "C:\temp\file.txt" -ItemType File
New-Item "C:\temp\folder"   -ItemType Directory
New-Item "C:\temp\folder"   -ItemType Directory -Force   # no error if exists

# Copy / Move / Delete
Copy-Item "source.txt" "dest.txt"
Copy-Item "C:\src" "C:\dst" -Recurse
Move-Item "old.txt" "new.txt"
Rename-Item "old.txt" "new.txt"
Remove-Item "file.txt"  # CONFIRM
Remove-Item "folder" -Recurse -Force   # BLOCKED — immediate and unrecoverable

# Read / Write
Get-Content "file.txt"                 # read entire file (returns array of lines)
Get-Content "file.txt" -Raw            # read as single string
Get-Content "file.txt" -Tail 20        # last 20 lines
Get-Content "file.txt" -Wait           # tail -f equivalent
Set-Content "file.txt" "new content"   # CONFIRM — overwrites
Add-Content "file.txt" "more content"  # CONFIRM — appends
Out-File    "output.txt"               # CONFIRM — overwrites
"text" | Out-File "file.txt" -Append  # CONFIRM

# Find files
Get-ChildItem -Recurse | Where-Object { $_.LastWriteTime -gt (Get-Date).AddDays(-1) }
Get-ChildItem -Recurse -Filter "*.log" | Where-Object { $_.Length -gt 10MB }
```

---

## Processes

```powershell
# List
Get-Process                            # all processes
Get-Process -Name "chrome"             # by name (wildcards ok: "chrom*")
Get-Process | Sort-Object CPU -Descending | Select-Object -First 10
Get-Process | Where-Object { $_.WorkingSet -gt 500MB }

# Properties available: Name, Id, CPU, WorkingSet, StartTime, Path, Company
$p = Get-Process -Name "code" | Select-Object -First 1
$p.Id ; $p.Path ; $p.CPU

# Start
Start-Process "notepad"
Start-Process "notepad" -ArgumentList "C:\file.txt"
Start-Process "cmd" -ArgumentList "/c dir C:\" -Wait    # wait for exit
Start-Process "script.ps1" -Verb RunAs                  # run as admin
Start-Process "app.exe" -WindowStyle Hidden             # no window

# Stop
Stop-Process -Name "notepad"  # CONFIRM
Stop-Process -Id 1234
Stop-Process -Name "chrome" -Force     # CONFIRM — force kill
Get-Process -Name "hung*" | Stop-Process -Force

# Exit codes
$p = Start-Process "cmd" -ArgumentList "/c exit 42" -Wait -PassThru
$p.ExitCode                            # 42
```

### Windows-specific: tasklist / taskkill

```powershell
# tasklist — view all running processes (classic Windows CLI)
tasklist                               # all processes
tasklist /FI "IMAGENAME eq chrome.exe" # filter by name
tasklist /FI "PID eq 1234"
tasklist /FO CSV                       # output as CSV
tasklist /V                            # verbose (includes window titles)

# taskkill — kill processes
taskkill /PID 1234
taskkill /IM "notepad.exe"
taskkill /IM "chrome.exe" /F           # CONFIRM — force kill
taskkill /IM "app.exe" /T             # BLOCKED — kills process tree

# systeminfo — machine details
systeminfo
systeminfo | findstr /C:"OS Name" /C:"Total Physical Memory"

# shutdown
shutdown /s /t 0                       # BLOCKED — system shutdown
shutdown /r /t 0                       # BLOCKED — system restart
shutdown /r /t 60                      # BLOCKED — restart (any timer)
shutdown /p                            # BLOCKED — immediate power off
shutdown /h                            # BLOCKED — hibernate
shutdown /a                            # abort pending shutdown
shutdown /l                            # log off current user
```

---

## Services (Windows)

```powershell
# PowerShell cmdlets
Get-Service                            # all services
Get-Service -Name "wuauserv"           # Windows Update
Get-Service | Where-Object { $_.Status -eq "Running" }
Get-Service | Where-Object { $_.StartType -eq "Automatic" -and $_.Status -ne "Running" }

Start-Service   "ServiceName"  # CONFIRM
Stop-Service    "ServiceName"  # CONFIRM
Restart-Service "ServiceName"  # CONFIRM
Suspend-Service "ServiceName"          # CONFIRM
Resume-Service  "ServiceName"  # CONFIRM

Set-Service "ServiceName" -StartupType Automatic   # BLOCKED — system service change
Set-Service "ServiceName" -Description "My service"

# sc.exe — lower-level, more control (needs admin for most ops)
sc query                               # list all services
sc query "wuauserv"                    # specific service
sc query type= all state= all         # all types and states
sc start  "ServiceName"
sc stop   "ServiceName"
sc config "ServiceName" start= auto   # BLOCKED — system service change
sc config "ServiceName" start= demand # manual
sc config "ServiceName" start= disabled  # BLOCKED
sc create "MySvc" binPath= "C:\app\service.exe" start= auto  # BLOCKED
sc delete "MySvc"  # BLOCKED
sc description "MySvc" "My service description"

# Check if service exists
(Get-Service -Name "ServiceName" -ErrorAction SilentlyContinue) -ne $null
```

---

## Network

```powershell
# Connectivity checks
Test-Connection "google.com"                                # ping (4 times)
Test-Connection "google.com" -Count 1 -Quiet               # returns $true / $false
Test-NetConnection "google.com" -Port 443                  # TCP port check
Test-NetConnection "8.8.8.8" -Port 53

# Network interfaces & IPs
Get-NetAdapter                                             # list adapters
Get-NetAdapter | Where-Object { $_.Status -eq "Up" }
Get-NetIPAddress                                           # all IP addresses
Get-NetIPAddress -AddressFamily IPv4
Get-NetIPConfiguration                                     # full config per adapter

# Open ports / connections
Get-NetTCPConnection                                       # all TCP connections
Get-NetTCPConnection -State Listen                         # listening ports only
Get-NetTCPConnection -LocalPort 8080                       # specific port
Get-NetTCPConnection -State Listen | Sort-Object LocalPort | Select-Object LocalPort, OwningProcess
# Check if port is in use
(Get-NetTCPConnection -LocalPort 3000 -ErrorAction SilentlyContinue) -ne $null

# DNS
Resolve-DnsName "google.com"                               # DNS lookup
Resolve-DnsName "google.com" -Type MX                     # specific record type
[System.Net.Dns]::GetHostAddresses("google.com")           # quick IP lookup

# HTTP requests (PowerShell built-in)
Invoke-WebRequest "https://example.com"                    # full response object
Invoke-WebRequest "https://example.com" | Select-Object StatusCode, Headers
(Invoke-WebRequest "https://example.com").Content          # body as string
Invoke-RestMethod "https://api.example.com/data"           # auto-parses JSON → PSObject
$r = Invoke-RestMethod "https://api.github.com/repos/microsoft/vscode"
$r.stargazers_count

# POST with body
Invoke-RestMethod "https://api.example.com/create" `
    -Method POST `
    -ContentType "application/json" `
    -Body '{"name":"test"}' `
    -Headers @{ Authorization = "Bearer $token" }

# Check public IP
(Invoke-RestMethod "https://api.ipify.org?format=json").ip
```

### curl (Windows built-in alias or standalone)

```powershell
# In PS, curl is an alias for Invoke-WebRequest by default
# To use real curl.exe (ships with Windows 10+ and Git):
curl.exe https://example.com
curl.exe -s https://api.example.com/data             # -s = silent (no progress)
curl.exe -o file.zip https://example.com/file.zip    # save to file
curl.exe -L https://example.com                      # follow redirects
curl.exe -I https://example.com                      # headers only
curl.exe -X POST https://api.example.com/data \
    -H "Content-Type: application/json" \
    -d '{"key":"value"}'
curl.exe -u user:pass https://example.com            # basic auth
curl.exe --retry 3 https://example.com               # retry on failure

# Remove PS alias to use curl.exe directly as "curl"
Remove-Alias curl -ErrorAction SilentlyContinue
```

---

## Git

```powershell
# Setup
git config --global user.name  "Name"
git config --global user.email "email@example.com"
git config --global core.editor "code --wait"         # VS Code as editor
git config --list                                      # show all config
git config --global --list                             # global only

# Init / Clone
git init                               # new repo in current dir
git init "my-project"                  # create folder + init
git clone https://github.com/user/repo
git clone https://github.com/user/repo my-folder       # clone into specific folder
git clone --depth 1 https://github.com/user/repo       # shallow clone (faster)

# Status & Log
git status
git status -s                          # short format
git log
git log --oneline                      # compact: hash + message
git log --oneline --graph --all        # visual branch graph
git log --oneline -10                  # last 10 commits
git log --author="Name" --oneline
git log --since="2 weeks ago" --oneline
git diff                               # unstaged changes
git diff --staged                      # staged changes
git show abc1234                       # show specific commit

# Staging & Committing
git add file.txt                       # stage specific file
git add .                              # stage everything
git add -p                             # interactive staging (review chunks)
git commit -m "message"
git commit -am "message"               # stage tracked files + commit
git commit --amend -m "new message"    # fix last commit message (unpushed only)
git commit --amend --no-edit           # add staged files to last commit

# Branches
git branch                             # list local branches
git branch -a                          # list all including remote
git branch feature/my-feature         # create branch
git checkout feature/my-feature        # switch to branch
git checkout -b feature/my-feature     # create + switch
git switch main                        # modern switch (PS7 friendly)
git switch -c feature/new              # create + switch (modern)
git branch -d feature/done            # delete merged branch
git branch -D feature/force           # force delete
git branch -m old-name new-name        # rename branch

# Remote
git remote -v                          # list remotes
git remote add origin https://...
git remote set-url origin https://...  # change remote URL
git fetch origin                       # fetch without merge
git fetch --all                        # fetch all remotes
git pull                               # fetch + merge
git pull --rebase                      # fetch + rebase (cleaner history)
git push origin main
git push -u origin feature/my-feature  # push + set upstream
git push --force-with-lease            # CONFIRM — force push

# Rebase
git rebase main                        # rebase current branch onto main
git rebase -i HEAD~3                   # interactive rebase last 3 commits
# In interactive rebase: pick, squash (s), reword (r), drop (d)
git rebase --continue                  # after resolving conflict
git rebase --abort                     # cancel rebase

# Stash
git stash                              # stash current changes
git stash push -m "wip: feature x"    # stash with name
git stash list                         # show all stashes
git stash pop                          # apply last stash + remove it
git stash apply stash@{1}             # apply specific stash, keep it
git stash drop stash@{0}              # delete specific stash
git stash clear                        # delete all stashes
git stash show -p stash@{0}           # show stash diff

# Cherry-pick
git cherry-pick abc1234                # apply specific commit to current branch
git cherry-pick abc1234..def5678       # range of commits
git cherry-pick --no-commit abc1234    # apply changes without committing

# Bisect — find which commit introduced a bug
git bisect start
git bisect bad                         # current commit is bad
git bisect good v1.0                   # last known good tag/commit
# git will checkout commits; test each one, then:
git bisect good                        # mark current as good
git bisect bad                         # mark current as bad
git bisect reset                       # done, return to original branch

# Undo
git restore file.txt                   # discard unstaged changes in file
git restore --staged file.txt          # unstage file (keep changes)
git reset HEAD~1                       # undo last commit, keep changes staged
git reset --soft HEAD~1                # undo last commit, keep changes staged
git reset --mixed HEAD~1               # undo last commit, unstage changes
git reset --hard HEAD~1                # BLOCKED — discards changes
git revert abc1234                     # create new commit that undoes a commit (safe)
git clean -fd                          # BLOCKED — deletes untracked files
git clean -n                           # dry run — show what would be deleted

# Tags
git tag                                # list tags
git tag v1.0.0                         # lightweight tag
git tag -a v1.0.0 -m "Release 1.0"    # annotated tag
git push origin v1.0.0                 # push specific tag
git push origin --tags                 # push all tags

# Exit codes: 0 = success, 1 = diff found (git diff), 128 = fatal error
# Check: if ($LASTEXITCODE -ne 0) { "git command failed" }
```

### GitHub CLI (gh)

```powershell
gh auth login                          # authenticate
gh auth status                         # check auth

# Repos
gh repo clone owner/repo               # clone repo
gh repo create my-repo --public        # create new public repo
gh repo view                           # open current repo in browser

# Pull Requests
gh pr create --title "Fix bug" --body "Details" --base main
gh pr create --draft                   # draft PR
gh pr list                             # list open PRs
gh pr view 42                          # view PR #42
gh pr checkout 42                      # checkout PR branch locally
gh pr merge 42 --squash                # merge PR
gh pr close 42
```

---

## Node / npm / pnpm

```powershell
node --version ; npm --version ; pnpm --version

# npm
npm install                            # install from package.json
npm install express                    # CONFIRM — installs packages
npm install -D typescript              # add devDependency
npm install -g typescript              # global install
npm uninstall express  # CONFIRM
npm update                             # update all to allowed versions
npm outdated                           # show outdated packages
npm run build                          # run script
npm run dev
npm list                               # installed packages (current project)
npm list -g                            # global packages
npm init -y                            # create package.json with defaults
npm ci                                 # clean install (uses package-lock.json exactly)
npm audit                              # check for vulnerabilities
npm audit fix

# pnpm (preferred — faster, disk-efficient)
pnpm install                           # install from package.json
pnpm add express                       # CONFIRM — installs packages
pnpm add -D typescript                 # add devDependency
pnpm add -g typescript                 # global install
pnpm remove express  # CONFIRM
pnpm update
pnpm outdated
pnpm run build
pnpm run dev
pnpm list
pnpm init                              # create package.json
pnpm dlx create-next-app               # run package without installing (like npx)
pnpm store prune                       # clean unused packages from store

# Common package.json scripts pattern
# "scripts": { "dev": "...", "build": "...", "start": "..." }
npm run dev   ; pnpm dev               # pnpm can omit "run" for scripts
npm run build ; pnpm build
```

---

## Go

```powershell
go version

# Modules
go mod init github.com/user/project    # create go.mod
go mod tidy                            # add missing, remove unused deps
go mod download                        # download deps to cache
go get github.com/some/package         # add dependency
go get github.com/some/package@v1.2.3 # specific version
go get github.com/some/package@latest

# Build & Run
go run main.go                         # compile + run (no binary output)
go run .                               # run package in current dir
go build                               # compile to binary (uses module name)
go build -o app.exe                    # compile with specific output name
go build ./...                         # build all packages in project
go install                             # compile + install to $GOPATH/bin

# Test
go test ./...                          # run all tests
go test ./... -v                       # verbose
go test ./... -run TestName            # specific test
go test -cover ./...                   # with coverage

# Tools
go fmt ./...                           # format all code
go vet ./...                           # check for issues
go clean -cache                        # CONFIRM — clears cache

# Env
$env:GOPATH                            # workspace dir (default: $HOME\go)
$env:GOROOT                            # Go installation dir
go env                                 # show all Go env vars
go env GOPATH
```

---

## Python

```powershell
python --version ; python3 --version
pip --version

# Virtual environments
python -m venv .venv                   # create venv in .venv folder
.\.venv\Scripts\Activate.ps1           # activate (PowerShell)
deactivate                             # deactivate

# pip
pip install requests                   # install package
pip install requests==2.31.0           # specific version
pip install -r requirements.txt        # install from file
pip install -e .                       # editable install (for local packages)
pip uninstall requests
pip list                               # installed packages
pip list --outdated
pip freeze > requirements.txt          # save current packages to file
pip show requests                      # info about package

# uv — fast Python package manager (pip replacement)
# Install: pip install uv  OR  winget install astral-sh.uv
uv --version
uv venv                                # create venv faster than python -m venv
uv pip install requests                # install (faster than pip)
uv pip install -r requirements.txt
uv pip freeze
uv pip compile requirements.in -o requirements.txt   # lock deps
uv run python script.py                # run script in managed env
uv init my-project                     # create new project with pyproject.toml
```

---

## Archives (tar / zip)

```powershell
# PowerShell built-in (Compress / Expand)
Compress-Archive -Path "folder" -DestinationPath "archive.zip"
Compress-Archive -Path "folder" -DestinationPath "archive.zip" -Force   # overwrite
Compress-Archive -Path @("file1.txt", "file2.txt") -DestinationPath "out.zip"
Expand-Archive   -Path "archive.zip" -DestinationPath "output-folder"
Expand-Archive   -Path "archive.zip" -DestinationPath "." -Force        # extract here

# tar (built into Windows 10+)
tar -czf archive.tar.gz folder/        # create .tar.gz
tar -cjf archive.tar.bz2 folder/       # create .tar.bz2
tar -cf  archive.tar folder/           # create .tar (no compression)
tar -xzf archive.tar.gz                # extract .tar.gz here
tar -xzf archive.tar.gz -C output/    # extract to folder
tar -tzf archive.tar.gz                # list contents without extracting
tar -xzf archive.tar.gz specific/file  # extract single file

# zip / unzip (available if installed, e.g. via Git for Windows)
zip -r archive.zip folder/
unzip archive.zip
unzip archive.zip -d output/
unzip -l archive.zip                   # list contents

# 7-Zip (if installed: winget install 7zip.7zip)
7z a archive.7z folder/               # create .7z
7z a archive.zip folder/              # create .zip
7z x archive.7z                       # extract here
7z x archive.7z -ooutput/             # extract to folder
7z l archive.7z                       # list contents
7z t archive.7z                       # test integrity
7z a archive.7z file.txt -p"password" # with password
```

---

## WinGet

```powershell
winget --version
winget --info                          # version, dirs, logs, links

# Search
winget search "visual studio code"
winget search --id Microsoft.          # by ID prefix
winget show Git.Git                    # detailed info
winget show --id Git.Git --versions    # list available versions

# Install
winget install Git.Git
winget install --id Git.Git --silent --accept-package-agreements --accept-source-agreements
winget install --id Git.Git --version 2.43.0          # specific version
winget install --id Git.Git --scope machine            # system-wide (needs admin)
winget install --id Git.Git --scope user               # current user only

# Upgrade
winget upgrade --id Git.Git
winget upgrade --all --silent --accept-package-agreements --accept-source-agreements
winget upgrade --all --include-unknown                 # include apps without version info

# List installed
winget list                            # all installed apps (including non-winget)
winget list --upgrade-available        # only apps with updates
winget list --id Git.Git               # check specific app

# Uninstall
winget uninstall --id Git.Git --silent

# Pin (prevent upgrade)
winget pin add    --id Git.Git         # pin to current version
winget pin add    --id Git.Git --version 2.43.0
winget pin remove --id Git.Git
winget pin list

# Export / Import
winget export -o apps.json             # export installed apps list
winget export -o apps.json --include-versions
winget import -i apps.json --accept-package-agreements --accept-source-agreements --ignore-unavailable

# Sources
winget source list
winget source update                   # refresh catalog
winget source reset --force            # reset to defaults

# Misc
winget download --id Git.Git           # download installer without installing
winget hash -f installer.exe           # compute SHA-256
winget settings                        # open settings.json

# Exit codes: 0 = success, -1978335212 (0x8A15002C) = no updates available
# Automation pattern:
winget upgrade --all --silent --accept-package-agreements --accept-source-agreements
if ($LASTEXITCODE -ne 0 -and $LASTEXITCODE -ne -1978335212) {
    Write-Error "winget upgrade failed: $LASTEXITCODE"
}
```

### Common Package IDs

```
Git.Git                          Microsoft.VisualStudioCode
Microsoft.WindowsTerminal        Microsoft.PowerShell
Microsoft.DotNet.SDK.9           Rustlang.Rustup
GoLang.Go                        Python.Python.3.12
OpenJS.NodeJS.LTS                pnpm.pnpm
Docker.DockerDesktop             Neovim.Neovim
JetBrains.Toolbox                JetBrains.Rider
JetBrains.GoLand                 JetBrains.WebStorm
GitHub.GitHubDesktop             GitHub.cli
7zip.7zip                        Notepad++.Notepad++
ShareX.ShareX                    Obsidian.Obsidian
Postman.Postman                  Bruno.Bruno
astral-sh.uv                     BurntSushi.ripgrep.MSVC
sharkdp.fd                       ajeetdsouza.zoxide
```

---

## Useful One-Liners

```powershell
# Find large files
Get-ChildItem C:\ -Recurse -File -ErrorAction SilentlyContinue |
    Sort-Object Length -Descending | Select-Object -First 20 |
    Select-Object FullName, @{N="MB"; E={[math]::Round($_.Length/1MB,2)}}

# Tail a log file live
Get-Content "app.log" -Wait -Tail 50

# Replace text in file
(Get-Content "file.txt") -replace "old-value", "new-value" | Set-Content "file.txt"

# Count lines
(Get-Content "file.txt").Count

# Kill all processes matching name
Get-Process | Where-Object { $_.Name -like "chrome*" } | Stop-Process -Force

# Check if port is listening
(Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue) -ne $null

# Get public IP
(Invoke-RestMethod "https://api.ipify.org?format=json").ip

# All listening ports with owning process name
Get-NetTCPConnection -State Listen |
    Select-Object LocalPort, OwningProcess,
        @{N="ProcessName"; E={(Get-Process -Id $_.OwningProcess -ErrorAction SilentlyContinue).Name}} |
    Sort-Object LocalPort

# Find files modified in last 24h
Get-ChildItem -Recurse | Where-Object { $_.LastWriteTime -gt (Get-Date).AddDays(-1) }

# Check if running as admin
([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(
    [Security.Principal.WindowsBuiltInRole]::Administrator)

# Download file
Invoke-WebRequest "https://example.com/file.zip" -OutFile "file.zip"
curl.exe -L "https://example.com/file.zip" -o "file.zip"
```

---

## Agent Notes

- Use `$?` after cmdlets, `$LASTEXITCODE` after native executables (`git`, `winget`, `node`, etc.)
- `0 = success` for all native tools; non-zero means failure unless noted otherwise
- `Write-Host` output is NOT captured in pipelines — use `Write-Output` for capturable output
- To capture output of a command: `$out = command; $out`
- Pipe to `Out-String` when you need plain text from object output: `Get-Process | Out-String`
- For winget automation always add: `--silent --accept-package-agreements --accept-source-agreements`
- Avoid `Invoke-Expression` (eval) — prefer direct cmdlet calls
- `Remove-Item` with `-Recurse -Force` is immediate and unrecoverable — no Recycle Bin
- Prefer `pwsh` (PS7) over `powershell` (PS5) — more features, faster, active development
- `$env:USERPROFILE` = `C:\Users\<username>` on Windows
- String comparison is case-insensitive by default; prefix with `c` for case-sensitive: `-ceq`, `-clike`, `-cmatch`