---
name: devpt-release
description: Increment version and update CHANGELOG.md from commits since last update. Use when making a release, bumping version, or updating changelog for dev-process-tracker.
---

# DevPT Release Skill

## Usage

```
<user asks: "make a patch release"> or "bump minor version" or "devpt release major"
```

## Workflow

1. **Read CHANGELOG.md** — extract current version from first `## X.Y.Z` header
2. **Find last update** — get SHA of the commit that last modified CHANGELOG.md
3. **Get commits since** — `git log <SHA>..HEAD --oneline --no-merges`
4. **Group & classify**:
   - Parse commit messages for intent (add/fix/change/remove/refactor/docs)
   - **Group related commits**: if a "fix" or "polish" follows a feature in time/subject, fold it into that feature line
   - Prioritize user-facing changes over internal polish
5. **Determine bump**:
   - `major` (0.x → 1.0 or breaking) / `minor` (features) / `patch` (fixes) — use user-specified if provided
6. **Generate entries** — write concise imperative-mood bullets:
   - "Added X so Y" for features
   - "Fixed Z so W" for bugs
   - Group related fixes with their feature when they're clearly connected
7. **Update CHANGELOG.md** — prepend new version section
8. **Set version** — run `./scripts/set-version.sh <X.Y.Z>` to update version.go, commit, and tag
9. **Push** — `git push && git push origin v<X.Y.Z>`

## Version Management

- **Version file**: `pkg/buildinfo/version.go` (`const Version = "X.Y.Z"`)
- **Set version script**: `./scripts/set-version.sh <X.Y.Z>` — updates version.go, commits, creates tag
- **Tags use `v` prefix**: `v0.2.1`
- **Pre-push hook**: validates version.go matches latest tag (via lefthook)

## Grouping Heuristics

When classifying commits, apply these rules:

1. **Time proximity**: Fixes within 1-3 commits of a feature likely belong to it
2. **Subject overlap**: "fix search" after "add search input" → same entry
3. **Keyword clues**: "polish", "tweak", "adjust", "follow-up" often indicate related work
4. **When uncertain**: Keep separate rather than over-grouping

## Flags

- `--review` — show grouped commits and proposed entries before writing
- `--dry-run` — output the new section without modifying the file

## Example Output

```markdown
## 0.3.0

- Added dark mode toggle so users can switch themes without reloading
- Fixed theme persistence so preference survives across sessions
- Removed deprecated `/legacy` endpoint
```

## Edge Cases

- **No commits since last update**: Report "no changes since last release" and exit
- **Uncommitted changes**: Warn but proceed (commits are the source of truth)
- **Version is 0.x**: Treat as pre-release; minor bumps for features, patch for fixes
