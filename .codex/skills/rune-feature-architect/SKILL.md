---
name: rune-feature-architect
description: Architecture planning for Rune legacy and canonical workspace work. Use before implementing non-trivial Rune, facet, storage, schema, migration, graph, sync, agent-run, artifact, TUI, CLI, API, or cross-module behavior.
---

# Rune Feature Architect

Use this skill before non-trivial implementation. Read
`docs/rune-v2-architecture.md` for canonical Rune work and state explicitly
whether the change is legacy maintenance, a Rune contract slice, a structured
store slice, or a migration adapter.

## Planning Inputs

- Read the relevant source packages with `rg` and nearby tests.
- Identify whether the change owns domain, storage, migration, sync, graph, runs, artifacts, CLI, TUI, docs, or automation.
- Define the smallest write scope.
- Define the canonical source of truth and the adapter boundary.
- Define whether the change operates on the Rune object, a facet such as task
  execution, or a presentation-only view.
- Define the shared application/client contract used by `rune` surfaces and
  the `sync` API boundary; do not create a surface-specific domain model.
- Define IDs, revisions, transactions, conflict behavior, and permission boundaries when applicable.
- Define parent/child ordering, query/filter/sort semantics, and stable
  machine-readable references when applicable.
- List tests and manual smokes, including temp `RUNE_HOME` requirements.
- Surface storage, migration, sync, artifact, ID, project-scope, and terminal-layout risks.

## Output Shape

- Current behavior:
- Proposed behavior:
- Rune/facet contract:
- Compatibility boundary:
- Vertical slice:
- Owning files:
- Domain and storage contract:
- Shared client and `sync` boundary:
- Migration/sync implications:
- Risks:
- Validation:
- Packaging impact:
