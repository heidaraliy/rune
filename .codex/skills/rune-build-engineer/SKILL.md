---
name: rune-build-engineer
description: Build and validation workflow for Rune legacy and Rune 2 code. Use for Go tests, SQLite/schema migrations, API or worker checks, artifact/run fixtures, dependencies, local binaries, and CI-equivalent commands.
---

# Rune Build Engineer

Use this skill when the task touches Go validation, dependencies, local binaries, or release-adjacent checks.

## Rules

- Prefer targeted package tests first, then `go test ./...`.
- Run `gofmt` or `go fmt ./...` for formatting-sensitive Go edits.
- Keep `go.mod` and `go.sum` changes intentional.
- Use temp `RUNE_HOME` for any command that could create or rewrite notes.
- Use disposable databases, fake agent providers, local artifact roots, and ephemeral ports for Rune 2 validation.
- Test migrations from representative legacy Markdown before trusting a new structured store.
- Validate sync and worker behavior with deterministic fixtures; do not depend on live cloud services unless the task explicitly requires it.
- Verify the exact local binary path when testing installed commands.
- If the user accesses Rune with `rune` or asks for the app to be updated in PATH, run `tools/agents/scripts/install_path_binary.sh` after tests so the shell-resolved binary is rebuilt.

## Useful Commands

```bash
go test ./...
go test ./cmd/rune ./internal/core ./internal/app
go run ./cmd/rune --version
tools/agents/scripts/install_path_binary.sh
```

For Rune 2 slices, add the focused package, migration, API, worker, or artifact
commands before the full suite. Keep provider and cloud checks explicit when
they cannot run locally.

For manual CLI smokes:

```bash
RUNE_HOME="$(mktemp -d)" go run ./cmd/rune add "smoke task" --project smoke
```
