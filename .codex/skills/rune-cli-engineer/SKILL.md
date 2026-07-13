---
name: rune-cli-engineer
description: CLI workflow for Rune legacy and Rune 2 clients. Use for command parsing, structured output, capture/search/link/run/artifact commands, stdin/stdout, scope, and user-facing command text.
---

# Rune CLI Engineer

Use this skill for `cmd/rune/**` and CLI-visible behavior.

## Rules

- Preserve `run(args, stdout, stderr, stdin, cwd)` as the test seam.
- Keep normal output on stdout and errors on stderr.
- Preserve interspersed flags and command aliases where tests or README document them.
- Preserve `RUNE_HOME`, `cwd`, `--project`, `--global` read/search scope, and project detection behavior.
- Keep legacy command output compatible until a documented cutover.
- Route new commands through application services and stable domain/API types rather than formatting storage structs directly.
- Keep JSON output structured, versionable, and stable for scripts, mobile clients, and agents.
- Make capture fast, links addressable, run state observable, and artifact references inspectable from the terminal.
- Treat agent execution as queue/run commands with explicit confirmation and permission options; keep one-shot `codex` launch as a compatibility path, not the new core model.
- Keep usage examples aligned with README.

## Validation

- Run `go test ./cmd/rune` for focused CLI changes.
- Run `go test ./...` for behavior that crosses into `internal/core` or `internal/app`.
- Use temp `RUNE_HOME` for manual smokes.
