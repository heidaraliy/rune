---
name: rune-plan-auditor
description: Plan review for Rune. Use to audit non-trivial implementation plans before edits begin, especially when storage, CLI scope, or TUI workflows can regress.
---

# Rune Plan Auditor

Use this skill to challenge a plan before broad edits.

## Audit Questions

- Did the plan read the owning package and nearby tests?
- Does it protect the user's real note store with temp `RUNE_HOME` validation?
- Does it preserve Markdown metadata, body text, IDs, nesting, and archive paths?
- Does it preserve stdout, stderr, stdin, cwd, and scope semantics for CLI changes?
- Does it preserve keyboard help, status feedback, and compact layout for TUI changes?
- Is validation proportional to the risk?
- Does packaging depend on Git state that was actually verified?

Return blockers first, then refinements, then a short approval if the plan is ready.
