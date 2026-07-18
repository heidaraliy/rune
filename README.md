
<div align="center">

<pre>
 '||''| '||  ||` `||''|,  .|''|,
  ||     ||  ||   ||  ||  ||..||
.||.    `|..'|. .||  ||. `|...
</pre>

<p>
  <strong>Rune</strong> is a terminal-first workspace for capturing ideas,
  documents, tasks, and agent work before they disappear.
</p>

<p>
  <code>rune add "fix stuns" --tag combat,bug</code>
</p>

The `rune` app presents one workspace through the CLI and TUI. Its legacy
commands store plain Markdown in `~/notes` (and optionally under `.rune`),
while the structured preview adds revisioned Runes, links, runs, artifacts,
and local `sync` behavior.

</div>

---

## Install

```sh
go install github.com/heidaraliy/rune/cmd/rune@latest
```

## Quick Start

```sh
rune add "fix stuns" --tag combat,bug
rune list
rune edit 1hc --end "animation plays\n\tbut the mob still walks"
rune done 1hc
rune
```

Run `rune` with no arguments to open the TUI.

Outside a git project, pass `--project` to write directly to
`~/notes/projects/<project>.md`.

## Storage

The default legacy commands write Markdown. The current legacy path has no
hosted service. `sync` is the shared API/protocol boundary for the structured
workspace; the local file-backed peer is only a development harness, not a
hosted service or required daemon.

```text
~/notes/
  projects/<project>.md
  archive/YYYY-WW.md
```

Set `RUNE_HOME` to use another store.

```sh
RUNE_HOME="$(mktemp -d)" rune add "try rune safely" --project scratch
```

The structured Rune workspace is available as an opt-in preview through the
compatibility namespace `rune v2`. It uses SQLite at `RUNE_HOME/rune-v2.db` by
default, or an explicit `--db` path. Execution artifacts live in the
content-addressed `RUNE_HOME/rune-v2-artifacts` directory by default. The
preview does not provision a cloud service, but it can connect to a private
single-workspace sync server for device dogfooding:

```sh
rune v2 capture "design sync" --project rune
rune v2 capture "sync proposal" --note --facets proposal,research --property audience=team --facet-property proposal.decision=pending
rune v2 capture "ready for review" --state ready
rune v2 list --project rune
rune v2 list --state in_progress
rune v2 capture "break proposal into a task" --parent rune://<parent-id>
rune v2 list --parent rune://<parent-id> --sort sibling_order
rune v2 show rune://<id>
rune v2 list --json
rune v2 import path/to/notes.md --project rune --db /path/to/rune-v2.db
rune v2 queue <task-id>
rune v2 run <run-id>
rune v2 cancel <run-id>
rune v2 runs
rune v2 artifacts <run-id>
rune v2 artifact <artifact-id>
rune v2 edit <id> --title "new title"
rune v2 edit <id> --state complete
rune v2 edit <id> --property owner=codex --facet-property proposal.decision=accepted
rune v2 edit <id> --remove-property audience
rune v2 delete <id> --confirm
rune v2 restore <id>
rune v2 sync
rune v2 sync --remote /tmp/rune-remote --artifact-root /tmp/rune-artifacts
rune v2 sync serve --listen 0.0.0.0:8787 --workspace local --token "$RUNE_SYNC_TOKEN"
rune v2 sync --remote https://sync.example --token "$RUNE_SYNC_TOKEN"
rune v2 tui --project rune
rune v2 tui --project rune --remote https://sync.example --token "$RUNE_SYNC_TOKEN" --auto-sync
```

The structured Rune TUI is a client over the shared application contract and
structured store. Use `1`-`4` to switch workspace, runs, artifacts, and local
sync status; `a`/`n` capture; `q` queues a task; `x` executes a run; `c`
cancels it; `/` searches; `f` cycles all/task/note workspace filters; `y`
starts a background sync when a remote is configured; and `Q` quits. Pass
`--remote <directory|http(s) URL>` and, for HTTP, `--token` to configure the
target. `--auto-sync` runs one sync at startup; later syncs are explicit so
offline and conflict outcomes remain visible. Structured Rune edits are
revision-checked and recorded in the local sync ledger.
`rune v2 delete` requires `--confirm` and creates a reversible tombstone rather
than physically removing the item; `rune v2 restore` clears that tombstone.
The TUI exposes `e`/`E` for title/body editing, `s` for cycling authored Rune
state, and displays custom facets and properties alongside the selected Rune.
`d` plus confirmation tombstones, and `u` restores. `rune v2 sync` reports the
local change cursor, pending changes, and visible conflicts. A remote sync
report identifies the embedded `sync.v1` contract. Passing
`--remote <directory>` exercises the revision-aware push/pull protocol against
a disposable file-backed development peer, including artifact blobs. Passing
an HTTP(S) URL uses the authenticated `sync.v1` server and the same change and
artifact protocol. `rune v2 sync serve` is a single-workspace personal server;
use it only on a private network without TLS, or provide `--cert` and `--key`
(or terminate TLS in front of it). Multi-user authorization and production
deployment remain future work. Set `RUNE_SYNC_REMOTE` and `RUNE_SYNC_TOKEN` to
make the endpoint the default for CLI and TUI sync commands.

For notes that should travel with a repository, initialize a project-local
store:

```sh
rune init --project lune
```

That creates `.rune/config.json` and stores each top-level note as its own
Markdown file:

```text
.rune/
  config.json
  projects/<project>/<order-id-title>.md
  archive/<project>/YYYY-WW/<id-title>.md
```

Inside that repo, Rune automatically uses `.rune` unless `RUNE_HOME` is set.
To migrate an existing monolithic project file, run:

```sh
rune migrate ~/notes/projects/lune.md --project lune
```

Migration copies and splits the source file, assigns missing IDs when needed,
and leaves the original Markdown file unchanged.

## IDs

Rune stores 8-character internal IDs, but displays the shortest unique prefix
with a minimum of 3 characters. Commands accept any unique prefix.

If a prefix is ambiguous, Rune prints the matching items and asks for a longer
ID.

## CLI

```sh
rune add "text" [--tag a,b] [--project p] [--note] [--body "..."]
rune list [--global] [--all] [--done] [--tag t] [--project p] [--sort created_at|finished_at] [--reverse] [--json]
rune show <id> [--raw]
rune yank <id> [--print]
rune ticket <id>
rune codex <id> [--minimal|--low|--medium|--high|--xhigh]
rune edit <id> --end "..." | --replace "..." | --title "..." | --stdin
rune done <id>
rune undone <id>
rune toggle <id>
rune tag <id> a,b
rune untag <id> a,b
rune find "query" [--global] [--tag t]
rune projects
rune tags
rune archive --done [--project p]
rune import <file> --project lune
rune init [--project lune]
rune migrate [file] [--project lune] [--force]
rune path [<id>|--store]
rune doctor [--fix]
  rune v2 <init|capture|list|show|edit|delete|restore|status|search|link|links|queue|run|cancel|runs|artifacts|artifact|sync [serve]|tui|import> ...
```

Quoted CLI text decodes `\n`, `\t`, and `\\`, so quick terminal capture can
still include Markdown and multiline details.

Use `rune show <id>` for a quick human-readable view of an item in the
terminal.

`rune yank <id>` and TUI `y` copy an agent-ready ticket to the system clipboard.
Project files use their project agent by default, such as `$lune-agent` for
`projects/lune.md`. Inside tmux Rune also mirrors the ticket into a
`rune-ticket` tmux buffer, so prefix + paste can send it into another pane
without relying on a remote device clipboard. Use `rune ticket <id>` or
`rune yank <id> --print` to write the ticket to stdout, and `rune codex <id>` to
start Codex directly with that ticket as the prompt. Add a reasoning flag such
as `--low`, `--xhigh`, or `--reasoning xhigh` for one-off Codex launches.

Add a top-level project comment to override the ticket handoff text.

```md
<!-- rune-ticket-agent: $custom-agent -->
<!-- rune-ticket-instruction: implement this ticket, $custom-agent -->
```

## TUI

```text
j/k move
H/h/left collapse children
L/l/right unfurl children
space toggle done
a add below
A add above
e edit in $EDITOR
y yank ticket
c open ticket in Codex, then pick reasoning
/ search
f cycle open/all/done
s cycle document/created/finished sort (default: created newest)
S reverse sort direction
r refresh
g toggle project/global
x archive completed
q quit
```

## License

MIT.
