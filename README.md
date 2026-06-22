# warden

[![CI](https://github.com/elev1e1nSure/warden/actions/workflows/ci.yml/badge.svg)](https://github.com/elev1e1nSure/warden/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/elev1e1nSure/warden?label=latest)](https://github.com/elev1e1nSure/warden/releases/latest)

CLI computer-control agent for Windows. Go TUI + Python backend + Ollama or OpenRouter.

![demo](assets/warden-gif.gif)

## Download

[Download latest release for Windows x64](https://github.com/elev1e1nSure/warden/releases/latest)

The release ZIP contains `warden.exe` (TUI frontend) and `warden-backend.exe` (Python backend). Keep both in the same folder.

## Stack

| Layer | Technology |
|---|---|
| Frontend | Go 1.24+, bubbletea, lipgloss, glamour |
| Backend | Python 3.11+, aiohttp |
| LLM | Ollama (local) or OpenAI-compatible APIs (OpenRouter) |
| Computer use | pyautogui, Pillow |
| Browser | Playwright |
| Search | duckduckgo-search |

## Architecture

```
Go TUI (bubbletea)
    |  HTTP NDJSON over localhost:8765
Python backend (aiohttp)
    |  Ollama SDK / OpenAI client
LLM (Ollama or remote API)
    |  Tool calls
[PowerShell] [filesystem] [screenshot] [mouse/keyboard] [browser] [search]
```

Frontend and backend are strictly separated: the TUI knows nothing about Ollama, the backend knows nothing about the UI.

## Project structure

```
warden/
├── go/
│   ├── cmd/warden/          # Entry point (boots backend + starts TUI)
│   ├── client.go            # HTTP client to backend
│   ├── commands.go          # Slash command handlers
│   ├── keys*.go             # Key bindings
│   ├── model.go             # Main tea.Model
│   ├── slash.go             # Slash command autocomplete
│   ├── status.go            # Status bar and input
│   ├── update.go            # Self-updater
│   ├── view.go              # Layout glue
│   ├── blocks.go            # Message rendering
│   ├── diff.go              # Diff viewer
│   ├── markdown.go          # Markdown parser
│   ├── styles.go            # lipgloss styles
│   └── ...
├── agent/
│   ├── server.py            # aiohttp server, routes
│   ├── chat.py              # Chat session & streaming
│   ├── llm_client.py        # Ollama / OpenAI client
│   ├── prompt.py            # System prompt builder
│   ├── tool_runner.py       # Tool dispatch & execution
│   ├── confirmations.py     # User confirmation flow
│   ├── skills.py            # Skill discovery & loading
│   ├── logger.py            # Structured logging
│   ├── ollama_process.py    # Ollama lifecycle
│   ├── memory/              # SQLite-backed persistent memory
│   ├── safety/              # Risk classification & policies
│   ├── tools/               # Tool implementations
│   └── test_*.py            # pytest suite
├── .warden/
│   └── skills/              # Built-in skills
├── justfile                 # Task runner
├── requirements.txt         # Python dependencies
└── run_backend.py           # PyInstaller entry point
```

## Build

All tasks use [just](https://github.com/casey/just):

```powershell
just build              # Go TUI → warden.exe
just build-backend      # Python backend → dist/warden-backend.exe (requires pyinstaller)
just release            # both
```

## Run

```powershell
.\warden.exe
```

On first run a connection wizard opens. Pick a provider and model, or pre-configure `~/.warden-config.json`:

```json
{
  "provider": "openrouter",
  "api_url": "https://openrouter.ai/api/v1",
  "api_key": "sk-or-v1-...",
  "model": "poolside/laguna-m.1:free"
}
```

With no config the backend stays unconnected until you run `/connect` or set the `WARDEN_MODEL` environment variable.

### How the backend starts

`warden.exe` auto-starts the backend before the TUI appears:

1. Looks for `warden-backend.exe` next to itself. If found, runs it.
2. Otherwise runs `python -m agent.server` after ensuring `requirements.txt` deps are installed.
3. Polls `http://localhost:8765/health` until ready.

Logs are written to `.warden/backend.out.log` and `.warden/backend.err.log`.

## Computer use

The agent can see and drive the screen. The loop is: `screenshot` -> look at the image -> `mouse` / `keyboard` -> `screenshot` again to confirm.

- **Vision** -- every `screenshot` is attached to the conversation as an image, so a vision-capable model literally sees the screen. Pick a model with image support (e.g. `qwen2.5vl` / `llava` Ollama model, or any vision model on OpenRouter); text-only models are blind to screenshots.
- **Coordinates** -- screenshots are downscaled before being sent to the model. The agent points at pixels **in the image it was shown**, and warden maps those back to real screen pixels automatically. No manual scaling.
- **`mouse`** -- `move`, `click`, `right_click`, `double_click`, `scroll`, and `drag` (give `x`/`y` as the start and `x2`/`y2` as the drop point).
- **`keyboard`** -- `type` writes text (non-ASCII like Cyrillic/emoji is pasted via the clipboard so it lands correctly), `press` sends keys and combos (`ctrl+c`, `alt+f4`, `win+d`).

A real screen corner is the pyautogui fail-safe: slam the cursor into the top-left corner to abort an automated action.

## Controls

| Key | Action |
|---|---|
| `Enter` | send |
| `Esc` | interrupt stream |
| `Esc` x2 | force-stop |
| `Esc` (in `/select` mode) | exit text selection |
| `Shift+Tab` | toggle Ask / Auto mode |
| `Tab` | complete slash command or skill name |
| `Up` / `Down` | scroll during stream; navigate input history when idle |
| `Scroll wheel` | scroll (5 lines per tick) |
| `Ctrl+W` | delete last word in input |
| `Ctrl+C` | exit (force-quit if streaming) |

## Slash commands

| Command | Action |
|---|---|
| `/connect` | set up provider and model |
| `/clear` | clear chat and reset session |
| `/compact` | summarize context to free up token budget |
| `/memory` | toggle or show memory settings (`on`, `off`, `clear`, `status`) |
| `/models` | switch model (interactive picker) |
| `/select` | enable text selection (disables mouse capture) |
| `/update` | download and install the latest release |
| `/verbose` | toggle verbose mode (show tool lines and diffs) |

## Skills

Skills are Markdown instruction sets the agent can invoke.

```
! <skill-name>          # invoke a skill
! <cmd>                 # run a PowerShell command directly
!                       # list available skills
```

Skills live in `.warden/skills/<name>/SKILL.md` (project) or `~/.warden/skills/<name>/SKILL.md` (global). Fallback `.claude/skills/`.

## Memory

warden has a SQLite-backed persistent memory layer (`~/.warden/memory.db`). It stores:

- **Entries** -- short-term facts per session (user info, tech stack, preferences, projects)
- **Snapshots** -- long-term aggregated summaries written at session end

Memory is injected into the system prompt as a `[Memory]` context block each turn. Toggle with `/memory on` / `/memory off`, inspect with `/memory status`, and clear with `/memory clear`.

## Safety

Risk classification is done by code, not the model:

- **safe** -- read-only: `Get-ChildItem`, `git status`, `screenshot`, `file_read` inside workspace, `browser_read`, `process_list`, `image_locate`, `ocr`
- **confirm** -- file writes, installs, mouse/keyboard, process kills, unknown binaries, external URLs
- **blocked** -- forced recursive delete, encoded commands, remote eval, disk format, registry changes, writes outside workspace, `file_move`/`file_copy` outside workspace

In **Ask** mode, confirm-level actions require `y` / `n` before executing. In **Auto** mode they run immediately (except `file_delete`, which always requires confirmation). Blocked actions are always rejected.

Path safety uses `Path.resolve()` with containment checks. UNC paths, device paths (`\\.\`, `\\?\`) and directory traversal (`../`) are blocked.

## Tools

`powershell` `bash` `file_read` `file_write` `file_delete` `file_list` `file_move` `file_copy` `glob` `grep` `edit` `apply_patch` `clipboard` `screenshot` `mouse` `keyboard` `browser_open` `browser_read` `browser_screenshot` `browser_click` `browser_fill` `google_search` `youtube_search` `webfetch` `archive` `process_list` `process_kill` `skill` `todowrite` `question` `window_list` `window_focus` `window_manage` `image_locate` `ocr` `wait_for` `system_info` `notify` `memory` `http_request` `lsp`

## Tests

```powershell
just install            # install dependencies
just test               # pytest (coverage)
just test --no-cov -q   # quick, no coverage
just test-go            # Go tests
just lint-go            # Go vet
just fmt-go             # Go format check
```

## Troubleshooting

| Symptom | Fix |
|---|---|
| `port 8765 is busy` | Another warden instance is running. Close it or run `taskkill /F /IM warden.exe` |
| `ollama is not responding` | Install Ollama from [ollama.com](https://ollama.com) and start it |
| `pip install failed` | Run `pip install -r requirements.txt` manually |
| Backend exits immediately | Check `.warden/backend.err.log` for the error |

## License

MIT
