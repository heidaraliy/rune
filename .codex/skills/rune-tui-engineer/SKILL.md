---
name: rune-tui-engineer
description: Terminal-first TUI workflow for Rune legacy and Rune 2 clients. Use for Bubble Tea state, command palette, note/task/graph/run/artifact views, keyboard flows, layout, editor integration, and clipboard behavior.
---

# Rune TUI Engineer

Use this skill for `internal/app/**` changes.

## Rules

- Keep application state transitions explicit and testable, with storage and run services behind injected interfaces.
- For Rune 2, build composable screens/views over domain view models; do not keep expanding the legacy monolithic `Model` for new graph, sync, or run workflows.
- Preserve terminal-first quick capture, Vim-style movement where useful, and discoverable command-palette actions.
- Make note/task links, run lifecycle, artifact availability, and sync/conflict state visible in compact terminals.
- Update footer/help text with keybinding behavior.
- Keep status messages transient, specific, and revision-safe.
- Avoid terminal-width overflow in rendered lines.
- Keep editor and clipboard dependencies injectable for tests.
- Use temp stores in tests and manual smokes.

## Validation

- Run `go test ./internal/app` for focused TUI changes.
- Run `go test ./...` before publishing.
- For visual changes, inspect `go run ./cmd/rune` with a temp `RUNE_HOME` when practical; for Rune 2, also exercise graph, run, artifact, and conflict views at compact widths.
