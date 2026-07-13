---
name: rune-store-safety-engineer
description: Storage safety workflow for Rune legacy Markdown and Rune 2 structured stores. Use for parsers, migrations, SQLite/schema writes, IDs, revisions, sync conflicts, artifacts, archive/import/restore, and path behavior.
---

# Rune Store Safety Engineer

Use this skill for any behavior that reads, writes, parses, imports, archives, restores, or resolves note files.

## Rules

- Treat the user's note store, structured database, sync state, and artifacts as production data.
- Use temp `RUNE_HOME` in tests and smokes.
- Preserve legacy plain Markdown content, headings, task nesting, body text, and metadata comments.
- Keep the Rune 2 structured store canonical for new behavior; do not encode new domain state solely in Markdown comments.
- Make legacy import/export explicit, loss-aware, and transactional where possible; never silently rewrite source files.
- Preserve internal 8-character IDs and shortest unique display prefixes.
- Use stable v2 IDs and revision/concurrency fields while retaining legacy ID mappings.
- Keep path and storage behavior centralized behind repository/adapter interfaces; legacy paths may continue through `internal/core` during migration.
- Make deletes, conflicts, sync cursors, artifact hashes, size limits, and retention behavior testable and observable.
- Add realistic Markdown fixtures when changing parser or save behavior.

## Validation

- Run `go test ./internal/core` for focused store changes.
- Run `go test ./...` before publishing.
- Add temp-dir/database tests for file-writing, migration, sync, and artifact behavior.
