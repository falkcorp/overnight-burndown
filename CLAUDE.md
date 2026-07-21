<!-- file: CLAUDE.md -->
<!-- version: 1.1.0 -->
<!-- guid: b9c4d7e2-3a1f-4e85-8d62-7f0b5c9a3e17 -->
<!-- last-edited: 2026-07-21 -->

# overnight-burndown

Launchd-driven nightly automation that drains a queue of small, safe work items
across configured GitHub repos. Language: Go. Requires Go 1.25+.

## Coding Standards

Org-wide coding standards are in the `.standards/` git submodule (cloned from
`https://github.com/falkcorp/.github`).
Always clone with `git clone --recurse-submodules` so these are available.

Key files:
- **File headers (MANDATORY):** `.standards/instructions/file-headers.md`
- **Go rules:** `.standards/instructions/go.md`
- **Commit format:** `.standards/instructions/commit-messages.md`


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
