# warden — claude code instructions

## project

CLI computer control agent. Go TUI + Python backend + Ollama. Strict minimalism.

## rules

- language: English, informal, no emojis
- UI/UX text and logs must be exclusively in English — no exceptions
- don't rewrite entire files — only what's needed
- ask before large changes
- don't add dependencies without reason
- code comments in English, short
- risk classification is deterministic code in `agent/safety.py` — never the model or the prompt
- risk markers in `.warden/powershell-reference.md` must match what `agent/safety.py` actually enforces; change them together

## stack

- **Go 1.23+** — frontend (bubbletea, lipgloss)
- **Python 3.11+** — backend (aiohttp, ollama SDK)
- pyautogui — mouse / keyboard
- Pillow — screenshots
- playwright — browser
- duckduckgo-search — search

## code style

- tabs, not spaces
- Go: camelCase, idiomatic Go; Python: snake_case
- typing is mandatory (`typing`, `dataclasses`)
- no unnecessary abstractions — simple and clear
- async where streaming is needed
- tests: `pytest agent/` — run after any change to `agent/safety.py`

## structure

```
go/        — bubbletea frontend
agent/     — backend: server, chat, tools, safety, logs
.warden/   — runtime reference docs (powershell-reference.md)
```

## visual

- no colored text backgrounds. only text + color + bold
- colors: cyan for warden, yellow for tools, red for errors, dim for meta info
- controls: arrows, Enter, Esc, Ctrl+C
- no buttons, no mouse clicks in the TUI itself
- confirmation block: title, "will run:" preview, "why:" bullets, key hints; `y` confirms, Enter / Esc / `n` cancel (cancel is the default)
- if there's a need to add something truly new — discuss with the user first, then add to this section
