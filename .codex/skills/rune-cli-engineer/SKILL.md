---
name: rune-cli-engineer
description: CLI workflow for the canonical Rune app and legacy clients. Use for command parsing, structured output, capture/search/link/run/artifact commands, stdin/stdout, scope, and user-facing command text.
---

# Rune CLI Engineer

Use this skill for `cmd/rune/**` and CLI-visible behavior.

## Rules

- Preserve `run(args, stdout, stderr, stdin, cwd)` as the test seam.
- Keep normal output on stdout and errors on stderr.
- Preserve interspersed flags and command aliases where tests or README document them.
- Preserve `RUNE_HOME`, `cwd`, `--project`, `--global` read/search scope, and project detection behavior.
- Keep legacy command output compatible until a documented cutover.
- Route new commands through the shared Rune application/client contract and stable domain/API types rather than formatting storage structs directly.
- Keep JSON output structured, versionable, and stable for scripts, TUI/app clients, mobile clients, and agents.
- Make capture fast, links and `rune://` references addressable, query/filter/sort behavior consistent, run state observable, and artifact references inspectable from the terminal.
- Treat agent execution as queue/run commands with explicit confirmation, permission, and Rune lifecycle transitions; keep one-shot `codex` launch as a compatibility path, not the new core model.
- Treat `sync` as the shared API/protocol boundary. Do not document a daemon or hosted service unless it exists and is validated.
- Keep usage examples aligned with README.

## Validation

- Run `go test ./cmd/rune` for focused CLI changes.
- Run `go test ./...` for behavior that crosses into `internal/core` or `internal/app`.
- Use temp `RUNE_HOME` for manual smokes.
