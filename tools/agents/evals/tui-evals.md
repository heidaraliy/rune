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
