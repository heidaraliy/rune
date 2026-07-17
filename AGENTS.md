---
scope: Project
alwaysApply: true
description: Rune agent entrypoint. Keep this file sparse; detailed workflows live in tools/agents and .codex/skills.
---

# Rune Agent Guide

## Goal

Build `rune` as the canonical terminal-first, local-first workspace for
braindumps, proposals, tasks, links, agent runs, and artifacts. The long-term
product is one shared workspace across the terminal, desktop, mobile, and
cloud while remaining useful offline.

The current implementation is a legacy Markdown task tracker. Protect it during
the transition, but do not mistake its line-oriented storage model for the
structured Rune model. `sync` is the shared API/protocol boundary for workspace
state and replication; it is not a required daemon or separate product name.

## Load Order

1. Read this file first.
2. Before repo-tracked implementation, commit, push, worktree setup, or PR work, read `tools/agents/instructions/pre-worktree-pr.instructions.md`.
3. Read `tools/agents/instructions/index.md` and only the instruction files that match the touched paths or task domain.
4. For non-trivial features, cross-module behavior, storage migration, TUI flow changes, or PR publishing, read `tools/agents/instructions/accuracy-pipeline.instructions.md`.
5. Load every relevant Rune skill before planning or editing.
6. Search local repo context with `rg` before designing or changing behavior.

## Skill Routing

- `rune-agent`: full feature pipeline from context bundle through audited plan, implementation, validation, review, and draft PR.
- `rune-feature-architect`: feature planning and architecture.
- `rune-plan-auditor`: plan review before implementation.
- `rune-code-reviewer`: final diff review before merge.
- `rune-build-engineer`: Go build, test, local install, and validation triage.
- `rune-cli-engineer`: CLI command parsing, flags, stdin/stdout/stderr, JSON output, and project scope behavior.
- `rune-tui-engineer`: Bubble Tea model/update/view work, keyboard flows, status feedback, layout, and clipboard/editor integration.
- `rune-store-safety-engineer`: Markdown store, IDs, metadata comments, archive/restore/import, and file-write safety.
- `rune-docs-engineer`: README, examples, install docs, release notes, and contributor guidance.

Use the smallest skill set that covers the task.

## Hard Invariants

- When this directory is a Git checkout, never implement, commit, or push feature work from `main`.
- If `.git` is absent, make local edits only and report that commit, push, worktree, and PR packaging are unavailable.
- Never run destructive experiments against the user's real `~/notes`; use temp `RUNE_HOME` for tests and smokes.
- For legacy paths, preserve plain Markdown, `<!-- rune:... -->` metadata, nesting depth, and user-authored body text.
- For structured Rune paths, use a structured domain/store as the source of truth for identity, hierarchy, state, revisions, and relationships. Treat Markdown as a first-class content/editing surface plus a legacy adapter; never add new state by sprinkling more comment fields into legacy files.
- Preserve shortest-unique-prefix ID semantics and clear ambiguity errors.
- Keep CLI behavior testable through `run(args, stdout, stderr, stdin, cwd)` and keep normal output on stdout, errors on stderr.
- Keep TUI keyboard workflows visible, responsive, and usable in compact terminals.
- Keep Runes, links, runs, and artifacts addressable by stable IDs and revision-aware APIs. Notes, proposals, braindumps, and task-capable items are facets or presentations over that shared identity.
- Keep CLI, TUI, app, and future clients on the same application contract, query semantics, and machine-readable references.
- Treat agent execution as a permissioned, auditable lifecycle; do not persist secrets or hidden model reasoning as ordinary artifacts.
- Keep root guidance compact; put detailed agent rules in `tools/agents/**` or skills.

## Required Validation

- Go changes: `go test ./...`.
- Formatting-sensitive Go changes: `gofmt` or `go fmt ./...`, then verify no unintended churn.
- CLI or storage changes: add temp-dir tests or run a temp `RUNE_HOME` smoke that does not touch real notes.
- TUI changes: cover state transitions or render helpers with tests; inspect manually when behavior depends on a live terminal.
- Structured Rune storage/sync/run changes: cover schema migration, transaction/conflict behavior, fake providers, artifact hashing, and offline/local paths.
- Local app updates requested for PATH: run `tools/agents/scripts/install_path_binary.sh` and verify the shell-resolved `rune` binary.
- Agent config/docs changes: `python3 tools/agents/scripts/validate_agent_config.py`, `bash -n tools/agents/git-hooks/* tools/agents/codex-hooks/*`, and `git diff --check` when Git metadata exists.

## Failure Handling

Read the first meaningful error and fix the root cause. If the same validation failure persists after three focused attempts, stop and report what was tried, what failed, and the most likely next fix.
