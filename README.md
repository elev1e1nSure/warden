# Warden

[![CI](https://github.com/elev1e1nSure/warden/actions/workflows/ci.yml/badge.svg)](https://github.com/elev1e1nSure/warden/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/elev1e1nSure/warden?label=latest)](https://github.com/elev1e1nSure/warden/releases/latest)
[![Windows](https://img.shields.io/badge/OS-Windows-blue)](https://github.com/elev1e1nSure/warden/releases/latest)

> **Describe the task. Skip the script.**

Warden is an AI agent that lives in your terminal and controls your Windows machine. You tell it what to do in plain English — it figures out the steps, runs them, and shows you what's happening in real time.

It sees your screen, runs shell commands, edits files, drives the browser. No IDE plugin. No Electron wrapper. No cloud account required.

![demo](assets/warden-gif.gif)

---

## Get it

[**⬇ Download for Windows x64**](https://github.com/elev1e1nSure/warden/releases/latest)

Unzip anywhere. Two binaries, one folder, zero setup. Run `warden.exe` and pick your LLM provider on first launch.

```powershell
# Or build from source (Go 1.25+, Python 3.11+)
just release
```

---

## What it's for

You have a task that's annoying enough to think about but not worth writing a script for. Warden handles those.

```
> find the five largest files on my desktop and zip them into archive.zip
> go through my Downloads, delete anything older than 30 days, keep PDFs
> open Chrome, go to GitHub, close any PR that's been sitting for 90 days
> check why the last CI run failed and show me the relevant logs
> rename all the screenshots in this folder to match the date they were taken
```

It works across your whole machine — shell, browser, file system, running processes — and streams every action back to you as it goes.

---

## How it works

```
You type a task
      │
      ▼
  Screenshot → model reasons over what it sees
      │
      ▼
  Runs a command / clicks / types / scrolls
      │
      ▼
  Screenshot again → confirm it worked
      │
      ▼
  Next step (or done)
```

Every action is visible in the chat stream. **Ask mode** pauses before writes and mouse clicks so you stay in control. **Auto mode** lets the agent run freely when you trust the task.

Toggle between them with `Shift+Tab` at any time.

---

## Safety

Actions are classified by code — not by the model — before anything runs.

| Level | What | Behaviour |
|---|---|---|
| Safe | Read-only: file reads, screenshots, `git status`, browser reads | Runs immediately |
| Confirm | Writes, installs, mouse/keyboard, process kills | Pauses in Ask mode |
| Blocked | Recursive deletes outside workspace, encoded payloads, registry changes | Always rejected |

Fail-safe: slam the mouse into the top-left corner to abort any automated action mid-run.

---

## Memory

Warden remembers facts about your projects and workflow between sessions — stored locally in SQLite, injected into context each turn.

```
/memory on          # enable
/memory off         # disable
/memory status      # inspect what's stored
/memory clear       # wipe everything
```

---

## Skills

Reusable instruction sets in plain Markdown. Drop a `SKILL.md` into `.warden/skills/<name>/` and invoke it with `!`:

```
!<skill-name>       # run a skill
!<cmd>              # run a PowerShell command directly
!                   # list all available skills
```

Skills live in `.warden/skills/` (project) or `~/.warden/skills/` (global).

---

## Configuration

Pre-configure with `~/.warden-config.json` to skip the setup wizard:

```json
{
  "model": "poolside/laguna-m.1:free",
  "api_url": "https://openrouter.ai/api/v1",
  "api_key": "sk-or-v1-..."
}
```

The API key is encrypted on disk (DPAPI on Windows). Key and auth token are passed to the backend via stdin — never in environment variables. Every backend request is authenticated with a shared secret generated at startup.

Use `WARDEN_MODEL` to override the model via environment variable. Works with Ollama (local) or any OpenAI-compatible API.

---

## Controls

| Key | Action |
|---|---|
| `Enter` | Send message |
| `Esc` | Interrupt agent |
| `Esc × 2` | Force-stop |
| `Shift+Tab` | Toggle Ask / Auto mode |
| `Tab` | Complete slash command or skill |
| `↑` / `↓` | Navigate input history |
| `Scroll` | Scroll chat output |
| `Ctrl+W` | Delete last word |
| `Ctrl+C` | Exit |

## Slash commands

| Command | Action |
|---|---|
| `/connect` | Set up provider and model |
| `/models` | Switch model on the fly |
| `/clear` | Clear chat, reset session |
| `/compact` | Summarize context to free tokens |
| `/memory` | Manage memory |
| `/select` | Enable text selection mode |
| `/update` | Download and install latest release |
| `/verbose` | Show tool calls and diffs |

---

## Stack

| Layer | Technology |
|---|---|
| TUI | Go 1.24+, bubbletea, lipgloss, glamour |
| Backend | Python 3.11+, aiohttp |
| LLM | Ollama or any OpenAI-compatible API |
| Screen & input | pyautogui, Pillow |
| Browser | Playwright |
| Search | duckduckgo-search |

Frontend and backend are fully decoupled over HTTP NDJSON. Either can be swapped independently.

---

## Build & test

```powershell
just build              # Go TUI only
just build-backend      # Python backend (requires PyInstaller)
just release            # Both

just install            # Python deps
just test               # pytest with coverage
just test --no-cov -q   # Quick run
just test-go            # Go tests
just lint-go            # Go vet
just fmt-go             # Go format diff
```

---

## Troubleshooting

| Symptom | Fix |
|---|---|
| `port 8765 is busy` | Another instance running — `taskkill /F /IM warden.exe` |
| `ollama is not responding` | Install [Ollama](https://ollama.com) and start it |
| Backend exits immediately | Check `.warden/backend.err.log` |

---

## License

MIT
