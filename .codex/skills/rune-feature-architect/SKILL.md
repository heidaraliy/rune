---
name: rune-feature-architect
description: Architecture planning for Rune. Use before implementing non-trivial features, storage changes, TUI workflow changes, or cross-module CLI behavior.
---

# Rune Feature Architect

Use this skill before non-trivial implementation.

## Planning Inputs

- Read the relevant source packages with `rg` and nearby tests.
- Identify whether the change owns CLI, TUI, store safety, docs, or automation.
- Define the smallest write scope.
- List tests and manual smokes, including temp `RUNE_HOME` requirements.
- Surface storage, ID, project-scope, and terminal-layout risks.

## Output Shape

- Current behavior:
- Proposed behavior:
- Owning files:
- Risks:
- Validation:
- Packaging impact:
