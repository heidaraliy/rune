# Repo Automation Instructions

Use for `.github/**`, releases, install docs, Git helpers, publishing, and PR hygiene.

## Rules

- Verify whether this directory is a Git checkout before claiming branch, commit, remote, CI, or PR state.
- Keep agent PRs draft by default unless the user asks otherwise.
- Keep commits scoped to the request and stage only in-scope files.
- Document any hosted checks that were not run locally.
- For install or release docs, prefer commands that work for users without needing Go when release assets exist.
- Do not add CI or release workflows that write to the user's note store.

## Validation

- Run `python3 tools/agents/scripts/validate_agent_config.py` for agent workflow changes.
- Run `go test ./...` for Go or release-script changes that can affect the built binary.
- Validate GitHub Actions YAML when `.github/**` exists or is added.
