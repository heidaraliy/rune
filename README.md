
<div align="center">

<pre>
 '||''| '||  ||` `||''|,  .|''|,
  ||     ||  ||   ||  ||  ||..||
.||.    `|..'|. .||  ||. `|...
</pre>

<p>
  <strong>Rune</strong> is a small terminal-native task tracker for catching ideas before they disappear.
</p>

<p>
  <code>rune add "fix stuns" --tag combat,bug</code>
</p>

Rune stores plain Markdown in `~/notes` (and optionally, within your project's root directory, under `.rune`), detects the current git project, and gives every item a short ID that's easy to use from shell.

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

The default Rune commands write Markdown. The current legacy path has no hosted
service or sync layer.

```text
~/notes/
  projects/<project>.md
  archive/YYYY-WW.md
```

Set `RUNE_HOME` to use another store.

```sh
RUNE_HOME="$(mktemp -d)" rune add "try rune safely" --project scratch
```

Rune 2 is available as an opt-in local structured-store preview. It uses SQLite
at `RUNE_HOME/rune-v2.db` by default, or an explicit `--db` path, and does not
sync to the cloud yet:

```sh
rune v2 capture "design sync" --project rune
rune v2 list --project rune
rune v2 import path/to/notes.md --project rune --db /path/to/rune-v2.db
```

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
rune v2 <init|capture|list|show|edit|status|search|link|links|import> ...
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
