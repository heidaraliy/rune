# Agent Config Instructions

Use for `AGENTS.md`, `.codex/skills/**`, `tools/agents/**`, hooks, evals, guardrails, and validators.

## Rules

- Keep root `AGENTS.md` sparse and action-oriented.
- Route durable detail through `tools/agents/instructions/**` and repo-local skills.
- Keep skill descriptions triggerable and concrete.
- Mirror the Navia workflow shape, but use Rune-specific domains: CLI, TUI, Markdown store safety, build validation, docs, and repo automation.
- Keep hook files opt-in and safe for a Git checkout, but do not require Git for docs-only local validation.
- When adding a routed instruction file or required skill, update `tools/agents/scripts/validate_agent_config.py`.

## Required Surfaces

- Root contract: `AGENTS.md`
- Instruction index: `tools/agents/instructions/index.md`
- Workflow docs: `tools/agents/README.md`
- Validators: `tools/agents/scripts/validate_agent_config.py`, `tools/agents/scripts/pre_worktree.py`, `tools/agents/scripts/assert_worktree_ready.py`
- Repo-local skills: `.codex/skills/*/SKILL.md`
- Evals and guardrails for CLI, TUI, and store risks

## Validation

Run:

```bash
python3 tools/agents/scripts/validate_agent_config.py
bash -n tools/agents/git-hooks/* tools/agents/codex-hooks/*
```

If Git metadata exists, also run:

```bash
git diff --check
```
