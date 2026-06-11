# warden

[Русский](README.ru.md)

CLI computer control agent for Windows. Go TUI + Python backend + Ollama.

![demo](assets/warden-gif.gif)

## install

```bash
pip install -r requirements.txt
cd go && go build -o warden.exe ./cmd/warden
```

## run

```bash
./warden.exe
```

Starts Ollama and downloads the model automatically on first launch.

### OpenRouter

```bash
$env:OPENROUTER_API_KEY="sk-or-v1-..."
.\warden.exe --provider openrouter --model poolside/laguna-m.1:free
```

| flag | default | description |
|---|---|---|
| `--provider` | `ollama` | `ollama` or `openrouter` |
| `--model` | `qwen3:8b` | model name |
| `--api` | — | override API base URL |

## controls

| key | action |
|---|---|
| `Enter` | send |
| `Esc` | interrupt stream |
| `Esc` ×2 | force-stop |
| `Shift+Tab` | toggle Ask / Auto mode |
| `↑` / `↓` | scroll during stream |
| `scroll wheel` | scroll (5 lines per tick) |
| `↑` / `↓` (idle) | navigate input history |
| `Ctrl+C` | exit |

## slash commands

| command | action |
|---|---|
| `/auto` | Auto mode — confirm-level actions run without prompt |
| `/ask` | Ask mode — confirm-level actions require `y` / `n` |
| `/reset` | reset session and clear screen |
| `/clear` | clear screen, keep session |
| `/status` | show model, provider, mode |
| `/models` | switch model (interactive picker) |
| `/provider <name>` | switch provider (`ollama` \| `openrouter`) |
| `/api <url>` | override API base URL |
| `/compact` | summarize context to free up token budget |
| `/copy-last` | copy last assistant response to clipboard |
| `/verbose` | toggle verbose mode (show tool lines and diffs) |
| `/pwd` | show current working directory |

## skills

Skills are Markdown instruction sets the agent can invoke.

```
! <skill-name>          # invoke a skill
!                       # list available skills
```

Skills live in `.warden/skills/<name>/SKILL.md` (project) or `~/.warden/skills/<name>/SKILL.md` (global).

## safety

Risk classification is done by code, not the model:

- **safe** — read-only: `Get-ChildItem`, `git status`, `screenshot`, `file_read` inside workspace
- **confirm** — file writes, installs, mouse/keyboard, process kills, unknown binaries
- **blocked** — forced recursive delete, encoded commands, remote eval, disk format, registry changes, writes outside workspace

In **Ask** mode, confirm-level actions require `y` / `n` before executing. In **Auto** mode they run immediately. Blocked actions are always rejected.

## tools

`powershell` `file_read` `file_write` `file_delete` `file_list` `glob` `grep` `edit` `apply_patch` `clipboard` `screenshot` `mouse` `keyboard` `browser_open` `browser_read` `browser_screenshot` `google_search` `youtube_search` `webfetch` `skill` `todowrite` `question`
