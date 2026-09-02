# ComicGO AI Workflow

## Purpose

This workflow prevents long-running Codex/ACP sessions, duplicated repository reads, and verbose command output from dominating token usage.

## Default task-to-task model

`Coordinator -> AgentDock task -> native tools -> one Codex task only if needed -> verification -> checkpoint`

Task-to-task means independent responsibilities and checkpoints. It does **not** mean one persistent Codex conversation per role.

## Context budget rules

1. Start from `docs/PROJECT_STATE.md`.
2. Identify the smallest file set needed for the task before reading code.
3. Prefer `git grep`, targeted directory listings and focused file ranges over broad recursive reads.
4. Do not load old plan documents unless they are directly relevant to the current behavior.
5. Do not paste complete build logs into prompts.
6. End a task after validation; carry forward a compact handoff instead of the transcript.

## Expensive paths to avoid by default

- `node_modules/**`
- `**/dist/**`
- `**/coverage/**`
- `**/release-baseline/**`
- packaged Electron output
- generated binaries/assets
- historical `docs/plans/**` unrelated to the current change
- lockfiles unless dependency resolution is the task

## Codex use boundary

Use Codex/ACP for:

- cross-module implementation,
- non-trivial refactors,
- difficult debugging,
- code generation requiring repository semantics.

Prefer AgentDock native tools for:

- Git status/diff/log,
- running existing tests/builds,
- moving/copying files,
- simple deterministic edits,
- checking paths/configuration,
- collecting concise diagnostics.

## Validation output policy

Success: return one short PASS line per validation stage.

Failure: preserve full log in `.agent-logs/`; return only the final relevant lines. Read more from the log only when the failure cannot be diagnosed from the tail.

## Session rotation

`agent.max_session_turns` bounds successful backend ACP turns. Default is 4. This is intentionally a small number for active development; raise it only when continuity is more valuable than context cost.
