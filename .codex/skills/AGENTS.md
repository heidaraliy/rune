# Rune Skills

Load only the skills needed for the current task. Use the full `rune-agent`
pipeline for cross-module Rune 2 work; use domain skills for narrow changes.

Rune 2 vocabulary is defined in `docs/rune-v2-architecture.md`. Read that
document when a task involves the structured domain, graph, sync, agent runs,
artifacts, or a legacy-to-v2 migration.

Core routing:

- `rune-agent`: end-to-end feature work and PR-style delivery.
- `rune-feature-architect`: architecture plans before non-trivial edits.
- `rune-plan-auditor`: plan checks before broad implementation.
- `rune-code-reviewer`: final diff review.
- `rune-build-engineer`: Go build, tests, package, and validation.
- `rune-cli-engineer`: CLI command and output behavior.
- `rune-tui-engineer`: Bubble Tea user interface behavior.
- `rune-store-safety-engineer`: Markdown note storage and file safety.
- `rune-docs-engineer`: README, install, examples, and agent docs.
