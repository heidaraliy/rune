# Rune Architecture Instructions

Use for canonical Rune, structured domain, storage, migration, graph, `sync`,
agent-run, artifact, API, worker, or cross-client work.

Read `docs/rune-v2-architecture.md` before planning or editing.

## Required Decisions

- Classify the change as legacy maintenance, canonical Rune work, a structured
  store slice, or a migration adapter.
- Identify whether the change owns the Rune object, a facet, or a client
  presentation.
- Name the canonical source of truth and the compatibility boundary.
- Name the shared application/client contract and the `sync` API boundary.
- Use stable IDs and revision-aware mutations for new entities.
- Keep parent/child hierarchy, sibling order, and query/filter/sort semantics
  explicit and shared across clients.
- Define deletion, conflict, offline, and retry behavior when synchronization is involved.
- Define agent permission, cancellation, context provenance, and run terminal states for execution work.
- Define artifact type, hash, size, retention, and secret-handling behavior for generated content.
- Keep graph relationships explicit and typed; do not infer durable edges from terminal or Markdown presentation alone.

## Implementation Order

For cross-module work, prefer:

1. domain types and invariants
2. repository/schema or adapter behavior
3. application service and transaction boundary
4. CLI, TUI, API, or worker projection
5. migration fixtures and focused tests

Do not add a new client surface directly against a storage implementation or
invent a client-specific Rune model.

## Validation

- Legacy Markdown work uses realistic fixtures and temp `RUNE_HOME`.
- Structured storage uses disposable databases and migration tests.
- Sync uses deterministic revisions, idempotency, and conflict fixtures.
- Runs use fake providers and test cancellation/failure/retry paths.
- Artifacts use temp roots and verify hashes, limits, provenance, and cleanup.
- TUI views are checked at compact terminal widths.
