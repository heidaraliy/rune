---
name: rune-code-reviewer
description: Final diff review for Rune. Use to find correctness bugs, storage regressions, TUI behavior drift, missing tests, and validation gaps before commit or PR.
---

# Rune Code Reviewer

Use this skill for final review before delivery.

## Review Priorities

Lead with findings by severity and cite file paths and lines when possible.

Check for:

- CLI behavior that changes stdout, stderr, exit codes, stdin handling, or scope resolution unexpectedly.
- Store behavior that can lose Markdown body text, metadata comments, IDs, nesting, tags, or archive paths.
- TUI behavior that leaves help text, keybindings, focus, status, or layout inconsistent.
- Tests that touch the user's real home directory or note store.
- Missing focused tests for parser, CLI, and state-transition changes.
- Git or PR claims that are not backed by actual repo state.

If no issues are found, say that clearly and mention any remaining test gaps or manual checks.
