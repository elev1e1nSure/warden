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
- **Commits:** commit after every logically complete change — one message per user turn, split into multiple commits only when the diff clearly contains independent concerns. Leave the worktree clean. Exception: if a turn leaves a feature objectively half-done, say so and ask whether to commit anyway or finish first.

## stack

- **Go 1.25+** — TUI (bubbletea, lipgloss, glamour)
- **Python 3.11+** — backend (aiohttp, ollama SDK)
- pyautogui — mouse / keyboard
- Pillow — screenshots
- playwright — browser automation
- duckduckgo-search — web search

## project structure

```
go/                      — bubbletea TUI
  cmd/warden/            — entry point & launcher
  blocks.go              — connect wizard & block rendering
  chain.go               — action chain (animations)
  client_adapter.go      — backend message adapter
  commands.go            — slash command handlers & tick
  diff.go                — diff viewer
  input.go               — input model & history
  keys.go                — top-level key routing
  keys_action.go         — Esc / Ctrl+C handlers
  keys_nav.go            — arrow navigation & tab
  markdown.go            — glamour markdown renderer
  messages.go            — message entry types
  model.go               — main tea.Model
  render.go              — pulse, shimmer, think duration
  slash.go               — slash command autocomplete
  status.go              — status bar & wave
  styles.go              — lipgloss colors & helpers
  tools.go               — tool line rendering
  update.go              — self-updater
  update_modal.go        — modal state handlers
  update_stream.go       — streaming token handlers
  update_system.go       — backend lifecycle handlers
  view.go                — layout & message rendering
  viewport*.go           — scrollable viewport

justfile                 — task runner

agent/                   — Python backend
  server.py              — aiohttp server, routes
  chat.py                — chat session & streaming
  llm_client.py          — Ollama / OpenAI client
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
  - blue `#38BDF8` — secondary: Auto mode highlights (wave, input border)
  - neutral `#d0d0d0` — tool names in action log
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
- **tool lines:** `→ name  args` → `+ name  result  +N -N`; name neutral `#d0d0d0`, result dim, stats green/red, errors red; click line to expand diff inline
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
