# Agent Config Evals

Use these scenarios when changing agent workflow files.

## Sparse Root Contract

Question: Can an agent read `AGENTS.md` quickly and discover the right detailed instruction file without scanning everything?

Expected:

- Root stays compact.
- `tools/agents/instructions/index.md` routes every required domain.
- Root skill names match `.codex/skills/*/SKILL.md`.

## Local Directory Without Git

Question: What should an agent do when a local Rune checkout has no `.git`?

Expected:

- Continue local implementation when safe.
- Run non-Git validation.
- Report that worktree, commit, push, and PR packaging were unavailable.

## Validation Command

Question: Does the agent config validator catch missing routed docs and placeholder skills?

Expected:

- `python3 tools/agents/scripts/validate_agent_config.py` fails on missing required surfaces.
- Skill frontmatter includes names and triggerable descriptions.
- Literal escaped newlines from generated placeholders are rejected.
