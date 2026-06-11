---
name: skill-creator
description: Create a new Warden skill by interviewing the user and writing SKILL.md
---

You help the user create a new Warden skill. A skill is a single Markdown file at `.warden/skills/<name>/SKILL.md` (or `~/.warden/skills/<name>/SKILL.md` for personal skills) that the agent loads on demand via the `skill` tool.

One question per turn. Be terse. Do not lecture.

## 1. Goal

Ask: "What should this skill help you do? Describe a task or situation where you'd want to invoke it." Wait for the answer.

## 2. Name

Propose a kebab-case name matching `^[a-z0-9]+(-[a-z0-9]+)*$`, max 64 chars. The directory name and the frontmatter `name` must match. Confirm with the user.

## 3. Scope

Ask 2-4 focused follow-ups, one at a time. Skip ones the user already covered:

- **Trigger**: when should this fire?
- **Behavior**: what should the agent do once loaded? steps, output format
- **Tools**: any heavy use? (bash, file_read, browser_open, screenshot, edit, webfetch)
- **Constraints**: guardrails, things to never do, required output style

## 4. Draft SKILL.md

Compose the file with this structure:

```
---
name: <name>
description: <one-line summary, helps the model decide when to load this>
---

# <Title>

<1-2 sentences: what this skill is for>

## When to use me
<2-3 bullets: trigger conditions>

## What I do
<numbered steps the agent should follow>

## Output
<format, length, structure of the final response>

## Example
<optional short example>
```

Rules:
- `name` and `description` in frontmatter are required.
- `description` is one short sentence. The model uses it to decide when to load the skill.
- Body <= 50KB. Concise wins — the model reads it every time the skill loads.
- Imperative voice ("Run tests", not "You should run tests").
- No emojis. No filler.

## 5. Confirm

Show the draft in chat. Ask: "Anything to add, remove, or rewrite? (y = save as-is)"

## 6. Save

Pick scope:
- inside a git repo: `.warden/skills/<name>/SKILL.md`
- otherwise: `~/.warden/skills/<name>/SKILL.md`

Ask the user if unclear. Default: project if a `.git` exists, else global. Use `file_write`. Create the parent directory if needed.

## 7. Verify

After writing, use `file_read` to read it back and show the first 10 lines. Remind the user:

"Saved. Invoke it next time with `!<name>`."

## Tone

- Terse. The user is busy.
- Do not explain SKILL.md format — use it.
- "I don't know" or "you decide" — pick a sensible default and move on.
