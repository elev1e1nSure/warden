# warden

Minimal CLI computer-control agent. Go TUI and agent loop. Screenshot → model → action loop via vision models (Ollama/OpenRouter). Windows only.

## Stack
- Go 1.25+ — bubbletea, lipgloss, glamour
- Config: `~/.warden-config.json` (API key encrypted via `internal/security/`)

## Commands
- Build: `just build` (Go)
- Test:  `just test` (Go)
- Lint:  `just lint` (go vet)
- Run:   `warden.exe` (run build first)

## Architecture
- `cmd/warden/` — entry main package and `AgentBackend` implementing `tui.Backend`
- `agent/` — native Go agent. `session.go` (session loop), `runner.go` (tool executor)
- `agent/tools/` — tool implementations in Go: shell, files, browser, screen, search, patch, etc.
- `agent/safety/policy.go` — ground truth for risk classification in Go. Model/prompt never decides safety.
- `agent/memory/` — Go SQLite memory store + aggregation

## Conventions
- **Language:** English everywhere — UI, logs, comments. No Russian in code.
- **Commits:** Conventional Commits with scope, after each complete change. One per turn.
- Tabs; Go camelCase.

## Anti-patterns
- Do NOT move safety logic into prompts — only `agent/safety/policy.go` decides.
- Do NOT add a tool to the auto-promotion list in `policy.go` without workspace checks.
- Do NOT restructure TUI without discussion — visual consistency has no test coverage.

## References
- TUI visual spec — `docs/tui-spec.md` (читать при работе с TUI)
- Safety levels — `agent/safety/policy.go` (safe/confirm/blocked, deterministic, не в промпте)
- Skills — `.warden/skills/README.md` (читать при работе со скиллами)
