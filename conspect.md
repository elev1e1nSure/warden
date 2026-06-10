# Conspect

## Stack
- Frontend: Go 1.23+, Bubbletea, Lipgloss
- Backend: Python 3.11+, aiohttp
- LLM: Ollama (qwen3:8b)
- Computer: Pyautogui, Pillow
- Browser: Playwright
- Search: DuckDuckGo

## Architecture
Go TUI <-> Python backend (HTTP NDJSON) <-> Ollama <-> Tools (PowerShell, filesystem, mouse, etc.)

## Structure
- `go/` — bubbletea TUI frontend
- `agent/` — Python backend (server, chat, tools, safety)
- `.warden/` — runtime reference docs (powershell-reference.md)
- `requirements.txt` — Python dependencies
- `CLAUDE.md` — project style and agent instructions

## Tools
| Type | Examples |
|---|---|
| CLI | powershell, bash, file_read, file_write, file_delete, file_list |
| Clipboard | clipboard (read/write) |
| Screenshot | screenshot, browser_screenshot |
| Browser | browser_open, browser_read |
| Interaction | mouse, keyboard |
| Search | google_search, youtube_search |

## Modes
- **Ask**: Requires confirmation for risky actions
- **Auto**: Auto-approves confirmed commands

## Confirmation
- Shows command, risk reason, and confirmation keys (y/n)
- Times out after 5 minutes

## Commands
- `/auto` - Disable confirmations
- `/ask` - Enable confirmations
- `/reset` - Clear pending actions
- `/thinking` - Toggle model reasoning

## Security
- **Safe**: Read-only operations
- **Confirm**: Writes, installs, mouse clicks
- **Blocked**: High-risk actions (no execution)

