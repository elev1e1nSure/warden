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
| `Esc` | interrupt |
| `Esc` ×2 | force-interrupt |
| `Shift+Tab` | toggle Ask / Auto mode |
| `↑` / `↓` | scroll |
| `Ctrl+C` | exit |

## slash commands

| command | action |
|---|---|
| `/auto` | Auto mode — dangerous commands run without confirmation |
| `/ask` | Ask mode — confirmation for dangerous commands |
| `/reset` | reset session |
| `/provider <name>` | switch provider (`ollama` or `openrouter`) |
| `/api <url>` | set API base URL (e.g. for OpenRouter) |

## safety

Risk classification is done by code, not the model:

- **safe** — read-only: `Get-ChildItem`, `git status`, `screenshot`, `file_read` inside workspace
- **confirm** — file writes, installs, mouse/keyboard, process kills, unknown binaries
- **blocked** — forced recursive delete, encoded commands, remote eval, disk format, registry changes, writes outside workspace

In **Ask** mode, confirm-level actions require `y` / `n` before executing. In **Auto** mode they run immediately. Blocked actions are always rejected.

## tools

`powershell` `file_read` `file_write` `file_delete` `file_list` `glob` `grep` `edit` `apply_patch` `clipboard` `screenshot` `mouse` `keyboard` `browser_open` `browser_read` `browser_screenshot` `google_search` `youtube_search` `webfetch` `skill` `todowrite` `question`
