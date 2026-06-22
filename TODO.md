# TODO

## Animation stutter — IN PROGRESS

- \ Throttle `syncViewport()` during token streaming — per-token glamour re-render freezes spinner/wave/pulse.
- `update_stream.go`: removed `syncViewport()` from `tokenMsg`, viewport syncs only on tick (70ms) + stream start/end.
- **Next:** incremental markdown cache — avoid full re-render for unchanged messages.

## History / select mode — DONE

- \ Fix mouse wheel navigating prompt history during `/select` mode.
- `keys_nav.go`: added `!m.selectMode` guard to `handleKeyUp`/`handleKeyDown`.

## Vision-less models — DONE

- \ Remove canned "Sorry, this model doesn't support images" message.
- \ Add `[note: ...]` annotation when images are stripped so model knows it's blind.

## Launcher — DONE

- \ Simplify loading screen: remove lipgloss styling, padding, centering — just spinner + "Warden loading..." + elapsed ms.
