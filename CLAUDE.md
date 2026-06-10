# warden — claude code instructions

## project

CLI computer control agent. Go TUI + Python backend + Ollama. Strict minimalism.

## rules

- language: English, informal, no emojis
- don't rewrite entire files — only what's needed
- ask before large changes
- don't add dependencies without reason
- code comments in English, short

## stack

- **Go 1.21+** — frontend (bubbletea, lipgloss)
- **Python 3.11+** — backend (aiohttp, ollama SDK)
- pyautogui — mouse / keyboard
- Pillow — screenshots
- playwright — browser
- duckduckgo-search — search

## code style

- tabs, not spaces
- snake_case everywhere
- typing is mandatory (`typing`, `dataclasses`)
- no unnecessary abstractions — simple and clear
- async where streaming is needed

## structure

```
go/        — bubbletea frontend
agent/     — backend: server, chat, tools, logs
```

## visual

- no colored text backgrounds. only text + color + bold
- colors: cyan for warden, yellow for tools, red for errors, dim for meta info
- controls: arrows, Enter, Esc, Ctrl+C
- no buttons, no mouse clicks in the TUI itself
- if there's a need to add something truly new — discuss with the user first, then add to this section
