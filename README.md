# Warden

[![CI](https://github.com/elev1e1nSure/warden/actions/workflows/ci.yml/badge.svg)](https://github.com/elev1e1nSure/warden/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/elev1e1nSure/warden?label=latest)](https://github.com/elev1e1nSure/warden/releases/latest)
[![Windows](https://img.shields.io/badge/OS-Windows-blue)](https://github.com/elev1e1nSure/warden/releases/latest)

**AI agent that sits in your terminal and controls your Windows machine.** Warden sees your screen, runs commands, edits files, drives the browser, and does it all through a keyboard-first TUI — no IDE plugins, no Electron bloat, no cloud lock-in.

![demo](assets/warden-gif.gif)

---

## Install

[⬇️ Download latest release for Windows x64](https://github.com/elev1e1nSure/warden/releases/latest)

Unzip anywhere — two binaries, one folder, zero setup. Run `warden.exe` and pick your LLM provider on first launch.

```powershell
# Or build from source (requires Go 1.24+, Python 3.11+)
just release
```

---

## What it does

Warden is a general-purpose desktop agent. You give it natural-language instructions, it figures out the steps and executes them on your machine.

**Common use cases:**

- **Dev ops** — deploy, build, debug, grep through logs, restart services, check git status across repos
- **File & code work** — batch rename, search-and-replace, generate reports, format and lint multiple files
- **Browser automation** — scrape data, fill forms, navigate multi-step flows, take screenshots
- **System management** — read event logs, check disk usage, kill stuck processes, manage windows
- **Research** — search the web, read documentation, fetch APIs, summarize results back to you
- **Anything you'd script but don't want to write a script for**

Every action is visible in the chat stream. You stay in control — **Ask mode** pauses for confirmation on writes and mouse clicks, **Auto mode** lets the agent run freely.

---

## Quick tour

```
$ .\warden.exe

> find the largest 5 files on my desktop and zip them into archive.zip

  ✓ Screenshot taken
  ✓ Running: Get-ChildItem "$env:USERPROFILE\Desktop" -Recurse -File | sort Length -Descending | select -First 5
  ✓ Compress-Archive ...
  Done. Created C:\Users\you\Desktop\archive.zip (142 MB)

> /connect          # set up provider and model
> /models           # switch models on the fly
> Tab               # autocomplete slash commands and skills
> Shift+Tab         # toggle Ask / Auto mode
> Esc               # interrupt the agent mid-stream
```

---

## Stack

| Layer | Technology |
|---|---|
| TUI | Go 1.24+, [bubbletea](https://github.com/charmbracelet/bubbletea), lipgloss, glamour |
| Backend | Python 3.11+, aiohttp |
| LLM | Ollama (local) or any OpenAI-compatible API (OpenRouter, etc.) |
| Screen & input | pyautogui, Pillow |
| Browser | Playwright (headless or visible) |
| Search | duckduckgo-search |

---

## Architecture

```
┌────────────────────────────────────────────────┐
│  Go TUI (bubbletea)                            │
│  Keyboard-first chat, streaming output, diffs  │
└──────────────┬─────────────────────────────────┘
               │ HTTP NDJSON :8765
┌──────────────▼─────────────────────────────────┐
│  Python backend (aiohttp)                      │
│  Session management, tool orchestration,       │
│  risk classification, memory                   │
└──────────────┬─────────────────────────────────┘
               │ SDK calls
┌──────────────▼─────────────────────────────────┐
│  LLM (Ollama / OpenRouter / any OpenAI API)    │
│  Vision, tool calling, structured output       │
└────────────────────────────────────────────────┘
```

Frontend and backend are fully decoupled — the TUI has no idea what LLM is running, the backend doesn't know about the UI. Either can be swapped independently.

---

## Computer use

Warden sees what's on your screen through screenshots, reasons about the pixels, and drives the mouse and keyboard like a human would.

- **Vision** — every screenshot is fed to the model as an image. Use any vision-capable model (Qwen2.5VL, LLaVA, GPT-4o, Claude, etc.)
- **Smart coordinates** — the model points at pixels in the image it was shown; Warden maps them to real screen coordinates automatically
- **Mouse** — move, click, right-click, double-click, scroll, drag
- **Keyboard** — type text (non-ASCII and emoji go through clipboard for reliability), press combos (`Ctrl+C`, `Alt+F4`, `Win+D`)
- **Fail-safe** — slam the mouse into the top-left corner to abort any automated action

The loop is simple: screenshot → model decides → mouse/keyboard → screenshot again to confirm.

---

## Safety

Warden classifies every action before execution — by code, not by the model.

| Risk | What it covers | Behaviour |
|---|---|---|
| 🟢 Safe | Read-only ops: file reads, git status, screenshots, process listing, browser reads, search | Runs without confirmation |
| 🟡 Confirm | File writes, installs, mouse/keyboard, process kills, external URLs | Pauses in Ask mode; runs immediately in Auto mode |
| 🔴 Blocked | Recursive deletes, encoded commands, registry changes, writes outside workspace | Always rejected |

Path traversal, UNC paths, and device paths are blocked at the resolver level.

---

## Memory

Persistent SQLite-backed memory that survives between sessions. Warden automatically stores facts about your workflow, preferences, and projects.

```
/memory on          # enable
/memory off         # disable
/memory status      # inspect stored facts
/memory clear       # wipe everything
```

Memory is injected into the system prompt each turn. At session end, short-term entries are aggregated into long-term snapshots.

---

## Skills

Extend Warden with reusable Markdown instruction sets. Drop a `SKILL.md` into `.warden/skills/<name>/` and invoke it with `! <name>`.

```
! <skill-name>      # run a skill
! <cmd>             # run any PowerShell command directly
!                   # list all available skills
```

Skills live in `.warden/skills/` (project) or `~/.warden/skills/` (global), with fallback to `.claude/skills/`.

---

## Configuration

Pre-configure via `~/.warden-config.json`. Skip the setup wizard entirely:

```json
{
  "provider": "openrouter",
  "api_url": "https://openrouter.ai/api/v1",
  "api_key": "sk-or-v1-...",
  "model": "poolside/laguna-m.1:free"
}
```

Or set `WARDEN_MODEL` as an environment variable and skip `/connect`.

---

## Controls

| Key | Action |
|---|---|
| `Enter` | Send message |
| `Esc` | Interrupt agent |
| `Esc` × 2 | Force-stop |
| `Shift+Tab` | Toggle Ask / Auto |
| `Tab` | Complete slash command or skill |
| `↑` / `↓` | Navigate input history (disabled in `/select` mode) |
| `Scroll` | Scroll chat output |
| `Ctrl+W` | Delete last word |
| `Ctrl+C` | Exit (force-quits during streaming) |

### `/select` mode

`/select` disables mouse capture so you can select text with the terminal. While active:
- `↑`/`↓` navigate input history is disabled (prevents wheel-to-history in some terminals)
- Left-click message expand is disabled
- `Esc` exits back to normal mode

## Slash commands

| Command | Action |
|---|---|
| `/connect` | Set up provider and model |
| `/clear` | Clear chat, reset session |
| `/compact` | Summarize context to free tokens |
| `/memory` | Manage memory |
| `/models` | Switch model (interactive picker) |
| `/select` | Enable text selection (disables mouse capture) |
| `/update` | Download and install latest release |
| `/verbose` | Toggle verbose mode (show tool calls and diffs) |

---

## Build from source

```powershell
just build              # Go TUI only
just build-backend      # Python backend (requires PyInstaller)
just release            # Both
```

## Tests

```powershell
just install             # Python deps
just test                # pytest with coverage
just test --no-cov -q    # Quick run
just test-go             # Go tests
just lint-go             # Go vet
just fmt-go              # Go format diff
just fmt-go-check        # Go format check (CI)
```

---

## Troubleshooting

| Symptom | Fix |
|---|---|
| `port 8765 is busy` | Another instance is running. `taskkill /F /IM warden.exe` |
| `ollama is not responding` | Install [Ollama](https://ollama.com) and start it |
| `pip install failed` | `pip install -r requirements.txt` |
| Backend exits immediately | Check `.warden/backend.err.log` |

---

## License

MIT