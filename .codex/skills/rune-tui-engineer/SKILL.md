---
name: rune-tui-engineer
description: Terminal-first TUI workflow for the canonical Rune app and legacy clients. Use for Bubble Tea state, command palette, Rune/facet/graph/run/artifact views, keyboard flows, layout, editor integration, and clipboard behavior.
---

# Rune TUI Engineer

Use this skill for `internal/app/**` changes.

## Rules

- Keep application state transitions explicit and testable, with storage and run services behind injected interfaces.
- Build composable screens/views over shared Rune domain view models; do not keep expanding the legacy monolithic `Model` for new graph, sync, or run workflows.
- Preserve terminal-first quick capture, Vim-style movement where useful, and discoverable command-palette actions.
- Make Rune facets, parent/child links, run lifecycle, artifact availability, and sync/conflict state visible in compact terminals.
- Consume the shared application/client contract and query semantics; do not create TUI-only lifecycle, filter, or hierarchy behavior.
- Update footer/help text with keybinding behavior.
- Keep status messages transient, specific, and revision-safe.
- Avoid terminal-width overflow in rendered lines.
- Keep editor and clipboard dependencies injectable for tests.
- Use temp stores in tests and manual smokes.
- Treat `sync` as the shared boundary behind the client; do not imply that the TUI is a separate sync daemon or source of truth.

## Validation

- Run `go test ./internal/app` for focused TUI changes.
- Run `go test ./...` before publishing.
- For visual changes, inspect `go run ./cmd/rune` with a temp `RUNE_HOME` when practical; for structured Rune work, also exercise graph, run, artifact, and conflict views at compact widths.
