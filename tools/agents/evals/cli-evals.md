# CLI Evals

Use these scenarios when changing `cmd/rune/**` or CLI-visible behavior.

## Quick Capture

Set `RUNE_HOME` to a temp directory, run `rune add "fix stuns" --tag combat,bug`, then `rune list`.

Expected:

- The item is written under the detected project, or an explicit `--project` when outside a project.
- The displayed ID is a shortest unique prefix.
- Tags are normalized and listed.

## Multiline Edit

Append text containing `\n`, `\t`, and backticks through `rune edit <id> --end`.

Expected:

- Escapes are decoded where existing commands support them.
- Body text is preserved in Markdown.
- `rune show <id>` displays the appended content.

## Ambiguous Prefix

Create fixture items with overlapping IDs and resolve a short ambiguous prefix.

Expected:

- The command fails cleanly.
- Matching items are visible enough for the user to choose a longer ID.
