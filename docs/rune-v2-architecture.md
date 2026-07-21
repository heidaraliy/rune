# Rune Architecture RFC

Status: Slice 7C browser dogfood. The local structured
preview, fake-provider loop, Bubble Tea client, revision cursor ledger,
file-backed and HTTP sync peers, revision-aware push/pull, artifact transfer,
visible conflict records, stable `rune://` references, explicit sibling
ordering, shared query/client contracts, persisted facets, canonical lifecycle
state, and custom facet/property authoring are implemented behind the opt-in
`rune v2` namespace. The versioned embedded and HTTP `sync.v1` boundaries are
explicit; multi-user authorization, production deployment, offline PWA/mobile
clients, and remote workers remain future work.

## North Star

Rune is a terminal-first, local-first workspace for capturing thoughts,
proposals, tasks, and working documents; relating them; queueing work to
agents; inspecting execution; and preserving the resulting artifacts.

The core loop is:

```text
capture -> connect -> execute -> review -> remember
```

Terminal, desktop, mobile, and cloud clients should be different views over the
same domain rather than separate products with separate data models.

## Product And Boundary Vocabulary

`rune` is the canonical main app. Its CLI, TUI, desktop, and mobile surfaces
are clients over the same workspace and application contract; none of them
owns a separate note or task model.

`sync` is the shared API and protocol boundary for workspace state, revisions,
conflicts, identity, and replication. It can be embedded locally or offered
later as a hosted service. `sync` is a product boundary, not a required daemon
process, and this project should not use `syncd` as a product name.

A Rune is the durable, addressable workspace object. It has a stable ID,
workspace ownership, revision history, a Markdown body, structured properties,
relationships, and optional capabilities such as task execution. A note, idea,
or task is a Rune; tasks carry task execution capability while ideas remain
first-class living documents. Proposals, braindumps, decisions, and research
items are other document-oriented Rune presentations, not separate identity
systems.

The transitional v2 implementation may retain `Entity` and `Kind` internally,
but new contracts should use Rune terminology and leave room for additional
facets without turning every document into a task.

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
That remains a protected legacy path, but it is not the structured Rune storage model.

## Canonical Rune Model

The target model uses stable IDs, revisions, timestamps, and workspace
ownership. The current structured preview is an adapter toward this model, not
the final product vocabulary.

### Workspace and project

A workspace is the sync and permission boundary. A project/repository is a
useful context within a workspace, not the storage boundary for every note.
Project records may include repository URL, local roots, default agent profile,
and default views.

### Rune content and facets

Every Rune has a title, Markdown body, structured properties, optional source
location, and links. Markdown is a first-class editing and interchange surface
across CLI, TUI, and app clients; identity, relationships, state, and revisions
must not depend on Markdown line positions or comment placement.

Document-oriented Runes include notes, proposals, braindumps, decisions, and
research. Their content can be edited without first converting them into tasks.

### Task capability

A task-capable Rune has status, priority, parent/dependency links, source
context, assignee, executor profile, and optional due/review fields. It may be
presented inside a document, list, board, graph, or agent queue without changing
its identity.

Any work-bearing Rune, including a note used as a todo, may use the user-facing
lifecycle `draft`, `ready`, `in_progress`, and `complete`, with `blocked`,
`review`, and `failed` as explicit exceptional or review states.
Queue and run states such as `queued`, `running`, and `completed` describe an
execution attempt, not the Rune's entire content lifecycle. The state machine
must be explicit rather than inferred from a checkbox.

Rune edits are revision-checked and preserved in the sync ledger. Deletion is a
reversible tombstone: Rune never physically removes authored items, and restore
clears the tombstone with another revisioned change.

### Content, hierarchy, and query contract

Child todos are separate Runes with explicit `parent` or `contains` links and a
stable sibling order. Markdown indentation may be imported or rendered, but it
is not the canonical hierarchy.

Filter, sort, search, and saved-view semantics belong to the shared application
contract so CLI, TUI, app, and future `sync` clients produce consistent results.
Stable `rune://<id>` references should be usable in human text, Markdown,
terminal output, and agent context bundles.

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
acceptable default for authored note, idea, or task data.

## Storage And Sync Boundary

The preferred shape is:

- local SQLite for the offline-first client store
- server PostgreSQL for shared workspace state
- the `sync` API boundary for commands, queries, sync cursors, and run observation
- local and remote object storage for artifacts
- SQLite FTS locally, with server search added behind the same query concepts

The local store is authoritative while offline. A hosted `sync` service can
become the shared authority after synchronization. Push/pull must be
revision-aware, idempotent, and explicit about conflicts.

Do not make a project-local `.rune` directory the cloud database. It may contain
workspace configuration, a Markdown export, or a sync pointer, but the
structured local store belongs behind a storage interface.

## Legacy Markdown Migration

Migration is an adapter, not a rewrite of the old parser into a larger comment
format.

1. Read legacy Markdown without changing the source.
2. Create Rune records with document/task facets and retain legacy IDs as external mappings.
3. Preserve titles, bodies, headings, nesting, tags, timestamps, and source paths.
4. Report ambiguous links, unsupported constructs, and lossy conversions.
5. Export or mirror Markdown only through an explicit command or configured mode.

Legacy CLI behavior remains supported until the canonical Rune client reaches
parity. New structured state must not be written only into
`<!-- rune:... -->` comments.

## Terminal Client

The TUI should be a client over the shared Rune application contract and view
models, not a second storage implementation. Its main surfaces are:

- command palette and instant capture
- Rune documents and task-capable Runes
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

The shared task-capability lifecycle is:

```text
draft -> ready -> in_progress -> complete
                    \-> blocked
                    \-> review
                    \-> failed
```

When a bot claims a ready task-capable Rune, it marks the Rune `in_progress`.
A successful run may mark it `complete`; failed, blocked, canceled, and review
outcomes remain visible and do not silently claim success. Every run must have
an explicit permission policy, cancellation path, context provenance, and
terminal state. Local execution should be useful before remote workers or cloud
scheduling exist.

## Slice Plan

### Slice 1: contract and architecture

Refresh Rune agent guidance and land this RFC. No product-code migration is
included in this slice.

### Slice 2: local structured foundation

Add domain types, repository interfaces, SQLite schema/migrations, stable IDs,
legacy Markdown import, and CLI parity for capture, list, show, edit, delete,
restore, search, status, and links. The legacy CLI remains unchanged while the
opt-in `rune v2` namespace uses the structured store.

### Slice 3: local execution vertical slice

Add task queueing, a fake/local agent adapter, run lifecycle, context snapshots,
content-addressed artifacts, and terminal inspection of the full loop.

The initial implementation queues draft or ready tasks into SQLite, persists a
JSON context snapshot plus provider output under a content-addressed artifact
root, records visible run events, and exposes queue/run/runs/artifacts commands.
The fake provider is deterministic and intentionally does not represent a
remote worker or hidden model reasoning.

### Slice 4: structured Rune TUI

Build the new client shell over application services with note/task, graph,
queue, run, artifact, and conflict views.

The initial client is available through `rune v2 tui`. It keeps the legacy TUI
separate, renders workspace entities with typed links, exposes run and artifact
inspection, supports capture/search/queue/execute/cancel actions, and shows the
local sync cursor plus open conflicts.

### Slice 5A: revisioned local sync foundation

Add revisioned edits, reversible tombstones, the local change ledger, monotonic
cursors, conflict records containing both payloads, `rune v2 sync` inspection,
and a TUI sync surface. The local store remains authoritative offline; no
remote state is implied or overwritten.

### Slice 5B: sync and shared workspaces

Add a transport-neutral sync engine, persisted per-workspace cursors, unique
operation identities, revision-aware push/pull, conflict ingestion, and
content-addressed artifact transfer. `rune v2 sync --remote <directory>` uses
a file-backed SQLite peer as a deterministic development harness for two or
more local clients; it is not a hosted service or authentication layer.
Authentication, a versioned server API, hosted deployment, and hosted artifact
storage/retention policy remain after the identity boundary is selected.

### Slice 6A: canonical Rune contract and agent pipeline

Define Rune as the canonical object, `rune` as the main app, and `sync` as the
shared API/protocol boundary. Refresh the agent pipeline, routed skills,
instructions, evals, and README framing around that contract. No Go model
rename or hosted service is included in this slice.

### Slice 6B: canonical model and shared client contract

Introduce the Rune application/client contract over the transitional structured
store, including document/task facets, stable references, explicit parent-child
ordering, shared query semantics, and cross-surface JSON shapes. The initial
contract is implemented through canonical `Rune`, `RuneQuery`, `RuneUpdate`,
and `RuneClient` names while retaining compatibility aliases for the existing
SQLite-backed implementation. Parent capture/edit/list flows accept short IDs
and `rune://` references, and migration backfills deterministic sibling order
for existing rows.

An embedded `sync` API remains a separate slice; this boundary does not claim a
hosted service or replace the legacy Markdown commands.

### Slice 6C: first-class facets and Rune lifecycle

Persist document/task facet data and canonical Rune lifecycle state in the
structured store. Existing task `Status` values remain a compatibility
projection for the current queue/run implementation, while `state` is the
authored-object lifecycle shared by notes and tasks: `draft`, `ready`,
`in_progress`, `complete`, `blocked`, `review`, and `failed`.

The v6 migration backfills facets from the transitional kind and state from
legacy task status. Queueing keeps a Rune ready, execution moves it through
`in_progress` and then `complete`/`failed`, and cancellation returns it to
`ready`. Remote status payloads carry state while accepting older payloads that
only contain status. The CLI and TUI expose the canonical state without
removing legacy status commands.

### Slice 6D: custom facets and facet-scoped properties

Allow Runes to carry normalized custom facets such as `proposal`, `research`,
or `decision` while preserving the built-in `document` facet and task
capability invariants. Store common properties separately from facet-scoped
properties. Existing flat `properties_json` objects decode as common
properties; new structured writes use a versioned envelope in the same column,
so this slice does not require another schema migration.

The shared update contract supports revision-checked property changes and
facet replacement. CLI capture/edit/show/JSON flows author and inspect these
values, and the TUI displays them without creating a second client model.
Custom facets are labels in this slice, not user-defined schemas or executable
behavior.

### Slice 6E: embedded sync client boundary

Define the first versioned embedded sync contract as `sync.v1`. The contract
owns a request, target adapter, and report shape; the application exposes sync
as an optional `SyncClient` capability instead of making every Rune surface
know about the local engine. The embedded implementation remains local and
delegates to the existing revision-aware engine, while the file-backed SQLite
peer remains a disposable development adapter.

This slice does not add a daemon, network transport, authentication, hosted
authority, or a new storage schema. Those clients can be added behind the same
request/report boundary after identity, retry, and server deployment decisions
are made.

### Slice 7A: network sync dogfood

Expose the `sync.v1` boundary over authenticated HTTP for one configured
workspace. The HTTP peer preserves the existing revision-aware change and
content-addressed artifact exchange, while `rune v2 sync serve` provides a
self-hostable SQLite-backed personal server. CLI endpoint/token configuration
and optional TLS make the path usable over a private network without changing
the local file-backed development peer.

This slice does not claim multi-user authorization, hosted deployment, public
internet hardening, or a browser API for queries and commands. It is the
smallest real cross-device path for the existing CLI and future TUI clients.

### Slice 7B: TUI daily-driver sync

Add remote setup, sync-now/auto-sync policy, offline status, conflict
inspection, and compact list/filter improvements to the structured TUI. The
TUI remains a client over the application and `sync` contracts; the CLI owns
target construction and lifetime, while the model receives only an injected
target. `y` starts a non-blocking sync, `--auto-sync` performs one startup
sync, and `f` cycles the workspace between all Runes, tasks, and notes.

### Slice 7C: browser client

Build a thin responsive capture, inbox, detail/edit, lifecycle, search, and
run-observation client over the network API. The first dogfood client is
embedded in `rune v2 web`: it serves a responsive inbox/detail UI and
revision-aware JSON routes over `RuneClient`, with optional remote sync and
token authentication. It reuses Rune JSON and sync shapes rather than creating
a browser-specific note/task model. Offline PWA caching, graph visualization,
native mobile packaging, and rich artifact review follow after this shared
workflow has been used across browser, CLI, and TUI.

### Slice 7D: additional clients and workers

Add richer web/mobile capture, queue, review, and observation surfaces plus
remote workers using the same `sync` API and run contract.

## Slice 6A Acceptance Criteria

- Agent guidance names `rune` as the main app and `sync` as the shared boundary.
- The RFC defines one canonical Rune object with document/task facets, explicit
  hierarchy, shared query semantics, revisions, and stable references.
- Routed guidance distinguishes structured identity/state from first-class
  Markdown editing and legacy Markdown compatibility.
- Agent execution guidance separates Rune lifecycle state from run lifecycle
  state and requires bot transitions to be observable.
- CLI, TUI, app, and future clients are required to use one shared application
  contract rather than surface-specific domain models.
- Agent config validation and hook syntax checks pass.

## Slice 6B Acceptance Criteria

- Canonical Rune names are available to storage callers without breaking the
  transitional `Entity`/`Kind` implementation.
- Runes expose document/task facets and stable `rune://` references in JSON and
  terminal output.
- Parent-child identity and sibling order are explicit, revisioned, queryable,
  and migration-safe.
- CLI capture, edit, list, show, JSON output, and the TUI share the same client
  boundary and stable reference semantics.
- Query sorting is allow-listed, deterministic, and bounded; no user input is
  interpolated as SQL.

## Slice 6C Acceptance Criteria

- v6 structured databases persist normalized facet data and canonical Rune
  state, with deterministic migration backfill.
- Notes and tasks expose state through the shared domain/client contract;
  tasks retain a synchronized legacy status projection.
- Queue, run, completion, failure, cancellation, and remote replay preserve
  state/status consistency and revision semantics.
- CLI JSON/human output and the TUI display state; state edits remain
  revision-checked and tombstone-safe.
- Legacy Markdown commands and source files remain unchanged by structured
  state migration.

## Slice 6D Acceptance Criteria

- Facet names are normalized and validated; every Rune has `document`, and
  task-capable Runes retain `task`.
- Common and facet-scoped properties round-trip through JSON, SQLite, full
  entity sync, and legacy flat property storage without data loss.
- Property and facet mutations are revision-checked, reject inactive facet
  writes, and permit removing a facet together with its stale properties.
- CLI capture/edit/show/JSON output and the TUI expose the same facet/property
  contract with deterministic human-readable output.
- No Markdown files, metadata comments, archive paths, or legacy commands are
  changed by this structured property evolution.

## Slice 6E Acceptance Criteria

- `sync.v1` exposes a stable embedded request/report shape with an explicit
  protocol identifier.
- The application routes sync through a capability boundary and does not
  construct the concrete sync engine directly.
- Sync targets are adapters; the file-backed peer remains usable for
  deterministic temporary-directory tests and local development.
- Existing revision, tombstone, conflict, cursor, and artifact behavior is
  unchanged and remains covered by focused sync and CLI tests.
- Documentation makes no daemon, hosted service, authentication, or cloud
  storage claim beyond the implemented local boundary.

## Slice 7A Acceptance Criteria

- An authenticated HTTP `SyncTarget` round-trips Rune changes, tombstones,
  conflicts, cursors, runs, and content-addressed artifacts through `sync.v1`.
- `rune v2 sync --remote <http(s) URL>` uses the HTTP peer while directory
  remotes retain their existing file-backed behavior.
- `rune v2 sync serve` binds one explicit workspace, requires a bearer token,
  supports optional TLS, and never touches legacy Markdown storage.
- Server request sizes, artifact metadata, workspace scope, protocol version,
  and authorization are validated before mutation.
- Tests use `httptest`, disposable SQLite databases, temporary artifact roots,
  and the CLI `run` seam; no real note store or public endpoint is required.

## Slice 7B Acceptance Criteria

- `rune v2 tui --remote <directory|http(s) URL>` configures a sync target while
  keeping target construction outside the Bubble Tea model; HTTP targets use
  `--token` or `RUNE_SYNC_TOKEN`.
- `y` runs sync asynchronously and reports pushed/pulled counts, busy state,
  offline errors, and open-conflict follow-up without blocking CRUD or run
  interaction; `--auto-sync` is an explicit one-shot startup policy.
- The sync view shows configured target identity, connection/error state,
  cursors, pending changes, and selectable conflict rows with both payloads.
- Workspace search remains available through `/`, while `f` cycles compact
  task/idea/note/all kind filters; existing edit, tombstone, restore, and run flows
  remain discoverable in help/footer text.
- Tests use temporary SQLite stores and file peers; no real `~/notes`, public
  endpoint, or background daemon is required.

## Slice 7C Acceptance Criteria

- `rune v2 web` serves an embedded responsive browser client and requires an
  explicit bearer token; optional TLS is available through `--cert` and `--key`.
- Browser capture supports task, idea, and note Runes; list/search/filter,
  parent/child detail, body edit, lifecycle changes, tombstone/restore, and task
  queueing route through the shared `RuneClient` contract and preserve revision
  checks.
- The browser exposes recent run activity, sync status, and sync-now while
  keeping remote target construction in the CLI/server boundary.
- JSON routes use bounded request bodies, stable Rune/domain shapes, explicit
  error responses, and no direct Markdown-store access; the browser token is
  not persisted by the server.
- HTTP tests use disposable SQLite/artifact roots and `httptest`; live smokes
  use a temporary structured database and do not claim offline PWA behavior.

## Remaining Decisions For Hosted Sync

- remote cursor acknowledgement, retry, and idempotent push/pull protocol
- multi-user authentication, actor/device identity binding, and production
  server deployment shape
- remote artifact storage, size limits, retention defaults, and secret scanning
- conflict review/acknowledgement workflow without mutating authored items
- whether custom facets should gain versioned schemas and validation rules
- whether child Runes are rendered from links, materialized blocks, or both
