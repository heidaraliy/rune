---
name: rune-docs-engineer
description: Documentation workflow for Rune. Use for README, examples, install docs, release notes, contributor docs, and agent workflow documentation.
---

# Rune Docs Engineer

Use this skill for documentation and examples.

## Rules

- Keep examples executable against a temp or clearly scoped store.
- Keep README commands aligned with actual CLI behavior.
- Explain storage behavior plainly, especially `RUNE_HOME`, project detection, IDs, archive paths, and TUI keys.
- Keep agent workflow docs sparse at the root and routed in `tools/agents/**`.
- For docs-only agent changes, run the agent config validator and hook syntax checks.

## Validation

```bash
python3 tools/agents/scripts/validate_agent_config.py
bash -n tools/agents/git-hooks/* tools/agents/codex-hooks/*
```
