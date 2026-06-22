# Skills

Reusable instruction sets in Markdown.

## Location
- `.warden/skills/<name>/SKILL.md` — project
- `~/.warden/skills/<name>/SKILL.md` — global
- `.claude/skills/` — fallback

## Format
YAML frontmatter:
```yaml
---
name: kebab-case-name   # 1–64 chars
description: ...
---
```

Body ≤ 50 KB. Imperative voice. No emojis.

## Invocation
- User types `!<name>` in input — skill body sent as user message
- LLM may call `skill` tool to load a skill

## Discovery
`agent/skills.py`: project > global; `.warden` > `.claude`

`<available_skills>` XML injected into system prompt each turn.
