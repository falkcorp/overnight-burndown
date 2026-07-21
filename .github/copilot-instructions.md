<!-- file: .github/copilot-instructions.md -->
<!-- version: 1.1.0 -->
<!-- guid: a3f7e2d1-5c8b-4a90-b6e3-1d9f0c2e4a58 -->
<!-- last-edited: 2026-07-21 -->

# overnight-burndown — Additional Context

Org-wide coding standards (file headers, language rules, commit format) are at
**https://github.com/falkcorp/.github** and apply automatically to this repo.

For full project context: **CLAUDE.md** at the repo root.

## Project overview

Launchd-driven nightly automation that drains a queue of small, safe work items
across configured GitHub repos. Language: Go.

## Key directories

| Directory | Purpose |
|---|---|
| `cmd/burndown/` | Main entry point |
| `internal/agent/` | Triage and implementer agent logic |
| `internal/dispatch/` | Task dispatch and driver |
| `internal/triage/` | Triage classification |
| `internal/runner/` | Execution pipeline |
| `launchd/` | LaunchAgent plist |

## Build commands

```bash
make build           # Build binary to bin/burndown
make test            # Run tests
make ci              # vet + staticcheck + test + build (pre-PR gate)
make run             # Build and run
make dry-run         # Build and run in dry-run mode
make install-launchd # Install LaunchAgent
```


## 📝 Changelog & TODO — Use the Fragment System (MANDATORY)

**Do not hand-edit `CHANGELOG.md`, and do not add new tasks straight into the
`TODO.md` inbox.** Both files are assembled from per-change fragments so that
parallel PRs never collide on them.

- **`CHANGELOG.md` is assembled, not hand-edited.** Add a fragment under
  `changelog.d/` (run `scriv create`, or write the Markdown file by hand). The
  fragments are folded into `CHANGELOG.md` at release time by `scriv`, and a CI
  check (`changelog-check.yml`) requires one on each PR. See `changelog.d/README.md`.
- **New `TODO.md` tasks are added via fragments.** Drop a Markdown fragment in
  `todo.d/` (see `todo.d/README.md`) instead of editing the `## 📥 Inbox`
  section. `scripts/assemble_todo.py` folds fragments in daily. This is
  **add-only**: checking a task off or removing it is a normal direct edit of
  `TODO.md`.
