# warden

[![CI](https://github.com/elev1e1nSure/warden/actions/workflows/ci.yml/badge.svg)](https://github.com/elev1e1nSure/warden/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

CLI computer control agent. Go TUI + Python backend + Ollama.

## stack

| layer | technology |
|---|---|
| frontend | Go 1.24+, bubbletea, lipgloss |
| backend | Python 3.11+, aiohttp |
| llm | Ollama (qwen3:8b) or remote OpenAI-compatible API (OpenRouter) |
| computer use | pyautogui, Pillow |
| browser | Playwright |
| search | duckduckgo-search |

## architecture

```
go tui (bubbletea)
    ↓ HTTP NDJSON
python backend (aiohttp, localhost:8765)
    ↓
ollama
    ↓
[powershell] [filesystem] [screenshot] [mouse/keyboard] [browser] [search]
```

frontend and backend are separated: TUI knows nothing about Ollama, backend knows nothing about UI.

## structure

```
warden/
├── go/
│   ├── cmd/warden/      # launcher (starts backend + frontend)
│   │   └── main.go
│   ├── main.go          # TUI entry (package tui)
│   ├── model.go         # bubbletea model
│   ├── client.go        # http client
│   ├── view.go          # rendering, tool lines, status bar
│   ├── slash.go         # slash command handling
│   ├── commands.go      # bubbletea cmds (backend check, send, confirm)
│   ├── styles.go        # lipgloss styles
│   ├── logger.go        # frontend logs
│   ├── markdown.go      # markdown rendering
│   ├── go.mod
│   └── go.sum
├── agent/
│   ├── server.py          # aiohttp backend
│   ├── chat.py            # session and streaming
│   ├── llm_client.py      # LLM abstraction (Ollama / OpenAI-compatible)
│   ├── ollama_process.py  # ollama process management
│   ├── confirmations.py   # confirmation and question managers
│   ├── safety/            # risk classification (safe / confirm / blocked)
│   │   ├── __init__.py
│   │   ├── _filesystem.py
│   │   ├── _policy.py
│   │   └── _powershell.py
│   ├── tools.py           # all agent tools
│   ├── logger.py          # backend colored logs
│   ├── test_safety.py
│   ├── test_tools_core.py
│   ├── test_tools_patch.py
│   ├── test_server.py
│   ├── test_llm_client.py
│   ├── test_chat_tool_execution.py
│   ├── test_logger.py
│   └── test_ollama_process.py
├── .warden/
│   └── powershell-reference.md  # command reference with risk markers
├── requirements.txt
├── README.md
└── CLAUDE.md
```

## launch

```bash
# from the go/ directory:
cd go

# launcher starts backend + frontend together
go run ./cmd/warden

# or build and run
go build -o warden.exe ./cmd/warden
./warden.exe
```

backend starts on `localhost:8765`, automatically starts Ollama and downloads the model if needed.

### remote API (OpenRouter)

```bash
# set your API key
$env:OPENROUTER_API_KEY="sk-or-v1-..."

# launch with OpenRouter
.\warden.exe --provider openrouter --model poolside/laguna-m.1:free

# or set the API URL explicitly
.\warden.exe --api https://openrouter.ai/api/v1 --model poolside/laguna-m.1:free
```

OpenRouter reasoning-capable models are supported via the `reasoning` parameter. Warden preserves `reasoning_details` across turns when the provider sends them.

| flag | description |
|---|---|
| `--provider` | `ollama` (default) or `openrouter`. Auto-sets `--api` for known providers. |
| `--api` | Override API base URL. |
| `--model` | Model name. Default: `qwen3:8b`. |

## tools

| name | description |
|---|---|
| `powershell` | PowerShell commands (Windows PowerShell, `pwsh` if available) |
| `bash` | Alias for `powershell` |
| `file_read` | read file with line numbers (offset/limit for partial reads) |
| `file_write` | write file (creates parent folders) |
| `file_delete` | delete file, only within cwd |
| `file_list` | list files and folders |
| `glob` | find files by glob pattern (e.g. `**/*.py`) |
| `grep` | search file contents by regex (uses ripgrep if available) |
| `edit` | replace specific string in file (must match exactly once) |
| `apply_patch` | apply unified-format patch to multiple files |
| `clipboard` | read / write clipboard |
| `screenshot` | take a screenshot |
| `mouse` | move, click, right_click, double_click, scroll |
| `keyboard` | type, press (hotkey) |
| `browser_open` | open URL in the default browser |
| `browser_read` | read page text via Playwright |
| `browser_screenshot` | screenshot page via Playwright |
| `youtube_search` | search YouTube videos |
| `google_search` | web search (DuckDuckGo) |
| `webfetch` | fetch content from URL (HTML, JSON, plain text) |
| `skill` | load local skill file and sample files |
| `todowrite` | create and maintain a structured task list |
| `question` | ask user questions during task (handled by chat loop) |

## modes

| mode | behavior |
|---|---|
| **Ask** (`/ask`, Shift+Tab) | Safe commands run immediately. Confirm-level commands require user approval. Blocked commands are rejected — the model sees the block as a tool result. |
| **Auto** (`/auto`, Shift+Tab) | Safe and confirm-level commands run without confirmation. Blocked commands are still rejected. Use with caution. |

## keyboard shortcuts

| key | action |
|---|---|
| `Enter` | send message |
| `Esc` | interrupt streaming |
| `Esc` ×2 | force-interrupt if stuck |
| `Shift+Tab` | toggle Ask / Auto mode |
| `F2` | expand / collapse last thinking block |
| `↑` / `↓` | scroll message history |
| `Ctrl+C` | exit |

## confirmation

When a tool call requires confirmation, the TUI shows:
- **what** will run (command / path / action)
- **why** it was flagged (risk details)
- **keys**: `y` to confirm, `Enter` / `Esc` / `n` to cancel

Confirmations time out after 5 minutes (auto-cancel).

## slash commands

| command | action |
|---|---|
| `/auto` | switch to Auto mode |
| `/ask` | switch to Ask mode |
| `/reset` | reset session and cancel pending confirmations |
| `/thinking` | toggle model reasoning |

## security

Risk classification is done by `agent/safety/`, not the model:

- **safe**: read-only commands (`Get-ChildItem`, `git status`, `screenshot`, `browser_read`, `file_read` inside workspace)
- **confirm**: file writes, installs, mouse clicks, keyboard input, external URLs, process kills, unknown binaries
- **blocked**: recursive forced delete, encoded commands, `Invoke-Expression` with remote content, disk format, registry/system changes, file delete outside workspace

Path safety uses `Path.resolve()` with containment checks. UNC paths, device paths (`\\.\`, `\\?\`), and traversal (`../`) are blocked.

PowerShell reference with risk markers: [`.warden/powershell-reference.md`](.warden/powershell-reference.md)

## tests

```bash
pip install -r requirements.txt
pytest agent/
```

87% statement coverage across the `agent/` package. Run after any change to `agent/safety/`.

```bash
# quick run without coverage report
pytest agent/ --no-cov -q

# single module
pytest agent/test_safety.py -v
```

## model

### local (Ollama)

Recommended: `qwen3:8b`

```bash
ollama run qwen3:8b
```

### remote (OpenRouter)

Set `OPENROUTER_API_KEY` and launch with `--provider openrouter --model <model-id>`.

Free quick-start model: `poolside/laguna-m.1:free`
