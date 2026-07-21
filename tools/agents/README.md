# Rune Agent Configuration

Rune keeps root instructions sparse. Detailed agent behavior lives in this tree
and in repo-local skills under `.codex/skills`. Rune architecture vocabulary,
the `rune`/`sync` boundary, and slice boundaries live in
`docs/rune-v2-architecture.md`.

## Layout

- `instructions/`: routed task and path guidance.
- `checklists/`: review prompts for implementation risk and validation scope.
- `guardrails/`: concrete policies that should shape code, tests, and review.
- `evals/`: project-specific scenarios agents should use when judging changes.
- `prompting/`: task and validation report templates.
- `scripts/`: fast local validators and worktree helpers.
- `git-hooks/`: opt-in hooks for local Git users.
- `codex-hooks/`: example hooks for environments that support Codex hook runners.
- `docs/rune-v2-architecture.md`: canonical Rune, shared `sync`, structured workspace, graph, run, and artifact contract.

## Common Commands

```bash
python3 tools/agents/scripts/validate_agent_config.py
bash -n tools/agents/git-hooks/* tools/agents/codex-hooks/*
go test ./...
```

This checkout may be a plain local directory without `.git`. In that state, keep work local, run the non-Git validation above, and say clearly that branch, commit, push, and PR steps were not available.

## Opt-In Hooks

Install hooks only when this directory is a Git checkout and the user wants local enforcement:

```bash
cp tools/agents/git-hooks/pre-commit .git/hooks/pre-commit
cp tools/agents/git-hooks/pre-push .git/hooks/pre-push
chmod +x .git/hooks/pre-commit .git/hooks/pre-push
```

The hooks block default-branch commits and run the agent config validator.
They do not replace Rune migration, storage, sync, or run-safety checks.
