# Implementation Accuracy Review

Use before finalizing non-trivial Rune changes.

- Does the diff solve the user's request without expanding scope?
- Is the work clearly legacy maintenance, a v2 slice, or a migration adapter?
- Is the canonical source of truth explicit, with no unowned duplicate state?
- Does the change preserve one Rune identity across CLI, TUI, app, and future
  `sync` clients, with facets rather than parallel note/task models?
- Did you read the owning package and nearby tests before editing?
- Did CLI changes preserve stdout, stderr, stdin, cwd, `RUNE_HOME`, and project/global scope behavior?
- Did TUI changes preserve keyboard discoverability, compact layout, and status behavior?
- Did store changes preserve Markdown content, metadata comments, IDs, nesting depth, archive/import/restore paths, and user body text?
- Did structured hierarchy, sibling ordering, query/filter/sort semantics, and
  `rune://` references remain explicit and shared?
- Did every file-writing behavior get temp-dir coverage or a temp `RUNE_HOME` smoke?
- Did structured storage migrations, revisions, conflicts, and tombstones get fixture coverage?
- Did graph links remain typed, stable, and queryable?
- Did agent runs cover permissions, cancellation, failure, retry, and terminal states?
- Did agent execution visibly transition task-capable Runes between user-facing
  states without confusing run state with Rune state?
- Did artifacts cover hashes, limits, provenance, retention, and secret handling?
- Did you avoid touching real `~/notes` during validation?
- Did `sync` claims stay within the implemented local or hosted boundary, with
  no unsupported daemon/auth/service claims?
- Did you run targeted validation before broader validation?
- If Git metadata is absent, did you report that commit, push, and PR packaging were unavailable?
- Are residual risks explicit and tied to concrete unrun checks?
