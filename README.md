# Rune

Rune is a small terminal-native project memory tool.

It is for the stuff I do not want to lose mid-work: bugs, follow-ups, ideas,
and "come back to this later" notes. It stores plain Markdown in `~/notes`,
detects the current git project, and gives every item a short ID that is easy
to use from the shell.

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

## Storage

Rune writes Markdown. No database, no hosted service, no sync layer.

```text
~/notes/
  inbox.md
  projects/<project>.md
  today/YYYY-MM-DD.md
  archive/YYYY-WW.md
```

Set `RUNE_HOME` to use another store.

```sh
RUNE_HOME="$(mktemp -d)" rune add "try rune safely"
```

## IDs

Rune stores 8-character internal IDs, but displays the shortest unique prefix
with a minimum of 3 characters. Commands accept any unique prefix.

If a prefix is ambiguous, Rune prints the matching items and asks for a longer
ID.

## CLI

```sh
rune add "text" [--tag a,b] [--project p] [--global] [--note] [--body "..."]
rune list [--global] [--all] [--done] [--tag t] [--project p] [--json]
rune show <id> [--raw]
rune yank <id>
rune edit <id> --end "..." | --replace "..." | --title "..." | --stdin
rune done <id>
rune undone <id>
rune toggle <id>
rune tag <id> a,b
rune untag <id> a,b
rune find "query" [--global] [--tag t]
rune today ["text"]
rune inbox ["text"]
rune projects
rune tags
rune archive --done [--project p]
rune import <file> --project lune
rune path [<id>|--store]
rune doctor [--fix]
```

Quoted CLI text decodes `\n`, `\t`, and `\\`, so quick terminal capture can
still include Markdown and multiline details.

## TUI

```text
j/k move
space toggle done
a add
e edit in $EDITOR
/ search
f cycle open/all/done
g toggle project/global
A archive completed
q quit
```

## License

MIT.
