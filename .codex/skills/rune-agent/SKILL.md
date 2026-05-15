---
name: rune-agent
description: Full feature-to-PR workflow for Rune. Use when a feature or fix should move from request to context bundle, audited plan, implementation, validation, review, and draft PR packaging when Git is available.
---

# Rune Agent Pipeline

Use this skill for full implementation work or when the user asks for orchestration around Rune.

## Pipeline Contract

1. Preflight Git availability, branch, and worktree state.
2. Build a context bundle from local repo search.
3. Produce an architecture plan for non-trivial or cross-module work.
4. Audit the plan before broad edits.
5. Implement locally or from a feature worktree, depending on Git availability and user direction.
6. Run targeted validation, then `go test ./...` when code changed.
7. Review the diff for correctness, storage safety, and test gaps.
8. Commit, push, and open a draft PR only when Git and remote context are available or explicitly requested.

Never invent branch, commit, push, or PR status when this directory has no `.git`.

## Preflight

- Run `git rev-parse --show-toplevel` before using Git workflow language.
- If Git exists, run `git branch --show-current` and `git status -sb`.
- If on `main`, create a worktree with `tools/agents/scripts/pre_worktree.py` before tracked implementation work.
- Load `tools/agents/instructions/index.md`, matching instruction files, and relevant Rune skills.

## Context Bundle

Use `rg` before designing. Include owning packages, nearby tests, CLI/TUI behavior, note-store safety risks, and validation gates.

When subagents are available and the user explicitly asked for them, use independent explorers for code context, risk review, and test discovery.

## Review And Delivery

Run `rune-code-reviewer` logic on the final diff. Fix correctness, note-store safety, and test gaps before packaging. Draft PR descriptions must cover summary, validation, note-store safety, and residual risk.
