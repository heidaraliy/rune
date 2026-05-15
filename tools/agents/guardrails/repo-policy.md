# Repo Policy Guardrails

- Preserve unrelated user changes.
- Verify Git availability before using worktree, branch, commit, push, or PR language.
- Do not implement, commit, or push feature work from `main` when Git is available.
- Keep validation evidence explicit in the final response.
- Keep generated or derived docs specific to Rune rather than generic scaffolding.
- Prefer existing package boundaries: `cmd/rune`, `internal/app`, and `internal/core`.
- Do not add provider-specific legacy agent trees unless the repo already uses them.
