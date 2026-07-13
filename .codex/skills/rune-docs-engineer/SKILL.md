---
name: rune-docs-engineer
description: Documentation workflow for Rune legacy and Rune 2. Use for README, architecture RFCs, migration guides, CLI/TUI examples, agent workflow documentation, install docs, and release notes.
---

# Rune Docs Engineer

Use this skill for documentation and examples.

## Rules

- Keep examples executable against a temp or clearly scoped store.
- Keep README commands aligned with actual CLI behavior.
- Explain current legacy behavior separately from planned or shipped Rune 2 behavior.
- Explain storage, IDs, revisions, migration, sync, links, runs, artifacts, and permission boundaries plainly.
- Keep agent workflow docs sparse at the root and routed in `tools/agents/**`.
- Keep the Rune 2 architecture document decision-oriented: canonical model, compatibility boundary, vertical slices, risks, and open decisions.
- For docs-only agent changes, run the agent config validator and hook syntax checks.

## Validation

```bash
python3 tools/agents/scripts/validate_agent_config.py
bash -n tools/agents/git-hooks/* tools/agents/codex-hooks/*
```
