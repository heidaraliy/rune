
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

Rune stores plain Markdown in `~/notes`, detects the current git project, and gives every item a short ID that's easy to use from shell.

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

Rune writes Markdown. No database, no hosted service, no sync layer.

```text
~/notes/
  projects/<project>.md
  archive/YYYY-WW.md
```

Set `RUNE_HOME` to use another store.

```sh
RUNE_HOME="$(mktemp -d)" rune add "try rune safely" --project scratch
```

## IDs

Rune stores 8-character internal IDs, but displays the shortest unique prefix
with a minimum of 3 characters. Commands accept any unique prefix.

If a prefix is ambiguous, Rune prints the matching items and asks for a longer
ID.

## CLI

```sh
rune add "text" [--tag a,b] [--project p] [--note] [--body "..."]
rune list [--global] [--all] [--done] [--tag t] [--project p] [--json]
rune show <id> [--raw]
rune yank <id> [--print]
rune ticket <id>
rune codex <id>
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
rune path [<id>|--store]
rune doctor [--fix]
```

Quoted CLI text decodes `\n`, `\t`, and `\\`, so quick terminal capture can
still include Markdown and multiline details.

Use `rune show <id>` for a quick human-readable view of an item in the
terminal.

`rune yank <id>` and TUI `y` copy an agent-ready ticket for `$rune-agent` to the
system clipboard. Inside tmux Rune also mirrors the ticket into a `rune-ticket`
tmux buffer, so prefix + paste can send it into another pane without relying on
a remote device clipboard. Use `rune ticket <id>` or `rune yank <id> --print` to
write the ticket to stdout, and `rune codex <id>` to start Codex directly with
that ticket as the prompt.

## TUI

```text
j/k move
space toggle done
a add
e edit in $EDITOR
y yank ticket
c open ticket in Codex
/ search
f cycle open/all/done
g toggle project/global
A archive completed
q quit
```

## License

MIT.
