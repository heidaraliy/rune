---
name: rune-store-safety-engineer
description: Markdown storage safety workflow for Rune. Use for internal/core parsing, file writes, IDs, metadata comments, archive/restore/import, and path behavior.
---

# Rune Store Safety Engineer

Use this skill for any behavior that reads, writes, parses, imports, archives, restores, or resolves note files.

## Rules

- Treat the user's note store as production data.
- Use temp `RUNE_HOME` in tests and smokes.
- Preserve plain Markdown content, headings, task nesting, body text, and metadata comments.
- Preserve internal 8-character IDs and shortest unique display prefixes.
- Keep path behavior centralized through `internal/core` helpers.
- Add realistic Markdown fixtures when changing parser or save behavior.

## Validation

- Run `go test ./internal/core` for focused store changes.
- Run `go test ./...` before publishing.
- Add temp-dir tests for file-writing behavior.
