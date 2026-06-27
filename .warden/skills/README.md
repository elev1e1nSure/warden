# Skills

Reusable instruction sets in Markdown.

## Location
Skills are stored in subdirectories containing a `SKILL.md` file. They are searched in the following locations (ordered by priority, later paths override earlier ones with the same skill name):

1. `~/.codex/skills/<name>/SKILL.md` (fallback)
2. `~/.agents/skills/<name>/SKILL.md` (fallback)
3. `~/.claude/skills/<name>/SKILL.md` (global fallback)
4. `~/.warden/skills/<name>/SKILL.md` (global user skills)
5. `./skills/<name>/SKILL.md` (local workspace skills)
6. `./.claude/skills/<name>/SKILL.md` (local project fallback)
7. `./.warden/skills/<name>/SKILL.md` (local project skills)

## Format
YAML frontmatter:
```yaml
---
name: kebab-case-name   # 1–64 chars
description: ...        # One-line summary
---
```

Body ≤ 64 KB. Imperative voice. No emojis.

## Invocation
- User types `!<name> [args]` in the input box. The Go TUI loads the skill contents and sends them as a user message to begin the session.
- Note: The LLM `skill` tool is defined as a stub in the Go backend (`agent/tools/misc.go`) and is currently not implemented. Users should rely on the `!<name>` input prefix.

## Discovery
The discovery logic is implemented in Go under [skills.go](file:///d:/Projects/warden/agent/skills/skills.go).

The `<available_skills>` XML tag containing active skill names and descriptions is injected into the system prompt at the start of each LLM turn.
