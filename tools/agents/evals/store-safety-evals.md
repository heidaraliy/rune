# Store Safety Evals

Use these scenarios when changing `internal/core/**`.

## Metadata Recovery

Parse Markdown where a `<!-- rune:... -->` metadata comment has drifted below body text.

Expected:

- Parser recovers the metadata for the right item.
- Save normalizes metadata directly under the item.
- User body text remains body text.

## Archive And Restore

Archive completed project items, then restore them.

Expected:

- Open items remain in the project file.
- Completed items move to the expected ISO week archive.
- Restore places archived sections back into the right project file.

## Import Existing Markdown

Import Markdown tasks without Rune metadata.

Expected:

- Every item receives a unique internal ID.
- Original task titles, done states, nesting, and headings remain recognizable.
