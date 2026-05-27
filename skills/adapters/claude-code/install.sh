#!/usr/bin/env bash
# Links every harness-agnostic skill into .claude/skills/ so Claude Code can
# discover it. Idempotent — safe to re-run after adding a skill.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
skills_dir="$(cd "$script_dir/../.." && pwd)"
repo_root="$(cd "$skills_dir/.." && pwd)"
dest="$repo_root/.claude/skills"

mkdir -p "$dest"
for skill in "$skills_dir"/*/; do
	[ -f "${skill}SKILL.md" ] || continue
	name="$(basename "$skill")"
	ln -sfn "../../skills/$name" "$dest/$name"
	echo "linked skills/$name -> .claude/skills/$name"
done
