# Risk Class Validation Matrix

| Risk class | Examples | Minimum validation |
| --- | --- | --- |
| Docs-only agent config | `AGENTS.md`, `.codex/skills/**`, `tools/agents/**` | `python3 tools/agents/scripts/validate_agent_config.py`, hook syntax checks, `git diff --check` when Git exists |
| CLI output and flags | `cmd/rune/**`, usage text, JSON, stdin | `go test ./cmd/rune`, then `go test ./...` when behavior changed |
| Store writes | add/edit/done/tag/archive/import/restore/path | temp-dir tests in `internal/core`, then `go test ./...` |
| Markdown parser | metadata recovery, body slicing, nesting depth | focused parser/store tests with realistic Markdown fixtures |
| TUI state | keybindings, filters, search, add, archive confirm | `go test ./internal/app`, plus manual temp-store smoke for visible behavior when practical |
| Build or dependency | `go.mod`, `go.sum`, local binary flow | `go test ./...`, relevant build or version smoke |
| Release or CI | `.github/**`, installer, release docs | local Go validation plus hosted-check notes |
