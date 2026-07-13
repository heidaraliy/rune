# CLI Instructions

Use for `cmd/rune/**`, command parsing, flags, stdin/stdout/stderr behavior,
JSON/API output, project/global scope, capture, links, runs, and artifacts.

## Rules

- Preserve the testable `run(args, stdout, stderr, stdin, cwd)` entrypoint.
- Keep normal command output on stdout and user-facing errors on stderr through `printError`.
- Preserve interspersed flags when supported by the command.
- Preserve `RUNE_HOME`, `cwd`, `--project`, `--global` read/search scope, and git-root project detection semantics.
- Preserve legacy commands while v2 commands are introduced; document cutover rather than silently changing their storage.
- Route v2 CLI operations through application services and stable domain types.
- Preserve quoted text decoding for `\n`, `\t`, and `\\` where commands already support it.
- Keep ID resolution prefix-based and keep ambiguity feedback actionable.
- For `--json`, prefer stable versioned domain/API output rather than ad hoc storage strings.
- Make queue/run/artifact output observable, scriptable, and explicit about lifecycle state.
- Avoid smokes against the user's real `~/notes`; use a temp `RUNE_HOME`.

## Validation

- Run targeted CLI tests such as `go test ./cmd/rune`.
- Run `go test ./...` before publishing.
- For manual CLI smokes, set `RUNE_HOME="$(mktemp -d)"` and use a throwaway cwd.
