# Note And Artifact Safety Guardrails

- Treat the user's real `~/notes` as production data.
- Use temp `RUNE_HOME` for tests, manual smokes, and reproductions.
- Avoid commands that archive, import, restore, or rewrite notes outside a controlled temp store.
- Preserve user-authored Markdown, comments, indentation, and body text whenever possible.
- Centralize path behavior through `internal/core` helpers.
- When a behavior intentionally rewrites Markdown layout, document the normalization and add a fixture test.
- Treat Rune 2 databases, sync cursors, tombstones, and artifact roots as production data too.
- Keep structured v2 state canonical for new behavior; do not hide it in legacy comments.
- Make migrations dry-run capable, loss-aware, and source-preserving by default.
- Store artifact metadata and hashes separately from large blobs; enforce type, size, retention, and secret-handling policy.
