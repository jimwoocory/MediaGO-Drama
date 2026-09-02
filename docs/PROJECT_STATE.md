# ComicGO Project State

> AI entrypoint. Read this file first. Do not reconstruct project history from old chat logs or scan the whole repository by default.

## Product

- Current product name: ComicGO.
- Repository path: `D:\openai\MediaGo-Drama` (legacy directory name is retained for compatibility).
- Base: MediaGo Drama v0.1.8 plus local ComicGO changes.
- Current branch at this optimization checkpoint: `checkpoint/pre-team-20260901`.

## Current priority

1. Keep ComicGO development stable and recoverable.
2. Reduce Codex/ACP context growth and repeated token consumption.
3. Preserve AgentDock task-to-task coordination without spawning multiple Codex agents by default.
4. Continue UI/workspace refinement only after the low-token workflow is stable.

## AI execution policy

- Use short task sessions. One task, one bounded context.
- Read only files required by the current task.
- Do not recursively scan `node_modules`, `dist`, `coverage`, build/release output, or historical plan documents unless the task explicitly requires them.
- Do not read the entire `docs/plans` directory as project memory.
- Prefer AgentDock native file/shell/git/task tools for inspection, build, file moves, diffs and validation.
- Use ACP/Codex only when code reasoning or implementation materially benefits from a coding agent.
- Do not run multiple Codex workers in parallel unless the task explicitly requires it.
- Keep build output out of model context: save full output to a log and return only a short tail on failure.

## ACP context control

ComicGO now bounds backend ACP conversation reuse:

- `agent.max_session_turns` default: **4**.
- The ACP process may stay resident for startup efficiency.
- After the turn budget, the backend ACP conversation is intentionally rotated.
- The fresh ACP session receives the existing compact recap (bounded by the server recap budget), so important recent decisions survive without keeping the unbounded provider context.
- Persisted ACP sessions loaded after a server restart are treated as having unknown historical size and rotate on the next bounded run.

Config: `services/server/configs/server.yaml`.

## Task handoff format

Every meaningful task should end with a compact handoff containing only:

- Goal
- Changed files
- Verified result
- Remaining risk/blocker
- Next task

Update this file only when project-wide state changes. Task-specific detail belongs in the AgentDock task checkpoint, not here.

## Verification

For low-noise validation use:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/agent-verify.ps1 -Target server
```

Full logs are written under `.agent-logs/` and are ignored by Git.
