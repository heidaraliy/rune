# Store Safety Instructions

Use for legacy `internal/core/**`, Markdown adapters, structured storage,
migrations, IDs, archive/restore/import, artifact paths, project paths, and
`RUNE_HOME` handling.

## Rules

- Treat note-file, database, sync-state, and artifact writes as safety-critical.
- Legacy plain Markdown remains protected and must preserve headings, user-authored body text, task nesting, and metadata comments.
- Structured Rune storage is canonical for identity, facets, hierarchy, state,
  relationships, and revisions; do not add new state only as ad hoc Markdown
  comments.
- Markdown remains a first-class content/editing surface. Keep body round trips
  stable while keeping structured identity and hierarchy outside line positions.
- Make imports/exports explicit, loss-aware, and transactional where possible.
- Keep `<!-- rune:id=... type=... tags=... created=... -->` metadata adjacent to its item when parsing or saving.
- Preserve 8-character internal IDs and shortest unique display prefixes with a minimum of 3 characters.
- Use stable v2 IDs, legacy-ID mappings, revisions, tombstones, and conflict records for new storage.
- Store parent/child links and sibling ordering explicitly rather than treating
  Markdown indentation as canonical hierarchy.
- Validate artifact hashes, size/type limits, provenance, retention, and secret handling.
- Use temp-dir tests for every behavior that creates, edits, imports, archives, restores, or scans note files.
- Do not depend on the developer machine's real home directory, editor setup, clipboard, or Git state in tests.
- When changing archive/restore/import behavior, verify both file content and source path expectations.
- Keep path construction centralized through existing helpers such as `Home`, `ProjectPath`, and `ArchivePath`.

## Validation

- Run targeted store tests such as `go test ./internal/core`.
- Run `go test ./...` before publishing.
- Include manual temp `RUNE_HOME` smokes when a migration or real CLI flow needs extra confidence.
