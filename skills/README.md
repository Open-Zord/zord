# Agent Skills

Harness-agnostic skills for driving development on this repository with an AI
coding agent. A *skill* is a unit of procedural knowledge — a documented
workflow the agent loads on demand. These skills are written so that **any**
tool can discover and run them, not just one specific harness (Claude Code,
Cursor, Aider, a custom agent, etc.).

## Discovery

Every skill is described in [`manifest.json`](manifest.json) — a single
machine-readable index any tool can parse without understanding a vendor's
private directory layout:

```json
{
  "schemaVersion": "1.0",
  "skills": [
    {
      "name": "task-init",
      "description": "Spawn parallel planning agents, one per task ...",
      "path": "task-init/SKILL.md",
      "version": "1.0.0",
      "capabilities": ["load_skill", "spawn_subagent", "run_shell", "read_file", "vcs"]
    }
  ]
}
```

A tool integrates in three steps:

1. Read `skills/manifest.json`.
2. For each entry, expose `name` + `description` to the model so it can decide
   when the skill is relevant.
3. When a skill is selected, read its `path` (a `SKILL.md` file) and follow it,
   mapping the declared `capabilities` to the tool's own primitives (see below).

## Skill file format

Each skill lives in `skills/<name>/SKILL.md` and follows the open
[Agent Skills](https://www.anthropic.com/news/skills) convention: a Markdown
file with YAML frontmatter.

```markdown
---
name: task-init
description: One line the model reads to decide relevance.
version: 1.0.0
capabilities: [load_skill, spawn_subagent, run_shell, read_file, vcs]
---

<the procedure, in harness-neutral prose>
```

`name` and `description` are required and mirror `manifest.json`. The body is the
procedure. It is written against the **capability vocabulary** below instead of
any one tool's API, so the same file runs unchanged on any harness.

## Capability vocabulary

Skills never call a vendor tool by name (no `Agent(...)`, no `Bash(...)`). They
reference abstract capabilities. Each harness maps them to its own primitives;
the mapping for a given harness lives in `skills/adapters/<harness>/`.

| Capability | Meaning | Claude Code mapping |
|---|---|---|
| `load_skill` | Load/activate another skill by name | `Skill` tool |
| `spawn_subagent` | Start a sub-agent with a prompt; may run in background / in parallel | `Agent` tool (`run_in_background`) |
| `run_shell` | Run a shell command | `Bash` tool |
| `read_file` | Read a file from the repo | `Read` tool |
| `vcs` | Version-control ops (worktree, fetch, pull, branch) | `Bash` + `git` |

If a harness lacks a capability (e.g. no background sub-agents), its adapter
documents the fallback (e.g. run sub-agents sequentially).

## Adapters

Adapters expose these skills to a specific harness without forking the skill
bodies. See [`adapters/`](adapters):

- [`adapters/claude-code/`](adapters/claude-code) — symlinks each
  `skills/<name>` into `.claude/skills/<name>` so Claude Code discovers the
  `SKILL.md`, plus the capability mapping table.
- [`adapters/generic.md`](adapters/generic.md) — contract for any tool that
  reads `manifest.json` directly.

## Conventions assumed by the skills

The orchestration skill is **task-source agnostic** but ships with a default:
OpenSpec change proposals (`openspec/changes/<id>/`) are the source of truth for
what to work on. A team using Jira/Linear/GitHub Issues adapts the "read the
task" step — everything else (parallel planning, worktree-per-task, human review
gate) is unchanged.
