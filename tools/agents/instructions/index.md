# Agent Instruction Index

Read only the instruction files that match the task.

| Path or task | Instruction file |
| --- | --- |
| before implementation, commit, push, worktree setup, or PR workflow | `pre-worktree-pr.instructions.md` |
| full feature-to-PR pipeline, autonomous implementation, or multi-step feature work | `accuracy-pipeline.instructions.md` |
| `AGENTS.md`, `.codex/skills/**`, `tools/agents/**`, hooks, evals, guardrails | `agent-config.instructions.md` |
| `cmd/rune/**`, CLI command parsing, flags, stdin/stdout/stderr, JSON output, project/global scope | `cli.instructions.md` |
| `internal/app/**`, Bubble Tea state, keybindings, layout, status feedback, editor or clipboard integration | `tui.instructions.md` |
| `internal/core/**`, Markdown parsing, note-file writes, IDs, archive/restore/import/path behavior | `store-safety.instructions.md` |
| `go.mod`, build/test workflow, local binary checks, CI-equivalent validation | `build-validation.instructions.md` |
| `.github/**`, releases, install docs, repo automation, PR publishing | `repo-automation.instructions.md` |

When a task spans domains, read all matching files and load the corresponding Rune skills.
