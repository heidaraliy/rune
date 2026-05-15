# CLI Instructions

Use for `cmd/rune/**`, command parsing, flags, stdin/stdout/stderr behavior, JSON output, and project/global scope.

## Rules

- Preserve the testable `run(args, stdout, stderr, stdin, cwd)` entrypoint.
- Keep normal command output on stdout and user-facing errors on stderr through `printError`.
- Preserve interspersed flags when supported by the command.
- Preserve `RUNE_HOME`, `cwd`, `--project`, `--global`, inbox, today, and git-root project detection semantics.
- Preserve quoted text decoding for `\n`, `\t`, and `\\` where commands already support it.
- Keep ID resolution prefix-based and keep ambiguity feedback actionable.
- For `--json`, prefer stable structured output from `internal/core` rather than ad hoc strings.
- Avoid smokes against the user's real `~/notes`; use a temp `RUNE_HOME`.

## Validation

- Run targeted CLI tests such as `go test ./cmd/rune`.
- Run `go test ./...` before publishing.
- For manual CLI smokes, set `RUNE_HOME="$(mktemp -d)"` and use a throwaway cwd.
