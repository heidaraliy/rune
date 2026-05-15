# Store Safety Instructions

Use for `internal/core/**`, Markdown parsing, note-file writes, IDs, archive/restore/import, project paths, and `RUNE_HOME` handling.

## Rules

- Treat note-file writes as safety-critical.
- Plain Markdown is the source of truth. Preserve headings, user-authored body text, task nesting, and metadata comments.
- Keep `<!-- rune:id=... type=... tags=... created=... -->` metadata adjacent to its item when parsing or saving.
- Preserve 8-character internal IDs and shortest unique display prefixes with a minimum of 3 characters.
- Use temp-dir tests for every behavior that creates, edits, imports, archives, restores, or scans note files.
- Do not depend on the developer machine's real home directory, editor setup, clipboard, or Git state in tests.
- When changing archive/restore/import behavior, verify both file content and source path expectations.
- Keep path construction centralized through existing helpers such as `Home`, `ProjectPath`, `InboxPath`, `TodayPath`, and `ArchivePath`.

## Validation

- Run targeted store tests such as `go test ./internal/core`.
- Run `go test ./...` before publishing.
- Include manual temp `RUNE_HOME` smokes when a migration or real CLI flow needs extra confidence.
