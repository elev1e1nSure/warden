# warden — agent knowledge base

Minimal CLI computer-control agent. Go TUI frontend + Python backend. Drives the screen via vision models (Ollama or OpenRouter).

## agent rules

- **Language:** English only, informal, no emojis — UI text, logs, comments, everything
- **Edits:** change only what's needed; never rewrite an entire file unless asked
- **Dependencies:** don't add without a reason; prefer stdlib
- **Comments:** English, short, only when logic is non-obvious
- **Large changes:** ask the user before doing them
- **Safety:** risk classification lives in `agent/safety/_policy.py` — deterministic code, never the model or prompt
- **New TUI patterns:** discuss with the user first, then document here

## stack

- **Go 1.24+** — TUI (bubbletea, lipgloss)
- **Python 3.11+** — backend (aiohttp, ollama SDK)
- pyautogui — mouse / keyboard
- Pillow — screenshots
- playwright — browser automation
- duckduckgo-search — web search

## project structure

```
go/                      — bubbletea TUI
  cmd/warden/            — entry point
  blocks.go              — message rendering
  chain.go               — update chaining
  client.go              — HTTP client to backend
  commands.go            — slash command handlers
  diff.go                — diff viewer
  input.go               — input model & history
  keys.go                — key bindings
  markdown.go            — markdown parser
  messages.go            — message types
  model.go               — main tea.Model
  render.go              — render helpers
  slash.go               — slash command autocomplete
  status.go              — status bar
  stream.go              — streaming state
  styles.go              — lipgloss styles
  tools.go               — tool line rendering
  update.go              — self-updater
  view.go                — layout glue
  viewport*.go           — scrollable viewport

agent/                   — Python backend
  server.py              — aiohttp server, routes
  chat.py                — chat session & message history
  llm_client.py          — OpenRouter / Ollama client
  prompt.py              — system prompt builder
  tool_runner.py         — tool dispatch & execution
  confirmations.py       — user confirmation flow
  skills.py              — skill discovery & loading
  logger.py              — structured logging
  ollama_process.py      — ollama lifecycle
  memory/                — memory aggregation & extraction
  safety/                — risk classification & policies
  tools/                 — tool implementations
  test_*.py              — pytest suite

.warden/skills/          — built-in skills
  skill-creator/SKILL.md — skill authoring skill
```

## code style

- Tabs for indentation, not spaces
- Go: camelCase, idiomatic; Python: snake_case
- Mandatory typing (`typing`, `dataclasses`)
- No unnecessary abstractions — keep it simple
- Async only where streaming or I/O concurrency is needed
- Run `just test` after any change to `agent/safety/`

## TUI visual spec

- **accent colors:**
  - green `#8AB89A` — primary: mode label, active input border, slash names, wave peak
  - blue `#38BDF8` — secondary: tool names, Auto mode highlights
  - red `#ff4444` — errors only
  - dim `#666666` — timestamps, descriptions, metadata
  - faint `#2a2a2a` — separators, inactive wave dots
- **layout:** no top header; chat viewport fills screen; bottom has full-width wave, rounded input, single-line status bar
- **status bar:** `Ask · model · hint [tokens]` — mode colored, model white, hint dim, tokens right-aligned
- **wave:** full-width bouncing `·` dots under input; green in Ask, blue in Auto, faint when idle
- **input:** `RoundedBorder`, green idle / blue Auto / faint streaming; prompt `> `
- **user messages:** `#242424` block, no `>` prompt in history
- **assistant messages:** `[HH:MM]  text` — no "Warden:" label; timestamp dim, markdown rendered
- **think line:** `[HH:MM]  + Thought: Xs` dim
- **tool lines:** `▶ name  args` → `  ✓ name → result`; name blue, result dim, errors red
- **slash hints:** 2 columns — name (green, 14-char) + description (dim)
- **controls:** arrows, Enter, Esc, Ctrl+C, Shift+Tab (toggle mode), Tab (complete), Ctrl+W (delete word)
- **prefix hints:** `/` → slash commands; `!` → skills.
- `/connect` — interactive provider/model picker; `/select` — text-selection mode (disables mouse capture)

## skills

Markdown instruction sets at `.warden/skills/<name>/SKILL.md` (project) or `~/.warden/skills/<name>/SKILL.md` (global). Fallback `.claude/skills/`.

- **Catalog** — `<available_skills>` XML injected into system prompt each turn
- **Invocation** — `!<name>` in input sends skill body as user message; LLM may also call the `skill` tool
- **Discovery** — `agent/skills.py`: project > global; `.warden` > `.claude`
- **Format** — YAML frontmatter with `name` (kebab-case, 1–64) and `description`; body ≤ 50 KB; imperative voice; no emojis

## safety levels

Deterministic classification in `agent/safety/_policy.py`:

- **safe** — read-only: `Get-ChildItem`, `git status`, `screenshot`, `file_read` inside workspace, `browser_read`
- **confirm** — writes, installs, mouse/keyboard, process kills, unknown binaries — requires `y/n` in Ask mode
- **blocked** — recursive delete outside workspace, encoded commands, remote eval, disk format, registry changes
