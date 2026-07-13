---
name: rune-agent
description: Full Rune feature-to-PR workflow for legacy maintenance and Rune 2 workspace work. Use for cross-module changes involving notes, tasks, graph links, structured storage, sync, agent runs, artifacts, TUI/CLI clients, or PR delivery.
---

# Rune Agent Pipeline

Use this skill for full implementation work or when the user asks for
orchestration around Rune. Read `docs/rune-v2-architecture.md` for any task
that crosses the legacy Markdown tracker boundary.

## Pipeline Contract

0. Classify the request as legacy maintenance, Rune 2 work, or a migration boundary.
1. Preflight Git availability, branch, and worktree state.
2. Build a path-grounded context bundle from local repo search.
3. Produce an architecture plan for non-trivial or cross-module work.
4. Audit the plan before broad edits.
5. Implement in dependency order: domain contract, storage/adapter, application service, then CLI/TUI/API/worker surface.
6. Run targeted validation, then `go test ./...` when Go code changed.
7. Review the diff for correctness, data safety, sync/run/artifact invariants, and test gaps.
8. Commit, push, and open a draft PR only when Git and remote context are available or explicitly requested.

Never invent branch, commit, push, or PR status when this directory has no `.git`.

## Preflight

- Run `git rev-parse --show-toplevel` before using Git workflow language.
- If Git exists, run `git branch --show-current` and `git status -sb`.
- If on `main`, create a worktree with `tools/agents/scripts/pre_worktree.py` before tracked implementation work.
- Load `tools/agents/instructions/index.md`, matching instruction files, and relevant Rune skills.

## Context Bundle

Use `rg` before designing. Include:

- the relevant Rune 2 section and current/legacy owner paths
- owning packages and nearby tests
- CLI/TUI/API/worker behavior and compatibility contracts
- storage, migration, ID, revision, and project-scope risks
- graph/link, agent-run, artifact, permission, and sync implications
- validation gates and any manual temp-store smoke

When subagents are available and the user explicitly asked for them, use independent explorers for code context, risk review, and test discovery.

## Rune 2 Boundaries

- Notes, tasks, links, runs, and artifacts are typed domain entities with stable IDs.
- Structured storage is canonical for new behavior; Markdown is an adapter or export surface.
- A graph is a projection over explicit typed links, not a reason to introduce a graph database prematurely.
- Agent runs must have explicit lifecycle, permissions, context snapshots, and artifact references.
- Sync must be revision-aware and conflict-visible; never silently overwrite user data.
- Keep provider-specific execution behind adapters so Codex, Claude, local shell, and remote workers share the same run model.
- Preserve legacy CLI, Markdown, IDs, and temp `RUNE_HOME` behavior until a migration/cutover plan explicitly changes them.

## Stop And Replan

Stop before editing when:

- a proposed v2 change would write structured state only into legacy Markdown comments
- the canonical source of truth, conflict policy, or agent permission boundary is unspecified
- an artifact may contain secrets and the retention/redaction behavior is unspecified
- the work expands from a vertical slice into cloud, mobile, graph, and provider integrations at once

## Review And Delivery

Run `rune-code-reviewer` logic on the final diff. Fix correctness, legacy
note-store safety, structured-storage/migration safety, graph/run/artifact
invariants, and test gaps before packaging. Draft PR descriptions must cover
summary, validation, migration/note-store safety, permissions, and residual
risk.
