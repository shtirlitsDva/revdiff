<todo-compare-mode>

<context>
The fork's review procedure says: when presenting >20 lines of text or >1 question, route it through `/revdiff:revdiff`. That works on the first pass via `--view=<path>` (file shown as context-only, no `+`/`-` markers). It breaks on round 2+: the user has to re-read the whole document to find what the agent changed in response to their last batch of annotations.

Long docs become exhausting to review iteratively; small revisions get missed.

Today's workaround (now codified in `.claude-plugin/skills/revdiff/SKILL.md` under `<followup-revisions-must-use-diff>`): commit v1 in git, save v2 in place, re-launch revdiff with no ref so the working-tree-vs-HEAD diff shows the changes. Works, but requires a repo and a throwaway commit for free-floating docs.
</context>

<upstream-already-solved-this>
Upstream `umputun/revdiff` shipped the exact missing primitive in **v1.0.0** (2026-05-02), PR [#163](https://github.com/umputun/revdiff/pull/163) by @rashpile.

<flag-shape>
```
revdiff --compare-old=<path-to-v1> --compare-new=<path-to-v2>
```

- Two separate flags, used together.
- No VCS repository required. Only `git` on `$PATH` (uses `git diff --no-index` under the hood).
- Output flows through the same `parseUnifiedDiff` parser every other diff source uses → all existing features work automatically: word-diff, compact mode, syntax highlighting, scrollbar, inline annotations, line-count dividers.
- Exit code 1 from `git diff` (files differ) is treated as success; exit 1 with empty stdout is a real failure.
- Mutually exclusive with: refs, `--staged`, `--only`, `--all-files`, `--stdin`, `--include`, `--exclude`, `--annotations`.
</flag-shape>

<pr-motivation-verbatim>
The PR description names this exact use case:

> "revdiff is built for rolling review loops with a Claude agent: the agent produces content, the user annotates lines, the agent revises, the loop repeats until the user quits without annotations. … The same loop falls apart for agent-rewritten documents — plans, design docs, specs, generated reports. Today the agent can only re-open the rewritten file as context-only (`--only`), so the user has to re-read the whole document to find what changed in response to their last round of annotations."
</pr-motivation-verbatim>

<also-shipped-in-1-0-0>
A `revdiff-planning` plugin that automates the snapshot loop via an `ExitPlanMode` hook. Snapshots stored in `$TMPDIR` keyed by an HTML-comment marker the agent prepends to its plan (`<!-- previous revision: /tmp/plan-rev-AAA.md -->`). Marker validation closes a confused-deputy primitive: only paths under `$TMPDIR` matching `plan-rev-*` are accepted. Claude-only (codex has no hook system).
</also-shipped-in-1-0-0>
</upstream-already-solved-this>

<fork-status>
- Fork version: `0.6.0+win.6` (just bumped to `0.6.0+win.7` for the followup-revisions rule).
- Upstream version: `v1.3.0` (2026-05-13).
- Missing from fork: every release from `v0.7.0` through `v1.3.0`.
- `--compare-old` / `--compare-new` landed in `v1.0.0` → the fork does NOT have this flag today.
- The Windows launcher (`launch-revdiff.ps1`) is fork-specific and does not know about `--compare-old` / `--compare-new`.
</fork-status>

<implementation-options>

<option-1-merge-upstream>
**Merge upstream master into the fork.**

- Brings compare-mode plus everything else from v0.7→v1.3.
- Largest blast radius. Likely heavy conflicts with the Windows-only changes (launcher rewrites, `--view=` flag, encoding fixes, non-ASCII path handling, `[revdiff:STARTED]` sentinel emission).
- Long-term right answer if the fork wants to stay close to upstream.
- Will need to re-apply Windows-only commits on top.
</option-1-merge-upstream>

<option-2-cherry-pick-only-pr-163>
**Cherry-pick just PR #163.**

Files changed in #163 (~700 lines total):
- `app/diff/compare.go` (new, +121)
- `app/diff/compare_test.go` (new, +358)
- `app/compare.go` (new, +64)
- `app/compare_test.go` (new, +123)
- `app/config.go` (+12) — `--compare-old` / `--compare-new` flag definitions + `validateCompareFlag`
- `app/main.go` (+19 -7) — wires compare mode before stdin/VCS dispatch
- `app/reviewinfo.go` (+1) — info overlay header
- `app/ui/...` (a few +1's)
- Tests + docs

Risk: PR may depend on intermediate refactors (the fork is 0.6.0, PR landed at 1.0.0, so there's a lot of churn between them — `app/` directory structure, `Renderer` interface shape, etc. may have shifted). Cherry-pick may not apply cleanly.

Path layout in the upstream PR uses `app/` (e.g. `app/diff/compare.go`); fork uses `diff/` and `cmd/revdiff/`. Will need adaptation regardless.
</option-2-cherry-pick-only-pr-163>

<option-3-upstream-binary-fork-launcher>
**Install upstream `revdiff.exe`, teach the Windows launcher about the new flags.**

```bash
go install github.com/umputun/revdiff/cmd/revdiff@latest
```

- Zero Go merging.
- Launcher already passes through arbitrary args; only needs to learn that `--compare-old=<path>` and `--compare-new=<path>` are file-bearing flags that should be validated and forwarded (and probably should NOT be turned into `--view=` style stdin redirects).
- Risk: lose any binary-level fork changes that aren't yet upstreamed. Worth auditing the fork's `diff/`, `ui/`, `theme/`, `keymap/` changes first.
- Probably the lowest-effort path if the fork's local Go changes are minimal.
</option-3-upstream-binary-fork-launcher>

<option-4-reimplement-locally>
**Add `--compare-old` / `--compare-new` directly to the fork without merging.**

Implementation is small (~200 lines of real code, mostly a `CompareReader` that shells out to `git diff --no-index` and feeds stdout to `ParseUnifiedDiff` — which the fork already has).

Skip if you'd rather not maintain divergent code.
</option-4-reimplement-locally>

</implementation-options>

<once-compare-mode-is-available>
Update `.claude-plugin/skills/revdiff/SKILL.md`:

1. Replace the current `<followup-revisions-must-use-diff>` mechanism (commit v1 in git → save v2 → re-launch with no ref) with the simpler `--compare-old`/`--compare-new` mechanism:
   - On first presentation: write the doc, launch revdiff with `--view=<path>`. KEEP a copy of v1 at a snapshot path (e.g. `$env:TEMP\revdiff-snap-<id>.md`).
   - On followup: overwrite the target with v2, launch revdiff with `--compare-old=<snapshot> --compare-new=<target>`.
2. Update the launcher to recognize `--compare-old` / `--compare-new` and pass them through (no `--view=` rewriting).
3. Bump plugin version again (currently `0.6.0+win.7`).
4. Consider adopting the upstream `revdiff-planning` ExitPlanMode hook pattern if it fits the Windows workflow.

Also worth a separate look:
- Upstream `--annotations` flag (preloads annotations from FormatOutput markdown) — could simplify loop state.
- Upstream `}`/`{` keys for cross-file annotation navigation.
- Upstream review history auto-save on quit.
</once-compare-mode-is-available>

<links>
- PR #163: https://github.com/umputun/revdiff/pull/163
- v1.0.0 release: https://github.com/umputun/revdiff/releases/tag/v1.0.0
- Upstream README compare-mode section: https://github.com/umputun/revdiff#two-file-comparison
</links>

</todo-compare-mode>