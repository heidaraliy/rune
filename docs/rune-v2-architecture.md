# Rune 2 Architecture RFC

Status: Slice 3 local execution vertical slice. The structured store and local
fake-provider loop are implemented behind the opt-in `rune v2` namespace; sync,
remote workers, and the new TUI remain future slices.

## North Star

Rune is a terminal-first, local-first workspace for capturing ideas, relating
notes and tasks, queueing work to agents, inspecting execution, and preserving
the resulting artifacts.

The core loop is:

```text
capture -> connect -> execute -> review -> remember
```

Terminal, desktop, mobile, and cloud clients should be different views over the
same domain rather than separate products with separate data models.

## Current Boundary

The current Rune implementation is a local Markdown task tracker. Its useful
compatibility contracts include:

- fast CLI capture and human-readable output
- `RUNE_HOME`, project detection, `--project`, and `--global` scope behavior
- shortest-unique-prefix IDs
- Markdown bodies, headings, nesting, metadata comments, archive paths, and imports
- a Bubble Tea TUI with quick capture, search, filtering, editor, clipboard, and tmux handoff
- one-shot Codex handoff through a generated ticket

The current parser and store derive items from Markdown lines and line offsets.
That remains a protected legacy path, but it is not the Rune 2 storage model.

## Canonical Rune 2 Model

Rune 2 uses typed entities with stable internal IDs, revisions, timestamps, and
workspace ownership.

### Workspace and project

A workspace is the sync and permission boundary. A project/repository is a
useful context within a workspace, not the storage boundary for every note.
Project records may include repository URL, local roots, default agent profile,
and default views.

### Note

A note has a title, Markdown body, structured properties, optional source
location, and links. Markdown remains a useful editing and interchange format,
but note identity and relationships do not depend on line positions.

### Task

A task is a first-class entity with title, description, status, priority,
parent/dependency links, source note, assignee, executor profile, and optional
due/review fields. A task may be presented inside a note, list, board, graph, or
agent queue without changing its identity.

Initial task states are `draft`, `ready`, `queued`, `running`, `blocked`,
`review`, `completed`, `failed`, and `canceled`. The state machine must be
explicit rather than inferred from a checkbox.

### Link

Links are typed edges between entities. Initial relation kinds include
`references`, `contains`, `parent`, `depends_on`, `blocks`, `generated_by`, and
`attached_to`. Wiki-style Markdown links may be imported as a convenience, but
the graph is stored as explicit relations.

The graph is a query and presentation projection over links. Do not introduce a
graph database until relational queries demonstrably stop being sufficient.

### Run

A run records an attempt to execute a task. It includes the agent/provider,
model, context snapshot, workspace/project, repository revision, branch or
worktree, permission policy, lifecycle state, timestamps, visible output, and
the final summary.

Provider-specific behavior belongs behind adapters. Codex, Claude, local shell,
and future remote workers should produce the same Rune run shape.

### Artifact

Artifacts are durable outputs related to notes, tasks, or runs: patches, diffs,
logs, test reports, screenshots, generated files, context bundles, and review
summaries. Store metadata and content hashes in the structured store. Store
large content in a content-addressed local directory or remote object store.

Artifacts need type/size checks, retention behavior, provenance, and explicit
secret-handling rules. Do not persist hidden model reasoning as ordinary run
content.

### Event and revision

Mutations carry an actor/device, revision, timestamp, and operation identity.
The local store and server use revisions or cursors to synchronize. Conflicts
must be visible and recoverable; silent last-write-wins behavior is not an
acceptable default for authored note or task data.

## Storage And Sync

The preferred shape is:

- local SQLite for the offline-first client store
- server PostgreSQL for shared workspace state
- a versioned API for commands, queries, sync cursors, and run observation
- local and remote object storage for artifacts
- SQLite FTS locally, with server search added behind the same query concepts

The local store is authoritative while offline. The server becomes the shared
authority after synchronization. A push/pull protocol should be revision-aware,
idempotent, and explicit about conflicts.

Do not make a project-local `.rune` directory the cloud database. It may contain
workspace configuration, a Markdown export, or a sync pointer, but the
structured local store belongs behind a storage interface.

## Legacy Markdown Migration

Migration is an adapter, not a rewrite of the old parser into a larger comment
format.

1. Read legacy Markdown without changing the source.
2. Create v2 notes/tasks and retain legacy IDs as external mappings.
3. Preserve titles, bodies, headings, nesting, tags, timestamps, and source paths.
4. Report ambiguous links, unsupported constructs, and lossy conversions.
5. Export or mirror Markdown only through an explicit command or configured mode.

Legacy CLI behavior remains supported until v2 reaches parity. New v2 state must
not be written only into `<!-- rune:... -->` comments.

## Terminal Client

The TUI should be a client over application services and view models, not a
second storage implementation. Its main surfaces are:

- command palette and instant capture
- notes and tasks
- saved views and boards
- graph neighborhood and backlinks
- agent queue and run activity
- artifact browser and review state
- sync/conflict status

Keep terminal-first strengths: fast capture, Vim-style movement where useful,
external editor support, compact layouts, stable IDs, and readable text output.
Use a discoverable command palette so the footer does not become the product's
entire information architecture.

## Agent Execution Contract

The shared lifecycle is:

```text
draft -> ready -> queued -> running -> review -> completed
                                  \-> blocked
                                  \-> failed
```

Every run must have an explicit permission policy, cancellation path, context
provenance, and terminal state. Local execution should be useful before remote
workers or cloud scheduling exist.

## Slice Plan

### Slice 1: contract and architecture

Refresh Rune agent guidance and land this RFC. No product-code migration is
included in this slice.

### Slice 2: local structured foundation

Add domain types, repository interfaces, SQLite schema/migrations, stable IDs,
legacy Markdown import, and CLI parity for capture, list, show, edit, search,
status, and links. The initial implementation is exposed behind the opt-in
`rune v2` command namespace while legacy commands continue using Markdown.

### Slice 3: local execution vertical slice

Add task queueing, a fake/local agent adapter, run lifecycle, context snapshots,
content-addressed artifacts, and terminal inspection of the full loop.

The initial implementation queues draft or ready tasks into SQLite, persists a
JSON context snapshot plus provider output under a content-addressed artifact
root, records visible run events, and exposes queue/run/runs/artifacts commands.
The fake provider is deterministic and intentionally does not represent a
remote worker or hidden model reasoning.

### Slice 4: Rune 2 TUI

Build the new client shell over application services with note/task, graph,
queue, run, artifact, and conflict views.

### Slice 5: sync and shared workspaces

Add authentication, server API, revision cursors, conflict records, and remote
artifact storage.

### Slice 6: additional clients and workers

Add web/mobile capture, queue, review, and observation surfaces plus remote
workers using the same API and run contract.

## Slice 1 Acceptance Criteria

- Agent guidance distinguishes legacy Markdown maintenance from Rune 2 work.
- Every Rune-specific skill routes storage, graph, sync, runs, artifacts, CLI,
  TUI, docs, build, and review work to the new contract.
- The root and routed agent docs prohibit adding v2 state through ad hoc
  Markdown comments.
- The RFC defines canonical entities, IDs, revisions, migration, sync, graph,
  run, artifact, client, and slice boundaries.
- Agent config validation and hook syntax checks pass.

## Open Decisions For Slice 2

- SQLite driver and migration framework
- exact workspace/project mapping and local database path
- Markdown block/task representation during import and export
- operation log versus revision snapshots for sync
- authentication provider and server deployment shape
- artifact size limits, retention defaults, and secret scanning
- whether tasks embedded in notes are rendered from links or materialized blocks
