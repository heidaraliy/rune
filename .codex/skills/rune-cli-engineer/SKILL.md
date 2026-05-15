---
name: rune-cli-engineer
description: CLI workflow for Rune. Use for command parsing, flags, stdin/stdout/stderr, JSON output, project/global scope, and user-facing command text.
---

# Rune CLI Engineer

Use this skill for `cmd/rune/**` and CLI-visible behavior.

## Rules

- Preserve `run(args, stdout, stderr, stdin, cwd)` as the test seam.
- Keep normal output on stdout and errors on stderr.
- Preserve interspersed flags and command aliases where tests or README document them.
- Preserve `RUNE_HOME`, `cwd`, `--project`, `--global` read/search scope, and project detection behavior.
- Keep JSON output structured and stable.
- Keep usage examples aligned with README.

## Validation

- Run `go test ./cmd/rune` for focused CLI changes.
- Run `go test ./...` for behavior that crosses into `internal/core` or `internal/app`.
- Use temp `RUNE_HOME` for manual smokes.
