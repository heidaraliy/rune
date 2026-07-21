# TUI Evals

Use these scenarios when changing `internal/app/**`.

## Keyboard Flow

Start from a temp store with several tasks, then exercise movement, toggle, filter, search, add, project/global toggle, and archive confirmation.

Expected:

- State transitions are deterministic.
- Footer and status text match available actions.
- No action writes outside the temp store.

## Compact Layout

Render common views at widths around 80 columns.

Expected:

- No rendered line exceeds the model width.
- Long titles, paths, bodies, and status messages truncate or wrap cleanly.

## Editor And Clipboard

Use injected editor and clipboard helpers in tests.

Expected:

- Tests do not open a real editor or write to the real clipboard.
- User-visible status distinguishes success from failure.

## Structured Rune Workspace Views

With disposable domain fixtures, inspect note/task, graph, run, artifact, and
sync-conflict views at compact widths.

Expected:

- Relationships are understandable without a graphical canvas.
- Run lifecycle and artifact availability are visible and stale updates do not overwrite newer state.
- The command palette exposes actions that are not present in the footer.

## Shared Rune Client Contract

Inspect one document Rune, one child task-capable Rune, and one run from the
same disposable workspace through the TUI and another client surface.

Expected:

- IDs, parent/child order, lifecycle state, links, and artifacts agree across
  surfaces.
- A bot claim is visible as Rune `in_progress` while the run is active, and
  completion/failure remains observable after the terminal update.
- Sync status is shown as client state from the shared `sync` boundary, not as
  a second TUI-owned store.
