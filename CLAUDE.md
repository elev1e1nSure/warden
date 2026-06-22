# warden

Minimal CLI computer-control agent. Go TUI + Python backend. Screenshot → model → action loop via vision models (Ollama/OpenRouter). Windows only.

## Stack
- Go 1.25+ — bubbletea, lipgloss, glamour
- Python 3.11+ — aiohttp
- Config: `~/.warden-config.json` (API key encrypted via `go/internal/security/`)

## Commands
- Build: `just build` (Go), `just release` (Go + PyInstaller backend)
- Test:  `just test` (pytest), `just test-go`
- Lint:  `just lint-py` (ruff), `just lint-go` (go vet)
- Run:   `just run` (backend), `warden.exe` (full app)

## Architecture
- `go/` — TUI. `main.go` = entry; `model.go` = tea.Model; `cmd/warden/` = launcher
- `go/internal/client/` — HTTP NDJSON backend client
- `agent/` — backend. `server.py` (aiohttp routes), `chat.py` (session + streaming)
- `agent/tools/` — tool implementations: shell, files, browser, screen, search, patch, etc.
- `agent/safety/_policy.py` — ground truth for risk classification. Model/prompt never decides safety.
- `agent/memory/` — SQLite memory store + aggregation

## Conventions
- **Language:** English everywhere — UI, logs, comments. No Russian in code.
- **Commits:** Conventional Commits with scope, after each complete change. One per turn.
- Tabs; Go camelCase, Python snake_case.
- Use `Path.resolve()` for paths, not `os.path.abspath` — abspath misbehaves on symlinks.

## Anti-patterns
- Do NOT move safety logic into prompts — only `agent/safety/_policy.py` decides.
- Do NOT add a tool to the auto-promotion list in `_policy.py` without workspace checks — patch tools already caused bugs.
- Do NOT restructure TUI without discussion — visual consistency has no test coverage.
- Do NOT put API keys in child process env — pass via stdin.

## References
- TUI visual spec — `@docs/tui-spec.md` (читать при работе с TUI)
- Safety levels — `@agent/safety/_policy.py` (safe/confirm/blocked, deterministic, не в промпте)
- Skills — `@.warden/skills/README.md` (читать при работе со скиллами)
