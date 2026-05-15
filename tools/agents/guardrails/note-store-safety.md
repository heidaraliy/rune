# Note Store Safety Guardrails

- Treat the user's real `~/notes` as production data.
- Use temp `RUNE_HOME` for tests, manual smokes, and reproductions.
- Avoid commands that archive, import, restore, or rewrite notes outside a controlled temp store.
- Preserve user-authored Markdown, comments, indentation, and body text whenever possible.
- Centralize path behavior through `internal/core` helpers.
- When a behavior intentionally rewrites Markdown layout, document the normalization and add a fixture test.
