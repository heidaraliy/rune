---
name: rune-feature-architect
description: Architecture planning for Rune legacy and Rune 2 work. Use before implementing non-trivial storage, schema, migration, graph, sync, agent-run, artifact, TUI, CLI, API, or cross-module behavior.
---

# Rune Feature Architect

Use this skill before non-trivial implementation. Read
`docs/rune-v2-architecture.md` for Rune 2 work and state explicitly whether
the change is legacy maintenance, a v2 slice, or a migration adapter.

## Planning Inputs

- Read the relevant source packages with `rg` and nearby tests.
- Identify whether the change owns domain, storage, migration, sync, graph, runs, artifacts, CLI, TUI, docs, or automation.
- Define the smallest write scope.
- Define the canonical source of truth and the adapter boundary.
- Define IDs, revisions, transactions, conflict behavior, and permission boundaries when applicable.
- List tests and manual smokes, including temp `RUNE_HOME` requirements.
- Surface storage, migration, sync, artifact, ID, project-scope, and terminal-layout risks.

## Output Shape

- Current behavior:
- Proposed behavior:
- Compatibility boundary:
- Vertical slice:
- Owning files:
- Domain and storage contract:
- Migration/sync implications:
- Risks:
- Validation:
- Packaging impact:
