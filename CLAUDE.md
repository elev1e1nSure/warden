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
- **accent colors:**
  - green `#00D47A` — primary accent: mode in status bar, Warden label, active input border, slash command names in hints, wave spinner peak
  - yellow `#FFD700` — secondary accent: "you" label in user messages, tool names in tool lines
  - red `#ff4444` — errors only
  - dim `#666666` — metadata: timestamps, result text, descriptions
  - faint `#444444` — separators, inactive wave chars
- **layout:** no top header; status bar at bottom (2 lines); rounded border on input
- **status bar line 1:** `ask · model · provider` — mode in green, rest dim
- **status bar line 2:** wave spinner (green `█▓▒░` bouncing) + hint (`esc interrupt` or confirm prompt)
- **wave spinner:** 7 positions, peak bounces left-right using `█▓▒░░░░`; idle = `·······` faint
- **input:** `RoundedBorder`, green-faint when idle, faint when streaming; prompt `> `
- **user messages:** `[HH:MM]  you  text` — "you" in yellow bold
- **assistant messages:** `[HH:MM]  text` — no "Warden:" label; timestamp dim, content rendered as markdown
- **think line:** `[HH:MM]  + Thought: Xs` dim; F2 expands (no hint shown in UI)
- **tool lines:** `▶ name  args` → `  ✓ name → result`; name in yellow, result dim, errors red
- **slash hints:** 2 columns — command name (green, 14-char left-aligned) + description (dim)
- controls: arrows, Enter, Esc, Ctrl+C
- no buttons, no mouse clicks in the TUI itself
- if there's a need to add something truly new — discuss with the user first, then add to this section
