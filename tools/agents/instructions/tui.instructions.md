# TUI Instructions

Use for `internal/app/**`, Bubble Tea state, keyboard workflows, layout, status feedback, and editor or clipboard integration.

## Rules

- Keep state transitions in `Model.Update` explicit and testable.
- Preserve modes for normal, search, add, and archive confirmation flows.
- Keep keyboard help and footer text aligned with actual behavior.
- Keep status toasts transient and revision-safe.
- Do not let long titles, paths, tags, status text, or body snippets exceed compact terminal widths.
- Keep real editor and clipboard integration injectable for tests.
- Preserve quick capture ergonomics: add above/below, search, filter, project/global toggle, archive confirmation, and task toggling.
- Use pure helpers for layout and render decisions when practical.

## Validation

- Run targeted TUI tests such as `go test ./internal/app`.
- Run `go test ./...` before publishing.
- For visible UI changes, run `go run ./cmd/rune` with a temp `RUNE_HOME` and inspect common viewport sizes when possible.
