# Agent instructions

These rules apply to any AI agent working in this repository.

## Scope

- Implement only what the user explicitly requested. Don't add anything
  else on your own: commands, struct fields, files, features, docs, tests,
  or stand-in implementations.
- If something extra seems needed, ask for confirmation with your
  interactive question or confirmation tool (for example AskUserQuestion in
  Claude Code) and wait for the answer before adding it. If you have no such
  tool, ask and stop. Never mention an addition only in a reply or a plan.
- When asked to simplify or remove things, remove everything the user didn't
  name instead of keeping what you think is useful.
