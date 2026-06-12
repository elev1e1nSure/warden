# warden

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

`powershell` `file_read` `file_write` `file_delete` `file_list` `glob` `grep` `edit` `apply_patch` `clipboard` `screenshot` `mouse` `keyboard` `browser_open` `browser_read` `browser_screenshot` `google_search` `youtube_search` `webfetch` `skill` `todowrite` `question`
