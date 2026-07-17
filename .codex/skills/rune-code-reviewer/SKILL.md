---
name: rune-code-reviewer
description: Final diff review for Rune legacy and canonical workspace changes. Use to find correctness bugs, Rune/client-contract drift, migration or sync regressions, graph/run/artifact safety issues, TUI/CLI drift, missing tests, and validation gaps before commit or PR.
---

# Rune Code Reviewer

Use this skill for final review before delivery.

## Review Priorities

Lead with findings by severity and cite file paths and lines when possible.

Check for:

- Legacy/v2 boundary violations or a second unowned source of truth.
- Rune identity/facet drift or a client-specific model that bypasses the shared application contract.
- Schema migrations that are not transactional, reversible, or fixture-tested.
- Stable IDs, revisions, deletion, conflict, and offline behavior that can lose data.
- Graph links that are ambiguous, orphaned, duplicated, or derived only from display text.
- Parent/child relationships that rely on Markdown indentation instead of explicit links and order.
- Agent runs that lack permission checks, context provenance, cancellation, clear terminal states, or observable Rune lifecycle transitions.
- Artifacts that lack content hashes, size/type checks, retention rules, or secret-handling decisions.
- CLI behavior that changes stdout, stderr, exit codes, stdin handling, or scope resolution unexpectedly.
- Store behavior that can lose legacy Markdown body text, metadata comments, IDs, nesting, tags, or archive paths.
- TUI behavior that leaves help text, keybindings, focus, status, or layout inconsistent.
- Tests that touch the user's real home directory or note store.
- Missing focused tests for parser, CLI, and state-transition changes.
- Git or PR claims that are not backed by actual repo state.
- `sync` claims that imply a daemon, hosted service, or auth behavior not implemented and validated locally.

If no issues are found, say that clearly and mention any remaining test gaps,
manual checks, or intentionally deferred sync/provider/cloud behavior.
