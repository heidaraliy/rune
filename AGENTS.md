---
scope: Project
alwaysApply: true
description: Rune agent entrypoint. Keep this file sparse; detailed workflows live in tools/agents and .codex/skills.
---

# Rune Agent Guide

## Goal

Build Rune as a fast, terminal-native project memory tool. Good work preserves quick capture, plain Markdown storage, short ID workflows, project detection, and safe note-file handling.

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
- Preserve plain Markdown as the source of truth, including `<!-- rune:... -->` metadata, nesting depth, and user-authored body text.
- Preserve shortest-unique-prefix ID semantics and clear ambiguity errors.
- Keep CLI behavior testable through `run(args, stdout, stderr, stdin, cwd)` and keep normal output on stdout, errors on stderr.
- Keep TUI keyboard workflows visible, responsive, and usable in compact terminals.
- Keep root guidance compact; put detailed agent rules in `tools/agents/**` or skills.

## Required Validation

- Go changes: `go test ./...`.
- Formatting-sensitive Go changes: `gofmt` or `go fmt ./...`, then verify no unintended churn.
- CLI or storage changes: add temp-dir tests or run a temp `RUNE_HOME` smoke that does not touch real notes.
- TUI changes: cover state transitions or render helpers with tests; inspect manually when behavior depends on a live terminal.
- Agent config/docs changes: `python3 tools/agents/scripts/validate_agent_config.py`, `bash -n tools/agents/git-hooks/* tools/agents/codex-hooks/*`, and `git diff --check` when Git metadata exists.

## Failure Handling

Read the first meaningful error and fix the root cause. If the same validation failure persists after three focused attempts, stop and report what was tried, what failed, and the most likely next fix.
