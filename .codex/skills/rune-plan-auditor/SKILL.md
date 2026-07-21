---
name: rune-plan-auditor
description: Plan review for Rune legacy and canonical workspace work. Use to audit non-trivial plans before edits begin, especially Rune model, facet, schema, migration, graph, sync, agent-run, artifact, CLI, or TUI changes.
---

# Rune Plan Auditor

Use this skill to challenge a plan before broad edits.

## Audit Questions

- Did the plan read the owning package and nearby tests?
- Does it state whether the work is legacy, canonical Rune, or an adapter boundary?
- Does it identify whether the change owns the Rune object, a facet, or a client presentation?
- Does it define one canonical source of truth rather than duplicating state in Markdown comments?
- Do CLI, TUI, app, and future clients use the same application contract, query semantics, and stable references?
- Does it protect the user's real note store with temp `RUNE_HOME` validation?
- Does it preserve legacy Markdown metadata, body text, IDs, nesting, and archive paths where compatibility applies?
- Does it define revision/conflict behavior for syncable data?
- Does it define agent permissions, secret handling, run lifecycle, and artifact retention?
- Does it model graph relationships explicitly rather than inferring them from presentation text alone?
- Does it model parent/child ordering explicitly rather than relying on Markdown indentation?
- Does it preserve stdout, stderr, stdin, cwd, scope, and machine-readable JSON semantics for CLI changes?
- Does it preserve keyboard help, status feedback, and compact layout for TUI changes?
- Does it distinguish Rune lifecycle state from run lifecycle state, including bot claim and completion behavior?
- Does it use `sync` as an API/protocol boundary without assuming a daemon or hosted service exists?
- Is validation proportional to the risk?
- Does packaging depend on Git state that was actually verified?

Return blockers first, then refinements, then a short approval if the plan is ready.
