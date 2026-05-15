# Implementation Accuracy Review

Use before finalizing non-trivial Rune changes.

- Does the diff solve the user's request without expanding scope?
- Did you read the owning package and nearby tests before editing?
- Did CLI changes preserve stdout, stderr, stdin, cwd, `RUNE_HOME`, and project/global scope behavior?
- Did TUI changes preserve keyboard discoverability, compact layout, and status behavior?
- Did store changes preserve Markdown content, metadata comments, IDs, nesting depth, archive/import/restore paths, and user body text?
- Did every file-writing behavior get temp-dir coverage or a temp `RUNE_HOME` smoke?
- Did you avoid touching real `~/notes` during validation?
- Did you run targeted validation before broader validation?
- If Git metadata is absent, did you report that commit, push, and PR packaging were unavailable?
- Are residual risks explicit and tied to concrete unrun checks?
