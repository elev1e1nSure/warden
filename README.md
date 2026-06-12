# warden

[![CI](https://github.com/elev1e1nSure/warden/actions/workflows/ci.yml/badge.svg)](https://github.com/elev1e1nSure/warden/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

[Русский](README.ru.md)

CLI computer control agent for Windows. Go TUI + Python backend + Ollama.

![demo](assets/warden-gif.gif)

## install

```bash
pip install -r requirements.txt
```

Windows:

```bash
build.bat
```

Or manually:

```bash
cd go && go build -o ../warden.exe ./cmd/warden
```

## run

```bash
./warden.exe
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

With no config file it defaults to Ollama (`qwen3:8b`).

## computer use

The agent can see and drive the screen. The loop is: `screenshot` → look at
the image → `mouse` / `keyboard` → `screenshot` again to confirm.

- **Vision** — every `screenshot` is attached to the conversation as an image,
  so a vision-capable model literally sees the screen. Pick a model with image
  support (e.g. a `qwen2.5vl` / `llava` Ollama model, or any vision model on
  OpenRouter); text-only models are blind to screenshots.
- **Coordinates** — screenshots are downscaled before being sent to the model.
  The agent points at pixels **in the image it was shown**, and warden maps
  those back to real screen pixels automatically. No manual scaling.
- **`mouse`** — `move`, `click`, `right_click`, `double_click`, `scroll`, and
  `drag` (give `x`/`y` as the start and `x2`/`y2` as the drop point).
- **`keyboard`** — `type` writes text (non-ASCII like Cyrillic/emoji is pasted
  via the clipboard so it lands correctly), `press` sends keys and combos
  (`ctrl+c`, `alt+f4`, `win+d`).

A real screen corner is the pyautogui fail-safe: slam the cursor into the
top-left corner to abort an automated action.

## controls

| key | action |
|---|---|
| `Enter` | send |
| `Esc` | interrupt stream |
| `Esc` ×2 | force-stop |
| `Esc` (in `/select` mode) | exit text selection |
| `Shift+Tab` | toggle Ask / Auto mode |
| `Tab` | complete slash command |
| `↑` / `↓` | scroll during stream |
| `scroll wheel` | scroll (5 lines per tick) |
| `↑` / `↓` (idle) | navigate input history |
| `Ctrl+W` | delete last word in input |
| `Ctrl+C` | exit |

## slash commands

| command | action |
|---|---|
| `/connect` | set up provider and model |
| `/clear` | clear chat and reset session |
| `/compact` | summarize context to free up token budget |
| `/models` | switch model (interactive picker) |
| `/select` | enable text selection (disables mouse capture) |
| `/update` | download and install the latest release |
| `/verbose` | toggle verbose mode (show tool lines and diffs) |

## skills

Skills are Markdown instruction sets the agent can invoke.

```
! <skill-name>          # invoke a skill
!                       # list available skills
```

Skills live in `.warden/skills/<name>/SKILL.md` (project) or `~/.warden/skills/<name>/SKILL.md` (global).

## safety

Risk classification is done by code, not the model:

- **safe** — read-only: `Get-ChildItem`, `git status`, `screenshot`, `file_read` inside workspace, `browser_read`
- **confirm** — file writes, installs, mouse/keyboard, process kills, unknown binaries
- **blocked** — forced recursive delete, encoded commands, remote eval, disk format, registry changes, writes outside workspace

In **Ask** mode, confirm-level actions require `y` / `n` before executing. In **Auto** mode they run immediately. Blocked actions are always rejected.

## tools

`powershell` `bash` `file_read` `file_write` `file_delete` `file_list` `file_move` `file_copy` `glob` `grep` `edit` `apply_patch` `clipboard` `screenshot` `mouse` `keyboard` `browser_open` `browser_read` `browser_screenshot` `google_search` `youtube_search` `webfetch` `archive` `process_list` `process_kill` `skill` `todowrite` `question`
