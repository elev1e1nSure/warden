# warden

CLI computer control agent. Go TUI + Python backend + Ollama.

## stack

| layer | technology |
|---|---|
| frontend | go 1.21+, bubbletea, lipgloss |
| backend | python 3.11+, aiohttp |
| llm | ollama (qwen3:8b) |
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
[bash] [filesystem] [screenshot] [mouse/keyboard] [browser] [search]
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
│   ├── styles.go        # lipgloss styles
│   └── logger.go        # frontend logs
├── agent/
│   ├── server.py        # aiohttp backend
│   ├── chat.py          # session and streaming
│   ├── ollama_client.py # ollama management
│   ├── tools.py         # agent tools
│   └── logger.py        # backend colored logs
├── requirements.txt
├── README.md
└── CLAUDE.md
```

## launch

```bash
# From the go/ directory:
cd go

# launcher starts backend + frontend together
go run ./cmd/warden

# or build and run
go build -o warden.exe ./cmd/warden && ./warden.exe
```

backend starts on `localhost:8765`, automatically starts ollama and downloads the model if needed.

## tools

| name | description |
|---|---|
| `bash` | PowerShell commands |
| `file_read` | read file |
| `file_write` | write file (creates folders) |
| `file_delete` | delete file, only within cwd |
| `file_list` | list files and folders |
| `clipboard` | read / write clipboard |
| `screenshot` | take a screenshot |
| `mouse` | move, click, right_click, double_click, scroll |
| `keyboard` | type, press (hotkey) |
| `browser_open` | open URL in user's browser |
| `browser_read` | read page text |
| `browser_screenshot` | screenshot page |
| `youtube_search` | search YouTube videos |
| `google_search` | web search (DuckDuckGo) |

## slash commands

Entered in the message field:

| command | action |
|---|---|
| `/auto` | auto mode — dangerous commands without confirmation |
| `/safe` | safe mode — confirmation for dangerous commands |
| `/reset` | reset session |
| `/thinking` | toggle model reasoning |

## security

- `bash`: dangerous patterns (`rm -rf`, `format`, `rmdir`, etc.) require confirmation in safe mode
- `file_delete`: only within cwd, always requires confirmation
- `file_write`: outside cwd requires confirmation
- in TUI: `y` + Enter — confirm, Esc — cancel

## model

Recommended: `qwen3:8b`

```bash
ollama run qwen3:8b
```
