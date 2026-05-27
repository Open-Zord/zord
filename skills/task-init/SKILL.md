---
name: task-init
description: Spawn one planning sub-agent per task and run them in parallel — for each task, create a dedicated worktree and a background sub-agent loaded with the task context that returns an implementation plan (it does not write code). Use when the user says "start <task>", "kick off these tasks", or passes a list of task identifiers to fan out work.
version: 1.0.0
capabilities: [load_skill, spawn_subagent, run_shell, read_file, vcs]
---

Kicks off one or more tasks the standard way: each task becomes a dedicated
**worktree** plus a **background sub-agent** loaded with the task context. The
sub-agent investigates and returns an **implementation plan** — it does not code.
When the plan comes back, the user reviews it, adjusts, and greenlights the
implementation (done by another sub-agent or in the main session).

It removes the friction of spawning parallel work: instead of doing it by hand
for each task (create worktree, open a terminal, paste context), one invocation
returns N plans ready for review.

## Task source

This skill is **task-source agnostic** with a default. Out of the box a task is
an OpenSpec change under `openspec/changes/<change-id>/` (`proposal.md`,
`design.md`, `tasks.md`). A team on Jira/Linear/GitHub Issues adapts only step 1
("read the task") — every other step is unchanged.

Throughout this file, `<task-id>` is the change id (OpenSpec) or the issue key
(tracker).

## When to use

- The user said something like "start payments-refactor", "open changes A, B, C",
  "kick these off", "fan these out".
- The user passed a list of task identifiers expecting each to become a separate
  stream of work.

If no identifier is explicit, **ask** which tasks before touching anything. Do
not infer from recent context — starting the wrong task leaves an orphan
worktree.

## Steps

For **each** task, run the sequence below. Tasks are independent — if one fails,
continue the others and report it in the final summary.

### 1. Read the task

Default (OpenSpec): `read_file` the change directory.

```
openspec/changes/<task-id>/proposal.md   # Why + What Changes
openspec/changes/<task-id>/design.md      # Key Decisions (if present)
openspec/changes/<task-id>/tasks.md       # checklist
```

Extract: title, intent, affected areas, and whether a design already exists.

If the change is already archived (`openspec/changes/archive/...`) or marked
done, **stop** that task and tell the user — starting finished work makes no
sense. Confirm before proceeding (could be a mistake).

> Tracker adaptation: replace this step with the tool's "get issue" call and, if
> the tracker has states, transition the issue to *In Progress* here.

### 2. Infer affected area and branch prefix

- **Affected area**: from the change's `What Changes` / labels. In a multi-module
  repo, this selects which sub-tree (and which role skill — step 4). If it spans
  multiple modules, spawn **one sub-agent per module**. If ambiguous, **ask**.
- **Branch prefix** (heuristic on the title, case-insensitive):
  - starts with fix / bug / resolve → `fix`
  - starts with chore / update / bump / move / rename → `chore`
  - otherwise (default) → `feat`
  - Honor an explicit override ("start X as chore").

### 3. Create the worktree

Worktrees live **outside** the main checkout so the primary working tree stays
clean and never accumulates per-task build artifacts. Default layout (override
via the project's convention if it has one):

```
<repo>/../worktrees/<prefix>-<task-id>     # branch <prefix>/<task-id>
```

Update the main checkout first (`vcs`), then add the worktree:

```bash
git -C <repo> fetch origin <default-branch>
git -C <repo> pull --ff-only origin <default-branch>

git -C <repo> worktree add <repo>/../worktrees/<prefix>-<task-id> -b <prefix>/<task-id>
```

- **If the pull fails** (dirty tree, non-fast-forward): do **not** force. Create
  the worktree from the current HEAD anyway and note in the summary that the base
  is behind. Rebasing later is trivial; clobbering local WIP is not.
- **If the branch already exists** (task being continued): drop `-b` and check
  out the existing branch:
  `git -C <repo> worktree add <path> <prefix>/<task-id>`.
- **If the worktree path already exists**: skip this step, note it, and continue
  to spawn the sub-agent.

Each worktree needs its own setup (`.env`, deps, build artifacts) — they are not
shared with the main checkout.

### 4. Spawn the planning sub-agent (background, parallel)

Use `spawn_subagent` in **background** for each task, and spawn **all of them in
parallel** (one turn). If the harness has no background sub-agents, its adapter
says to run them sequentially.

**Role skill (optional):** if the repo defines a specialized role skill for the
affected area (e.g. a `backend` / `frontend` / `infra` skill in this manifest),
instruct the sub-agent to `load_skill` it first. If there is none, skip — the
sub-agent works from the repo's `AGENTS.md` / `CLAUDE.md` and the code.

**Sub-agent prompt template:**

```
You are a task-init sub-agent. Your mission: produce an implementation PLAN for
one task — do NOT code, do NOT commit, do NOT open a PR.

Task: <task-id> — <title>
Affected area: <area, or "(unknown)">
Branch: <prefix>/<task-id> (already created)
Worktree: <repo>/../worktrees/<prefix>-<task-id>

## Task description

<proposal/design/tasks content, markdown preserved>

## Spec status

[If a design already exists:]
A design is recorded in openspec/changes/<task-id>/design.md. Read it; your plan
must follow it, citing the relevant decisions.

[If no design exists:]
There is no design yet. As part of the plan, PROPOSE the design (Why, What
Changes, Key Decisions, alternatives considered). The user reviews and decides
whether it becomes a formal proposal before implementation.

## Before planning

1. If a role skill applies, load it: load_skill "<role>". Otherwise skip.
2. Read the repo's agent guide (AGENTS.md or CLAUDE.md) and any module-level one.
3. Explore the worktree to understand the current state of the relevant code.
4. If the project ships a code generator / scaffold (CLI or MCP), prefer it over
   hand-writing boilerplate for the artifacts it covers; the plan must name which
   generator commands run at each checkpoint. Hand-editing is for logic the
   generator does not produce.

## Plan format (your final answer)

### Context
2-3 lines: what this task solves and why.

### Design (only if no spec design exists)
Why · What Changes · Key Decisions · Alternatives considered.

### Checkpoints
Small, verifiable steps. For each: what changes (concrete files/areas), how to
verify (test/command/lint), relative size (XS/S/M/L).

### Risks / watch-outs
What can go wrong, external dependencies, fragile areas touched.

### Question(s) for the user
Open decisions needed before coding. If none, "none — ready to implement".

## Constraints

- Do NOT write or edit code. Do NOT commit. Do NOT open a PR.
- You MAY run read-only commands (git status, grep, find, cat) and read files.
- If the project has a scaffold/codegen, the plan must cite which commands run
  per checkpoint instead of describing hand-written boilerplate.
```

### 5. Final summary (after spawning all)

One block per task:

```
<task-id> (<short title>):
  Area:      <area>
  Branch:    <prefix>/<task-id>
  Worktree:  <path> [new | existed]
  Spec:      <design found | "no design — sub-agent will propose one">
  Sub-agent: spawned in background (id: <agent-id>)
```

Add an `Attention:` line for anything that needs it (non-FF base, pre-existing
worktree with diverged branch, etc.). Then totalize: started OK / skipped
(done) / with attention. Tell the user: **"I'll notify you as each plan comes
back. When they arrive I'll show you each one to review/adjust before any
implementation."**

### 6. When sub-agents return

Each sub-agent finishes and returns a plan. **Do not implement automatically** —
always show the plan to the user and wait for an explicit greenlight.

For each returned plan: show it (or a summary with a pointer to the full text),
wait for approve / adjust / discard, and only after approval start the
implementation (a fresh sub-agent prompted to "implement this approved plan", or
the main session for small tasks). Never skip the human review between plan and
implementation.

## Guardrails

- **Never** create a worktree inside the main checkout — it pollutes the parent
  repo's status. Always `<repo>/../worktrees/...`.
- **Never** force `pull --rebase` or a non-`--ff-only` pull automatically. On FF
  failure, branch from current HEAD and warn.
- **Never** fabricate a spec/design just to fill step 1. If missing, the
  sub-agent is told to propose the design in the plan.
- **Never** assume the affected area when inference is ambiguous — ask. Asking is
  cheap; spawning in the wrong module wastes context.
- **Never** let the sub-agent code — the prompt always includes the "do not
  edit / do not commit" constraints. Implementation happens only after human
  review of the plan.
- **Never** spawn more than 5 sub-agents in parallel without confirmation — it
  blows the context/token budget. If the user wants more, ask.

## Notes

- This skill is the opening half of a task lifecycle: `task-init` plans, the user
  approves, implementation happens, and a close step finalizes after merge.
- Capability mapping (`spawn_subagent`, `load_skill`, `run_shell`, `read_file`,
  `vcs`) for your harness lives in `skills/adapters/<harness>/`.
