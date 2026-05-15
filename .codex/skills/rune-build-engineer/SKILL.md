---
name: rune-build-engineer
description: Build and validation workflow for Rune. Use for Go test triage, formatting, local binary checks, dependency changes, and CI-equivalent commands.
---

# Rune Build Engineer

Use this skill when the task touches Go validation, dependencies, local binaries, or release-adjacent checks.

## Rules

- Prefer targeted package tests first, then `go test ./...`.
- Run `gofmt` or `go fmt ./...` for formatting-sensitive Go edits.
- Keep `go.mod` and `go.sum` changes intentional.
- Use temp `RUNE_HOME` for any command that could create or rewrite notes.
- Verify the exact local binary path when testing installed commands.

## Useful Commands

```bash
go test ./...
go test ./cmd/rune ./internal/core ./internal/app
go run ./cmd/rune --version
```

For manual CLI smokes:

```bash
RUNE_HOME="$(mktemp -d)" go run ./cmd/rune add "smoke task" --project smoke
```
