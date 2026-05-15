# Pre-Worktree And PR Instructions

Use before any repo-tracked implementation work, commit, push, or PR creation.

## Git Availability

First check whether this directory is a Git checkout:

```bash
git rev-parse --show-toplevel
```

If that fails, do not invent branch or PR state. Continue with local edits only when appropriate, run non-Git validation, and report that worktree, commit, push, and PR packaging were unavailable.

## Hard Rule

When Git is available, never implement, commit, or push feature work from `main`. The default branch is integration-only unless the user explicitly says this local checkout should be edited in place.

## Pre-Work Gate

Before editing tracked files in a Git checkout:

1. Run `git branch --show-current` and `git status -sb`.
2. If the branch is `main`, empty, or detached from a feature branch, create or move to a feature worktree first.
3. Use the repo helper:

```bash
python3 tools/agents/scripts/pre_worktree.py "some-new-feature"
```

Then continue from the printed worktree path.

When entering an existing feature worktree for new work, fetch the default branch first:

```bash
git fetch origin main
```

If the feature branch has no local task commits, rebase onto `origin/main` before editing. Do not blindly merge `main` into a feature branch.

## Branch Naming

- Use `agent/<slug>` for agent work.
- Keep slugs lowercase with letters, digits, and hyphens.
- Use `codex/<slug>` only when an external workflow explicitly requires it.

## Draft PR Flow

1. Inspect `git status -sb` and `git diff`.
2. Stage only in-scope files.
3. Commit on the feature branch.
4. Run relevant validation.
5. Push with tracking.
6. Open a draft PR against `main`.
7. Report branch, commit, PR, validation, and residual risk.

If validation fails, do not push or open a PR unless the user explicitly asks for a failing draft.
