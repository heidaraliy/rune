---
name: rune-tui-engineer
description: Bubble Tea TUI workflow for Rune. Use for internal/app state, keyboard flows, layout rendering, footer/status text, editor integration, and clipboard behavior.
---

# Rune TUI Engineer

Use this skill for `internal/app/**` changes.

## Rules

- Keep Bubble Tea state transitions explicit and testable.
- Update footer/help text with keybinding behavior.
- Keep status messages transient, specific, and revision-safe.
- Avoid terminal-width overflow in rendered lines.
- Keep editor and clipboard dependencies injectable for tests.
- Use temp stores in tests and manual smokes.

## Validation

- Run `go test ./internal/app` for focused TUI changes.
- Run `go test ./...` before publishing.
- For visual changes, inspect `go run ./cmd/rune` with a temp `RUNE_HOME` when practical.
