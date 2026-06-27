# Warden: Python → Go migration (single binary)

Kill the Python backend entirely. Move all logic into Go. TUI and agent live in one process, communicate via Go channels instead of HTTP/NDJSON.

## Architecture After Migration

```
warden.exe (single binary)
├── main.go               — entry point
├── tui/                   — existing bubbletea TUI (minimal changes)
├── agent/                 — NEW: all backend logic in Go
│   ├── session.go         — ChatSession (agent loop, history, cancellation)
│   ├── llm/
│   │   ├── client.go      — LLMClient interface
│   │   ├── ollama.go      — Ollama streaming client (REST API directly)
│   │   └── openai.go      — OpenAI-compatible streaming client (SSE)
│   ├── tools/
│   │   ├── registry.go    — Tool interface + registry
│   │   ├── shell.go       — powershell, bash
│   │   ├── files.go       — read, write, edit, glob, grep, delete, list
│   │   ├── move.go        — file_move, file_copy
│   │   ├── patch.go       — apply_patch (unified diff)
│   │   ├── browser.go     — playwright-go (read, screenshot, click, fill, open)
│   │   ├── input.go       — screenshot, mouse, keyboard, clipboard (robotgo)
│   │   ├── screen.go      — image_locate, ocr, wait_for
│   │   ├── search.go      — google_search, webfetch, youtube_search
│   │   ├── http.go        — http_request
│   │   ├── window.go      — window_list, window_focus, window_manage (WinAPI)
│   │   ├── process.go     — process_list, process_kill
│   │   ├── archive.go     — zip/tar create, extract, list
│   │   ├── lsp.go         — LSP JSON-RPC client
│   │   ├── misc.go        — question, todowrite, skill, system_info, notify
│   │   └── memory_tool.go — memory tool (read/write/search/delete)
│   ├── safety/
│   │   ├── policy.go      — assess_tool_call → SafetyDecision
│   │   ├── filesystem.go  — is_dangerous_path, is_path_within_workspace
│   │   └── powershell.go  — classify PowerShell commands
│   ├── memory/
│   │   ├── store.go       — SQLite store (go-sqlite3)
│   │   ├── extractor.go   — heuristic fact extraction from messages
│   │   └── aggregator.go  — snapshot builder + merge
│   ├── skills/
│   │   └── skills.go      — discover, parse frontmatter, catalog
│   ├── prompt.go          — system prompt builder
│   ├── confirm.go         — ConfirmationManager + QuestionManager (channels)
│   └── runner.go          — tool runner (safety → confirm → execute → truncate)
└── internal/
    ├── security/          — existing (API key encryption)
    └── client/            — DELETE: HTTP client no longer needed
```

---

## 10 Steps

### Step 1: Agent interface + event types

Replace `Backend` interface. Instead of HTTP client methods, it exposes Go channels.

#### [MODIFY] [backend.go](file:///d:/Projects/warden/backend.go)

Current interface returns `<-chan client.Event` from `StreamChat`. New interface:

```go
type Backend interface {
    StreamChat(text string, skill string, args string) <-chan Event
    Interrupt()
    ResetSession()
    SetMode(auto bool)
    SendConfirm(id string, ok bool)
    SendQuestion(id string, answers [][]string)
    GetStatus() StatusResult
    Compact() CompactResult
    // ... same shape, but direct Go calls, no HTTP
}
```

Event types stay the same (already defined in `internal/client/stream.go`), just move them to root package or a new `agent/` package. No more JSON serialization.

**Files:** `backend.go`, new `agent/events.go`
**Effort:** ~100 lines

---

### Step 2: LLM clients (Ollama + OpenAI)

Port [llm_client.py](file:///d:/Projects/warden/agent/llm_client.py) (234 lines).

- **Ollama**: HTTP POST to `localhost:11434/api/chat` with `stream: true`, read NDJSON. No SDK needed — raw `net/http`. Simpler than Python because Go streams naturally.
- **OpenAI-compatible**: HTTP POST to `{base_url}/chat/completions` with `stream: true`, read SSE (`data: {...}`). Handle `tool_calls` delta accumulation. Retry logic for unsupported features (tool_choice, reasoning).

Go libs: `net/http` (zero deps), or optionally `sashabaranov/go-openai` for the OpenAI client.

**Files:** `agent/llm/client.go`, `agent/llm/ollama.go`, `agent/llm/openai.go`
**Effort:** ~400 lines, direct port from Python's 234

---

### Step 3: Safety policy

Port [_policy.py](file:///d:/Projects/warden/agent/safety/_policy.py) (325 lines), [_filesystem.py](file:///d:/Projects/warden/agent/safety/_filesystem.py) (43 lines), [_powershell.py](file:///d:/Projects/warden/agent/safety/_powershell.py) (273 lines).

This is almost pure string matching + regex — translates to Go trivially. `regexp` stdlib covers everything.

**Files:** `agent/safety/policy.go`, `agent/safety/filesystem.go`, `agent/safety/powershell.go`
**Effort:** ~600 lines, mechanical translation

---

### Step 4: Confirmations + Questions

Port [confirmations.py](file:///d:/Projects/warden/agent/confirmations.py) (131 lines).

Python uses `asyncio.Event` + `asyncio.wait_for`. Go equivalent: `chan struct{}` with `context.WithTimeout`. Actually simpler in Go because channels are first-class.

**Files:** `agent/confirm.go`
**Effort:** ~120 lines

---

### Step 5: Tools — core set (shell, files, patch, search, http, process)

Port the stateless tools that don't need external dependencies:

| Tool file | Lines | Go approach |
|---|---|---|
| [shell.py](file:///d:/Projects/warden/agent/tools/shell.py) | 60 | `os/exec` |
| [files.py](file:///d:/Projects/warden/agent/tools/files.py) | 470 | `os`, `path/filepath`, `bufio` |
| [move.py](file:///d:/Projects/warden/agent/tools/move.py) | 110 | `os.Rename`, `io.Copy` |
| [patch.py](file:///d:/Projects/warden/agent/tools/patch.py) | 560 | `sourcegraph/go-diff` or hand-roll |
| [search.py](file:///d:/Projects/warden/agent/tools/search.py) | 180 | `net/http` + HTML parsing |
| [http.py](file:///d:/Projects/warden/agent/tools/http.py) | 110 | `net/http` |
| [process.py](file:///d:/Projects/warden/agent/tools/process.py) | 160 | `os/exec` (`tasklist`, `taskkill`) |
| [archive.py](file:///d:/Projects/warden/agent/tools/archive.py) | 220 | `archive/zip`, `archive/tar` |
| [system.go](file:///d:/Projects/warden/agent/tools/system.py) | 180 | `os`, `runtime`, WinAPI toast |
| [misc.py](file:///d:/Projects/warden/agent/tools/misc.py) | 140 | Pure Go structs |

**Files:** `agent/tools/*.go`
**Effort:** ~1500 lines total, biggest chunk of work but each is independent

---

### Step 6: Tools — desktop control (screenshot, mouse, keyboard, OCR, window)

The Windows-specific tools. These are the riskiest because of external library dependencies:

| Tool | Python lib | Go lib |
|---|---|---|
| Screenshot | `Pillow.ImageGrab` | `kbinani/screenshot` + `image/png` |
| Mouse/Keyboard | `pyautogui` | `go-vgo/robotgo` or WinAPI directly |
| Clipboard | PowerShell subprocess | `golang.design/x/clipboard` or PowerShell |
| OCR | Windows.Media.Ocr via PowerShell | Same PowerShell script via `os/exec` |
| Image locate | `pyautogui.locateOnScreen` | `robotgo` or template matching via `gocv` |
| Window list/focus/manage | ctypes WinAPI | `golang.org/x/sys/windows` (EnumWindows, SetForegroundWindow) |

> [!WARNING]
> `robotgo` requires CGO and has build dependencies (gcc). Alternative: call WinAPI directly via `golang.org/x/sys/windows` for `SendInput`, which avoids CGO but is more code.

**Files:** `agent/tools/input.go`, `agent/tools/screen.go`, `agent/tools/window.go`
**Effort:** ~500 lines

---

### Step 7: Tools — browser (playwright-go)

Port [browser.py](file:///d:/Projects/warden/agent/tools/browser.py) (342 lines).

`playwright-community/playwright-go` API is 1:1 with Python. Session management (persistent page for click/fill) translates directly.

```go
import "github.com/playwright-community/playwright-go"
```

Requires `playwright install chromium` at first run.

**Files:** `agent/tools/browser.go`
**Effort:** ~350 lines, near 1:1 port

---

### Step 8: Memory system + Skills

Port memory ([store.py](file:///d:/Projects/warden/agent/memory/store.py) 263 lines, [extractor.py](file:///d:/Projects/warden/agent/memory/extractor.py) 223 lines, [aggregator.py](file:///d:/Projects/warden/agent/memory/aggregator.py) 90 lines) and skills ([skills.py](file:///d:/Projects/warden/agent/skills.py) 216 lines).

- SQLite: `mattn/go-sqlite3` (CGO) or `modernc.org/sqlite` (pure Go, no CGO)
- Extractor: regex-based, `regexp` stdlib
- Skills: filesystem scan, YAML frontmatter parsing

**Files:** `agent/memory/store.go`, `agent/memory/extractor.go`, `agent/memory/aggregator.go`, `agent/skills/skills.go`
**Effort:** ~700 lines

---

### Step 9: ChatSession + tool runner (agent loop)

The core: port [chat.py](file:///d:/Projects/warden/agent/chat.py) (516 lines) and [tool_runner.py](file:///d:/Projects/warden/agent/tool_runner.py) (208 lines).

This is the agent loop:
1. Build system prompt (+ memory context + skills)
2. Call LLM with streaming
3. Parse `<think>` tags, emit token/think events
4. Collect tool calls
5. For each tool call: safety check → confirm → execute → record result
6. Loop until no more tool calls or max iterations

Go version uses channels instead of `async for`:

```go
func (s *Session) Stream(text string) <-chan Event {
    ch := make(chan Event, 64)
    go func() {
        defer close(ch)
        // agent loop here, sending events to ch
    }()
    return ch
}
```

This is the same shape the TUI already expects — `<-chan Event`.

**Files:** `agent/session.go`, `agent/runner.go`, `agent/prompt.go`
**Effort:** ~600 lines

---

### Step 10: TUI wiring + cleanup

1. **New `Backend` impl**: `AgentBackend` struct that wraps the Go `agent.Session` directly. Implements the `Backend` interface by calling Go functions, not HTTP.

2. **Kill `cmd/warden/main.go` launcher**: No more `startBackend()`, no health-check polling, no Python process management. `main.go` creates `AgentBackend` inline and passes to `tui.Run()`.

3. **Delete Python**: Remove `agent/` (Python), `requirements.txt`, `pyproject.toml`, `run_backend.py`, `pytest.ini`, PyInstaller config.

4. **Delete HTTP bridge**: Remove `internal/client/` (Go HTTP client), `client_adapter.go`.

5. **Update CLAUDE.md**: Remove Python references, update stack description.

6. **Update `justfile`**: Remove `pip`, `ruff`, `pytest` targets. Add `go test ./agent/...`.

**Files:** New `agent_backend.go`, modified `main.go`, deleted ~40 Python files
**Effort:** ~200 lines new code + deletions

---

## Open Questions

> [!IMPORTANT]
> **CGO dependency**: `robotgo` and `mattn/go-sqlite3` require CGO (C compiler). On Windows this means `gcc` via MSYS2 or similar. 
> - Alternative for SQLite: `modernc.org/sqlite` (pure Go, zero CGO)
> - Alternative for mouse/keyboard: direct WinAPI calls via `golang.org/x/sys/windows` (no CGO, more code)
> 
> Do you want to stay CGO-free or is gcc acceptable?

> [!IMPORTANT]
> **LSP tool**: The current Python [lsp.py](file:///d:/Projects/warden/agent/tools/lsp.py) (540 lines) is the most complex single tool. It implements a JSON-RPC client to talk to LSP servers. Should we:
> - Port it fully?
> - Drop it for now and add later?
> - Use `gopls` MCP integration instead?

> [!IMPORTANT]
> **Testing strategy**: Python has ~6100 lines of tests. Do we:
> - Port tests in parallel with each step?
> - Write Go tests from scratch after migration?
> - Port tests at the end as a batch?

## Verification Plan

### Automated Tests
- `go test ./agent/...` — unit tests for each package
- `go build ./...` — compile check after each step
- `go vet ./...` — static analysis

### Manual Verification
- After Step 2: verify LLM streaming works with Ollama and OpenRouter
- After Step 5: verify shell commands, file operations work
- After Step 7: verify browser tools work
- After Step 10: full end-to-end test — send a message, get response, confirm a tool, use skills
