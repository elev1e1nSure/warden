# warden

CLI computer control agent. Go TUI + Python backend + Ollama.

## stack

| layer | technology |
|---|---|
| frontend | go 1.24+, bubbletea, lipgloss |
| backend | python 3.11+, aiohttp |
| llm | ollama (qwen3:8b) or remote OpenAI-compatible API (OpenRouter) |
| computer use | pyautogui, pillow |
| browser | playwright |
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
│   ├── view.go          # rendering, presence phrases, tool lines
│   ├── slash.go         # slash command handling
│   ├── commands.go      # bubbletea cmds (backend check, send, confirm)
│   ├── styles.go        # lipgloss styles
│   ├── logger.go        # frontend logs
│   ├── markdown.go      # markdown rendering
│   ├── go.mod           # Go dependencies
│   └── go.sum
├── agent/
│   ├── server.py          # aiohttp backend
│   ├── chat.py            # session and streaming
│   ├── llm_client.py      # LLM abstraction (Ollama / OpenAI-compatible)
│   ├── ollama_process.py  # ollama management
│   ├── confirmations.py   # dangerous tool confirmation manager
│   ├── safety.py          # risk classification (safe / confirm / blocked)
│   ├── test_safety.py     # safety tests (pytest)
│   ├── tools.py           # agent tools
│   └── logger.py          # backend colored logs
├── .warden/
│   └── powershell-reference.md  # command reference with risk markers
├── requirements.txt
├── README.md
├── CLAUDE.md
└── AGENTS.md
```

## launch

```bash
# From the go/ directory:
cd go

# launcher starts backend + frontend together
go run ./cmd/warden

# or build and run
go build -o warden.exe ./cmd/warden
./warden.exe
```

backend starts on `localhost:8765`, automatically starts ollama and downloads the model if needed.

### remote API (OpenRouter)

```bash
# Set your API key
$env:OPENROUTER_API_KEY="sk-or-v1-..."

# Launch with OpenRouter — provider auto-sets the API URL
.\warden.exe --provider openrouter --model poolside/laguna-m.1:free

# Or set the API URL explicitly
.\warden.exe --api https://openrouter.ai/api/v1 --model poolside/laguna-m.1:free
```

OpenRouter reasoning-capable models are enabled with the `reasoning` request parameter, and Warden preserves returned `reasoning_details` across turns when the provider sends them.

| flag | description |
|---|---|
| `--provider` | `ollama` (default) or `openrouter`. Auto-sets `--api` for known providers. |
| `--api` | Override API base URL. Used when `--provider` is not enough. |
| `--model` | Model name. Default: `qwen3:8b` locally; for OpenRouter, `poolside/laguna-m.1:free` is the quick start example. |

## tools

| name | description |
|---|---|
| `powershell` | PowerShell commands (Windows PowerShell, `pwsh` if available) |
| `bash` | Deprecated alias for `powershell` |
| `file_read` | read file with line numbers (offset/limit for partial reads) |
| `file_write` | write file (creates folders) |
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
| `browser_open` | open URL in user's browser |
| `browser_read` | read page text via Playwright |
| `browser_screenshot` | screenshot page via Playwright |
| `youtube_search` | search YouTube videos |
| `google_search` | web search (DuckDuckGo) |
| `webfetch` | fetch content from URL (HTML, JSON, plain text) |
| `skill` | load local skill file and sample files |
| `todowrite` | create and maintain structured task list |
| `question` | ask user questions during task (handled by chat loop) |

## modes

| mode | behavior |
|---|---|
| **Ask** (`/ask`) | Every tool call is classified by `agent/safety.py`. Safe commands run immediately. Confirm commands require user approval. Blocked commands are rejected and the model sees the block as a tool result. |
| **Auto** (`/auto`) | Safe and confirm-level commands run without confirmation. Blocked commands are still rejected. Use with caution. |

## confirmation

When a tool call requires confirmation, the TUI shows:
- **what** will run (command / path / action)
- **why** it was flagged (risk details)
- **keys**: `y` to confirm, `Enter` / `Esc` / `n` to cancel

Confirmations time out after 5 minutes (auto-cancel).

## slash commands

Entered in the message field:

| command | action |
|---|---|
| `/auto` | Auto — dangerous commands without confirmation |
| `/ask` | Ask — confirmation for dangerous commands |
| `/reset` | Reset session and cancel all pending confirmations |
| `/thinking` | Toggle model reasoning |

## security

Risk classification is done by `agent/safety.py`, not the model:

- **safe**: read-only commands (`Get-ChildItem`, `git status`, `screenshot`, `browser_read`, `file_read` inside workspace)
- **confirm**: file writes, installs, mouse clicks, keyboard input, external URLs, process kills, unknown binaries
- **blocked**: recursive forced delete, encoded commands, `Invoke-Expression` with remote content, disk format, registry/system changes, file delete outside workspace

Path safety uses `Path.resolve()` with containment checks. UNC paths, device paths (`\\.\`, `\\?\`), and traversal (`../`) are blocked.

PowerShell reference with risk markers: `.warden/powershell-reference.md`

## tests

```bash
pip install pytest
pytest agent/
```

Safety classification is covered by `agent/test_safety.py` — run it after any change to `agent/safety.py`.

## model

### local (Ollama)

Recommended: `qwen3:8b`

```bash
ollama run qwen3:8b
```

### remote (OpenRouter)

Set `OPENROUTER_API_KEY` and launch with `--api` and `--model`.

Quick start free model: `poolside/laguna-m.1:free`
