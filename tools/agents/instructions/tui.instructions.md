# TUI Instructions

Use for `internal/app/**`, Bubble Tea state, structured Rune client screens, keyboard
workflows, status feedback, graph/run/artifact views, and editor or clipboard
integration.

## Rules

- Keep state transitions in `Model.Update` explicit and testable.
- Keep storage and execution services behind interfaces and render shared Rune
  view models; do not grow the legacy monolithic model with new domain
  ownership.
- Preserve modes for normal, search, add, and archive confirmation flows.
- Preserve quick capture while adding a discoverable command palette and explicit views for links, runs, artifacts, and sync conflicts.
- Keep keyboard help and footer text aligned with actual behavior.
- Keep status toasts transient and revision-safe.
- Do not let long titles, paths, tags, status text, or body snippets exceed compact terminal widths.
- Keep real editor and clipboard integration injectable for tests.
- Keep graph neighborhoods readable in terminals; do not require a graphical canvas for core navigation.
- Use the shared application contract for Rune facets, hierarchy, query
  semantics, lifecycle state, and the `sync` boundary; do not create TUI-only
  behavior for those concerns.
- Preserve quick capture ergonomics: add above/below, search, filter, project/global toggle, archive confirmation, and task toggling.
- Use pure helpers for layout and render decisions when practical.

## Validation

- Run targeted TUI tests such as `go test ./internal/app`.
- Run `go test ./...` before publishing.
- For visible UI changes, run `go run ./cmd/rune` with a temp `RUNE_HOME` and inspect common viewport sizes when possible.
