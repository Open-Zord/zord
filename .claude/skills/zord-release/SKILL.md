---
name: zord-release
description: Cut a Zord release — confirm main is clean and the change is merged, tag vX.Y.Z on the merged commit, push it, watch the GoReleaser workflow, and verify the published artifacts and `go install`. Use when the user says "cut a release", "release vX.Y.Z", "tag a version", or "publish". Tagging a public release is outward-facing — confirm the version before pushing.
version: 1.0.0
capabilities: [run_shell, read_file, vcs, load_skill]
---

Cuts a release the way it actually has to happen: the tag lands on a **merged
main** commit (the `release.yml` workflow only exists there — tag earlier and
nothing fires), GoReleaser cross-compiles the matrix and publishes archives +
checksums, and you confirm `go install` works end-to-end.

Pushing a tag publishes a **public GitHub Release** — an outward-facing action.
**Confirm the exact version with the user before pushing the tag** unless they
already named it.

## 1. Pre-flight

```sh
git checkout main && git pull --ff-only
git status --porcelain            # MUST be empty (clean tree)
ls .github/workflows/release.yml .goreleaser.yaml   # both must exist on main
```

If you're releasing a specific merged PR, confirm it landed on this main:

```sh
gh pr view <N> --json state,mergeCommit -q '.state + " " + .mergeCommit.oid'
git log --oneline -1             # HEAD should be at/after that merge commit
```

Sanity-check the code is green before tagging — run **`/zord-verify`** (tiers 1–2
are enough; CI already ran the full battery on the merge).

## 2. Choose the version

Semver `vX.Y.Z`. Ask the user if not specified. Make sure it doesn't exist:

```sh
git tag | grep -x vX.Y.Z && echo "ALREADY EXISTS — pick another" || echo "ok"
```

GoReleaser strips the leading `v` for the archive/version string (tag `v1.2.0`
→ `zord_1.2.0_<os>_<arch>`), and injects it into `zord --version` via ldflags.

## 3. Tag and push

```sh
git tag -a vX.Y.Z -m "vX.Y.Z — <one-line summary>"
git push origin vX.Y.Z
```

## 4. Watch the release workflow

The tag push triggers `release.yml`. Find the run and poll it to completion in the
background (one notification when it finishes), covering success **and** failure:

```sh
gh run list --workflow=release.yml --limit 1 --json databaseId,status,headBranch
```

Then, with that `databaseId` as `$rid`, run this with `run_in_background: true`:

```sh
while :; do
  st=$(gh run view $rid --json status,conclusion -q '"\(.status) \(.conclusion)"' 2>/dev/null || echo "err err")
  set -- $st
  [ "$1" = "completed" ] && { echo "release run $rid: $2"; exit 0; }
  sleep 25
done
```

If the conclusion is not `success`, inspect it (`gh run view $rid --log-failed`)
and report — do not claim the release shipped.

## 5. Verify the published release

```sh
gh release view vX.Y.Z --json url,isDraft,isPrerelease,assets \
  -q '.url, "draft=\(.isDraft) pre=\(.isPrerelease)", (.assets[].name)'
```

Expect **6 archives** (linux/darwin/windows × amd64/arm64; `.tar.gz`, `.zip` on
Windows) plus `checksums.txt`, and `draft=false pre=false`.

Smoke-test the published module:

```sh
BIN=$(mktemp -d)
GOBIN="$BIN" GOFLAGS=-mod=mod go install github.com/Open-Zord/zord/cmd/zord@vX.Y.Z
"$BIN/zord" --help && rm -rf "$BIN"
```

> **Known nuance:** via `go install`, `zord --version` prints `dev` (go install
> can't inject ldflags). The **pre-built release binaries** report the real
> version. Flag this if the user expects the version from a `go install` build.

## 6. Report

Give the release URL, the conclusion of the workflow run, and the confirmed asset
list. If anything failed, say exactly what and stop — a half-published release is
worse than a clear failure.
