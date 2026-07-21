---
name: rune-docs-engineer
description: Documentation workflow for the canonical Rune app and legacy paths. Use for README, architecture RFCs, migration guides, CLI/TUI examples, agent workflow documentation, install docs, and release notes.
---

# Rune Docs Engineer

Use this skill for documentation and examples.

## Rules

- Keep examples executable against a temp or clearly scoped store.
- Keep README commands aligned with actual CLI behavior.
- Explain current legacy behavior separately from the structured Rune preview and future hosted behavior.
- Explain the Rune object and facets, shared CLI/TUI/app behavior, storage, IDs, revisions, migration, `sync`, links, runs, artifacts, and permission boundaries plainly.
- Keep agent workflow docs sparse at the root and routed in `tools/agents/**`.
- Keep the Rune architecture document decision-oriented: canonical model, compatibility boundary, vertical slices, risks, and open decisions.
- Keep `rune` as the main app name and `sync` as the shared API/protocol boundary; do not use `syncd` as a product name.
- For docs-only agent changes, run the agent config validator and hook syntax checks.

## Validation

```bash
python3 tools/agents/scripts/validate_agent_config.py
bash -n tools/agents/git-hooks/* tools/agents/codex-hooks/*
```
