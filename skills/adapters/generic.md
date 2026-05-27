# Generic adapter (any tool)

For a harness that does not have a native skills directory, integrate by reading
the manifest directly — no vendor convention required.

## Contract

1. **Discover.** Read `skills/manifest.json`. It lists every skill with `name`,
   `description`, `path` (relative to `skills/`), `version`, and `capabilities`.
2. **Surface.** Expose each `name` + `description` to the model so it can decide
   when a skill is relevant (e.g. inject them into the system prompt, or offer
   them as selectable tools).
3. **Load.** When a skill is selected, read `skills/<path>` (a `SKILL.md`) and
   feed its body to the model as the procedure to follow.
4. **Map capabilities.** Bind the capabilities the skill declares to your tool's
   primitives:

   | Capability | Provide |
   |---|---|
   | `load_skill` | a way to load another skill by name (repeat step 3) |
   | `spawn_subagent` | start a sub-agent with a prompt; ideally background/parallel |
   | `run_shell` | run a shell command |
   | `read_file` | read a repo file |
   | `vcs` | run `git` (worktree, fetch, pull, branch) |

   If you lack `spawn_subagent` in the background, run sub-agents sequentially —
   the skill's outcome (N plans for review) is the same, only slower.

## Minimal discovery snippet

```python
import json, pathlib

manifest = json.loads(pathlib.Path("skills/manifest.json").read_text())
for s in manifest["skills"]:
    body = pathlib.Path("skills", s["path"]).read_text()
    register_skill(name=s["name"], description=s["description"], body=body)
```
