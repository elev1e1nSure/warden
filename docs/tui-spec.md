# TUI visual spec

## Colors
- green `#8AB89A` — primary accent: mode label, active input border, slash names, wave peak
- blue `#38BDF8` — secondary: Auto mode highlights (wave, input border)
- `#d0d0d0` — neutral: tool names in action log
- `#ff4444` — red: errors only
- `#666666` — dim: timestamps, descriptions, metadata
- `#2a2a2a` — faint: separators, inactive wave dots

## Layout
- No top header
- Chat viewport fills screen
- Bottom bar: full-width wave + rounded input + single-line status bar

## Elements

### Status bar
`Ask · model · hint [tokens]` — mode colored, model white, hint dim, tokens right-aligned

### Wave
Full-width bouncing `·` dots under input. Green in Ask, blue in Auto, faint when idle.

### Input
`RoundedBorder`, green idle / blue Auto / faint streaming. Prompt: `> `

### Messages
- User: `#242424` block, no `>` prompt in history
- Assistant: `[HH:MM]  text` — no "Warden:" label, timestamp dim, markdown rendered
- Think: `[HH:MM]  + Thought: Xs` dim

### Tool lines
`→ name  args` → `+ name  result  +N -N`
- Name: neutral `#d0d0d0`
- Result: dim
- Stats: green/red
- Errors: red
- Click line to expand diff inline

### Slash hints
2-column autocomplete: name (green, 14-char) + description (dim)

## Controls
- Enter — send message
- Esc — interrupt; 2× force-stop
- Shift+Tab — toggle Ask / Auto mode
- Tab — complete slash command or skill
- ↑ / ↓ — navigate input history
- Ctrl+W — delete last word
- Ctrl+C — exit

## Prefix hints
- `/` — slash commands
- `!` — skills
