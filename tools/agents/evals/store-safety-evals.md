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

## Structured Rune Migration And Artifacts

Migrate representative legacy Markdown into a disposable structured store.

Expected:

- Source files remain unchanged unless an explicit export is requested.
- Legacy IDs map to stable v2 IDs and the migration reports unsupported or lossy constructs.
- Re-running the migration is idempotent or reports conflicts without duplicating entities.
- Artifact metadata records a content hash, provenance, limits, and retention decision.

## Rune Content And Hierarchy

With a disposable structured store and Markdown body fixture, edit a document
Rune and a child task-capable Rune.

Expected:

- Markdown body content round-trips without making line position the identity.
- Parent/child links and sibling order remain explicit and stable.
- Task lifecycle state is structured and is not hidden in a new Markdown
  comment field.
