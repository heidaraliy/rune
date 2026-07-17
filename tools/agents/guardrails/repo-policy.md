# Repo Policy Guardrails

- Preserve unrelated user changes.
- Verify Git availability before using worktree, branch, commit, push, or PR language.
- Do not implement, commit, or push feature work from `main` when Git is available.
- Keep validation evidence explicit in the final response.
- Keep generated or derived docs specific to Rune rather than generic scaffolding.
- Prefer explicit Rune boundaries such as `internal/domain`, `internal/application`, `internal/storage`, `internal/graph`, `internal/runs`, `internal/artifacts`, `internal/sync`, and `internal/tui`; keep `internal/core` as the legacy adapter during migration.
- Keep `rune` as the main app and `sync` as the shared API/protocol boundary. Do not introduce `syncd` as a product name or claim hosted sync behavior without implementation evidence.
- Keep one Rune identity and shared query/lifecycle contract across CLI, TUI, app, and future clients.
- Do not add provider-specific legacy agent trees unless the repo already uses them.
- Do not introduce a second source of truth by writing structured Rune state only into Markdown metadata.
- Do not claim cloud, mobile, worker, or hosted-artifact behavior without actual validation evidence.
