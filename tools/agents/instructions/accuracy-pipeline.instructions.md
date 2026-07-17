# Accuracy Pipeline Instructions

Use for non-trivial implementation, storage-sensitive behavior, canonical Rune
domain work, migration/sync, graph links, agent runs, artifacts, TUI flow changes,
multi-step CLI work, or draft PR publishing.

## Pipeline

1. Preflight Git/worktree state and note whether this checkout has `.git`.
2. Build a context bundle with `rg`, the relevant Rune RFC section, the shared
   client/`sync` boundary, and nearby tests.
3. Produce a plan with domain routing and validation gates.
4. Audit the plan before broad edits.
5. Implement in dependency order: Rune/facet contract, domain,
   storage/adapter, shared application/client service, then client/worker.
6. Run targeted validation, then broader validation.
7. Review the diff for correctness, note-file safety, and test gaps.
8. Commit, push, and open a draft PR only when Git and remote context are available or explicitly requested.

## Context Bundle

Gather only facts needed for the task:

- owning packages and nearby tests
- current CLI command behavior and output contracts
- current TUI state, keybindings, layout, and status behavior
- Markdown store shape, metadata comments, ID semantics, archive/import/restore paths
- Rune/facet identities, revisions, migration boundary, hierarchy, shared
  query semantics, graph links, run lifecycle, artifact policy, and `sync`
  behavior
- validation commands likely required
- docs, install, or release impact
- permission, secret-handling, and hosted-service impact

When subagents are available, use independent explorers for code context, risk review, and test discovery. Explorer output must be factual and path-grounded.

## Implementation Slices

Parallelize only when write sets do not overlap. Tell implementation workers they are not alone in the codebase and must not revert unrelated changes.

Useful slices:

- CLI behavior and command output
- TUI state transitions and rendering
- Markdown store and file safety
- domain/schema/migration
- graph/link behavior
- run/worker/artifact behavior
- docs and examples
- tests and review

## Stop Rules

Stop and report when:

- the request conflicts with a hard invariant
- the storage or scope behavior cannot be identified locally
- the canonical Rune source of truth, shared client contract, or legacy adapter boundary is unspecified
- a sync, run, or artifact change lacks an explicit conflict, permission, or retention policy
- validation fails after three focused fixes
- continuing would touch the user's real note store unnecessarily
- continuing would stage, revert, or overwrite unrelated dirty work
