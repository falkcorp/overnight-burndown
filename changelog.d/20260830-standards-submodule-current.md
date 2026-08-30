### Fixed

#### `.standards` was pinned at the submodule's first commit, so none of the standards were reaching anyone

`CLAUDE.md` names `.standards/instructions/` authoritative and points
contributors and agents at it. A git submodule is a **pinned commit**, not a
live link, and this repo's pin had never moved off `664ae68` — the initial
2026-06-12 commit that created the submodule. Nine commits and 851 added lines
of standards had landed upstream since, read by no consumer here, because the
pin is what the repo actually checks out.

`instructions/go.md` alone gained 331 lines (it went from having no version
header at all to version 1.3.0). The rules that were not reaching anyone:

- the Go version policy — Go 1.26 as the stated minimum, 1.27 preferred where
  every dependency supports it
- the `io/ioutil` ban and the rest of the deprecated-stdlib replacement table
- the `wg.Go(fn)` rule — never `wg.Add(1)` plus `defer wg.Done()`
- the testing-isolation table (`t.Setenv`, `t.Chdir`, `t.TempDir`, `synctest`)
- `omitempty` versus `omitzero` under `encoding/json/v2`, where `omitempty`
  changes meaning and emits `false` and `0`

The last two are directly load-bearing for this repo, which moved to Go 1.27
today.

So this bumps the pin *and* removes the need to remember it: a `gitsubmodule`
entry in `.github/dependabot.yml` puts `.standards` on the same weekly
multi-ecosystem group as the Actions and Go dependencies. Falling behind now
opens a pull request instead of going quietly unnoticed for two and a half
months.

The bump carries no build risk. Every workflow, Makefile and script was grepped
for `.standards` before the change: nothing in CI reads it. The one code
reference, `internal/dispatch/worktree.go`, runs `git submodule update --init
--recursive` to populate worktrees — it never reads the submodule's contents and
is indifferent to which commit is pinned.
