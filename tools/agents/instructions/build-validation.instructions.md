# Build Validation Instructions

Use for Go module, local binary, dependency, test, and CI-equivalent workflow changes.

## Rules

- Prefer the repo's existing Go toolchain and commands before adding scripts.
- Keep `go.mod` and `go.sum` changes intentional and explained.
- Use package-targeted tests first for focused failures, then `go test ./...`.
- For formatting-sensitive changes, run `gofmt` or `go fmt ./...`.
- When building a local binary, verify the command path you are testing and avoid assuming a global install location.
- When the user runs Rune as `rune`, or asks to update the app they use from PATH, rebuild the shell-resolved binary with `tools/agents/scripts/install_path_binary.sh` after validation instead of stopping at `go build` or `go run`.
- If a test or smoke could write notes, set `RUNE_HOME` to a temp directory.

## Validation

```bash
go test ./...
go test ./cmd/rune ./internal/core ./internal/app
tools/agents/scripts/install_path_binary.sh
```

Run only the narrower command when the task is docs-only or the broader suite would not add signal.
