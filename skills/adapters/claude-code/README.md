# Claude Code adapter

Claude Code discovers skills in `.claude/skills/<name>/SKILL.md`. This adapter
links the harness-agnostic `skills/<name>` directories into `.claude/skills/`
without forking the skill bodies, so updates to `skills/` are picked up
automatically.

## Install

```bash
skills/adapters/claude-code/install.sh
```

It symlinks every `skills/<name>` that contains a `SKILL.md` into
`.claude/skills/<name>` (relative symlinks, safe to commit). Re-run after adding
a skill.

## Capability mapping

The skill bodies reference abstract capabilities; in Claude Code they map to:

| Capability | Claude Code primitive |
|---|---|
| `load_skill` | the `Skill` tool (`Skill(skill: "<name>")`) |
| `spawn_subagent` | the `Agent` tool, `run_in_background: true` for parallel/background |
| `run_shell` | the `Bash` tool |
| `read_file` | the `Read` tool |
| `vcs` | the `Bash` tool with `git` |

Claude Code supports background sub-agents, so the `task-init` parallel-fan-out
runs as written — multiple `Agent` calls in one turn.
