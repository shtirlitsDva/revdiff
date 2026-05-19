---
name: windows-support
status: backlog
created: 2026-04-07T07:44:04Z
updated: 2026-04-07T07:59:48Z
progress: 0%
prd: .claude/prds/windows-support.md
github: https://github.com/shtirlitsDva/revdiff/issues/1
---

# Epic: windows-support

## Overview

Make revdiff and both Claude Code plugins (`revdiff`, `revdiff-planning`) build, run, and integrate cleanly on Windows 10/11 + WezTerm. The work is intentionally narrow: five concrete Go/code touchpoints, two PowerShell sibling scripts per plugin, and a documentation pass. No CI changes, no second binary, no terminal abstraction layer — we sit Windows logic next to existing Unix logic via Go build tags and `.ps1` siblings to existing `.sh` scripts, so the macOS/Linux paths remain untouched and risk-free.

The audit (already in conversation context) confirmed that the heavy lifting — `filepath.Join` discipline, `exec.Command` style, modern bubbletea/lipgloss, `os.UserHomeDir`, `os.CreateTemp("")` — is already cross-platform. The remaining gaps are five surgical fixes plus PowerShell ports of the launcher scripts.

## Architecture Decisions

1. **Build tags over `runtime.GOOS` branching.** Platform-specific Go code (TTY reattach, possibly path defaults if cleaner) lives in `_windows.go` / `_unix.go` files behind `//go:build` constraints. Keeps business logic free of platform conditionals and lets the Go compiler enforce mutual exclusion. Rejected alternative: `if runtime.GOOS == "windows"` branches inline — works, but pollutes call sites and resists test isolation.

2. **`os.UserConfigDir()` rather than manual `%APPDATA%` resolution.** The standard library already returns `%APPDATA%` on Windows, `$XDG_CONFIG_HOME` (or `~/.config`) on Linux, and `~/Library/Application Support` on macOS. We currently hardcode `~/.config` even on macOS, which is non-idiomatic. To minimize blast radius, we keep the existing `~/.config/revdiff/` layout on **Unix** (Linux + macOS) and only switch to `os.UserConfigDir()` semantics **on Windows**, gated by `runtime.GOOS == "windows"`. This avoids breaking existing macOS users who already have configs at `~/.config/revdiff/`.

3. **PowerShell siblings rather than rewriting the bash launcher.** `launch-revdiff.sh` is 290+ lines of terminal-specific overlay code. Rewriting it as a Go binary or unifying it cross-platform was considered and rejected: too much risk to the existing macOS workflow, and a second binary complicates plugin distribution. Instead, `launch-revdiff.ps1` is a from-scratch PowerShell file containing only the WezTerm path. Bash and PowerShell scripts are maintained side-by-side and a header comment in each links to its sibling.

4. **`wezterm cli spawn --new-tab` for plugin hosting.** Already locked in via PRD brainstorming. The new tab lives in the user's existing WezTerm window (so Claude stays visible), and the script can wait on the spawned pane id for cleanup. No `wezterm start` (steals focus, separate window) and no `split-pane` (less consistent with the existing kitty/wezterm bash overlay flow).

5. **SKILL.md dispatches on `$IsWindows`.** PowerShell defines `$IsWindows` natively in v6+. The SKILL.md (or the small loader script the harness invokes) checks platform and selects `.ps1` on Windows, `.sh` elsewhere. The bash path stays the documented default so existing users see no change.

6. **No CI Windows runner.** Validation is manual on the author's Windows + WezTerm machine via the success criteria checklist. Adding `windows-latest` to GitHub Actions and shipping signed binaries are explicit follow-ups, not blockers.

7. **Makefile is untouched.** `build.ps1` and `test.ps1` are added in parallel. A Windows contributor uses the `.ps1` files; a Unix contributor keeps using `make`. Two scripts, one source of truth (the same `go build` / `go test` flags), maintained by convention.

8. **No new Go dependencies.** `golang.org/x/term` was considered for cross-platform TTY handling but rejected — `os.Open("CONIN$")` on Windows and `os.Open("/dev/tty")` on Unix is two lines of code each, doesn't justify pulling a new module. Vendoring stays clean.

## Technical Approach

### Frontend Components

revdiff is a TUI with no web frontend, but the user-facing surface that needs Windows attention is:

- **Terminal rendering pipeline** (bubbletea + lipgloss): no code changes required. Versions in `go.mod` (`bubbletea v1.3.10`, `lipgloss v1.1.0`, `x/ansi v0.11.6`) already drive ConPTY correctly on Windows 10 1809+. Validation only.
- **Plugin launchers** (PowerShell): the user-visible entry point for both plugins. New file per plugin, identical UX to the bash flow.
- **TUI key handling on stdin mode**: must reattach the keyboard via `CONIN$` after consuming piped stdin. Without this fix, every key press goes nowhere on Windows.

### Backend Services

revdiff has no network backend, but it shells out to `git`. Audit confirmed all `exec.Command("git", ...)` calls are list-form (not shell strings) and Go's exec package auto-resolves `git.exe` from `PATH` on Windows. No changes needed in `diff/diff.go`, `diff/directory.go`, `diff/blame.go`, or `cmd/revdiff/main.go:502`.

The Go-side changes are concentrated in **one file** (`cmd/revdiff/main.go`) plus two new build-tagged sidecars:

- `cmd/revdiff/main.go`: modify `defaultConfigPath`, `defaultKeysPath`, `defaultThemesDir` to consult `runtime.GOOS` and route Windows callers through `os.UserConfigDir()`. Replace direct `openTTY()` body with a call to a build-tagged `openInteractiveTTY()` helper.
- `cmd/revdiff/tty_unix.go` (`//go:build !windows`): `func openInteractiveTTY() (*os.File, error) { return os.Open("/dev/tty") }`
- `cmd/revdiff/tty_windows.go` (`//go:build windows`): `func openInteractiveTTY() (*os.File, error) { return os.Open("CONIN$") }`

`cmd/revdiff/main_test.go` gains a small table-driven test that fakes the Windows branch via dependency injection or a build-tagged sibling test file.

### Infrastructure

- **Build tooling**: `build.ps1` and `test.ps1` at repo root. Each is ~20-30 lines of PowerShell wrapping the same `go build` / `go test` invocations the Makefile uses.
- **Vendoring**: no changes; no new dependencies.
- **Plugin distribution**: existing `.claude-plugin/plugin.json` and `.claude-plugin/marketplace.json` already point to skill paths that don't depend on the file extension, so adding `.ps1` files alongside `.sh` files requires no manifest changes. A version bump to plugin.json/marketplace.json is deferred to the end of implementation per CLAUDE.md.

## Implementation Strategy

### Phasing

**Phase 1 — Go core (parallel-friendly):** Land the TTY split and config-path Windows-awareness first. Once `revdiff.exe` builds and produces the right paths via `--dump-config`, every other piece of work has something concrete to test against.

**Phase 2 — Build scripts and plugin launchers (parallel after Phase 1):** `build.ps1`/`test.ps1`, the `launch-revdiff.ps1` + `detect-ref.ps1` pair, and `launch-plan-review.ps1` can all proceed in parallel because they touch disjoint files. Each gets validated independently against the `revdiff.exe` from Phase 1.

**Phase 3 — Documentation (parallel with Phase 2):** README, site/docs.html, CLAUDE.md, and plugin reference docs can be drafted as soon as Phase 1 design is locked in. They reference paths and commands that are stable from the PRD.

**Phase 4 — End-to-end validation (sequential, last):** Walk every PRD success criterion on the actual Windows + WezTerm machine. Fix any gaps surfaced. Bump plugin version once.

### Risk mitigation

- **Risk: bubbletea ConPTY rendering anomalies.** Mitigation: validate early in Phase 1 with a smoke test (`revdiff --help` then a real `revdiff` against the repo). If anomalies appear, the upstream `bubbletea` GitHub issues are the first stop, not a local fix.
- **Risk: `wezterm cli spawn` exit semantics differ from Unix overlay flow.** Mitigation: the PowerShell launcher uses the same pane-id-wait pattern as the bash script; if cleanup behavior differs, a `try/finally` block guarantees we don't leave orphan tabs.
- **Risk: macOS/Linux regression from SKILL.md dispatch logic.** Mitigation: Unix path is unchanged (the dispatch is a simple `if ($IsWindows)` guard at the top of the loader). A Phase 4 smoke test on macOS confirms zero regression.
- **Risk: Windows test runs fail due to git config (line endings, autocrlf).** Mitigation: validation step verifies `core.autocrlf` doesn't perturb existing diff parsing. If it does, document the required `git config` for Windows contributors rather than coding around it.

### Test strategy

- Unit tests for the new path defaults, gated by build tags so each platform tests its own branch.
- Unit test for `openInteractiveTTY()` is light-touch — we can't actually open `CONIN$` in CI without a console, so we test the symbol resolution and gate the smoke test to manual validation.
- All existing tests (`make test` on Unix, `.\test.ps1` on Windows) must stay green throughout.
- End-to-end manual validation is the canonical acceptance gate per PRD success criteria #1–#10.

## Task Breakdown Preview

Six tasks total (under the ≤10 cap). Tasks 2/3/4/5 can run in parallel after task 1. Task 6 is the final gate.

- [ ] **001 — Go core: Windows config paths + TTY reattach.** Modify `cmd/revdiff/main.go` (`defaultConfigPath`, `defaultKeysPath`, `defaultThemesDir`, `openTTY` callers). Add `tty_unix.go` and `tty_windows.go` build-tagged sidecars. Update `cmd/revdiff/main_test.go` for both branches. Single PR; conflicts with nothing.
- [ ] **002 — PowerShell build/test scripts.** Add `build.ps1` and `test.ps1` at repo root mirroring `make build` / `make test`. Output to `.bin\revdiff.exe`. No Makefile changes.
- [ ] **003 — revdiff plugin Windows launcher.** Add `.claude-plugin/skills/revdiff/scripts/launch-revdiff.ps1` and `.claude-plugin/skills/revdiff/scripts/detect-ref.ps1`. Update `.claude-plugin/skills/revdiff/SKILL.md` to dispatch on `$IsWindows`. WezTerm spawn via `wezterm cli spawn --new-tab`. Behavior parity with `launch-revdiff.sh`.
- [ ] **004 — revdiff-planning plugin Windows launcher.** Add `plugins/revdiff-planning/scripts/launch-plan-review.ps1`. Update the planning plugin's SKILL.md dispatch. Same WezTerm pattern as task 003.
- [ ] **005 — Documentation pass.** Update `README.md` (Installation + Config + Plugin sections), `site/docs.html` (in sync with README), `CLAUDE.md` (Windows path notes), `.claude-plugin/skills/revdiff/references/install.md` and `.../config.md`. No code changes.
- [ ] **006 — End-to-end Windows validation + plugin version bump.** Walk all 10 PRD success criteria on Windows 11 + WezTerm. Fix any gaps. Run macOS/Linux smoke test for regressions. Bump `plugin.json` and `marketplace.json` to next minor.

### Parallelization map

```
                                    ┌── 002 (build.ps1/test.ps1) ──┐
                                    │                              │
001 (Go core: paths + TTY) ─────────┼── 003 (revdiff launcher) ────┼── 006 (validation)
                                    │                              │
                                    ├── 004 (planning launcher) ───┤
                                    │                              │
                                    └── 005 (docs) ────────────────┘
```

Tasks 002/003/004/005 share no files and can be assigned to four parallel agents. Task 006 is sequential and gates the epic.

## Dependencies

### External
- WezTerm with `wezterm cli` subcommand on the validator's PATH (for tasks 003, 004, 006).
- Git for Windows on PATH (for task 006 validation).
- Go matching `go.mod` (for tasks 001, 002, 006).
- PowerShell 5.1+ (preinstalled on Windows 10/11) — `$IsWindows` requires PowerShell 6+, so for Windows PowerShell 5.1 we fall back to `[System.Environment]::OSVersion.Platform` checks. Confirm during task 003 implementation.

### Internal
- Audit findings from this conversation — implementing engineer must re-read before starting task 001.
- CLAUDE.md `site/docs.html` sync rule (task 005).
- CLAUDE.md plugin version bump rule (task 006).
- The PRD's "no regressions on macOS/Linux" requirement governs task 006.

### No blocking dependencies
- All dependencies are tooling/runtime concerns, not other epics. This work can begin as soon as the epic syncs to GitHub.

## Success Criteria (Technical)

1. **Builds clean on Windows.** `go install github.com/shtirlitsDva/revdiff/cmd/revdiff@latest` and `.\build.ps1` both produce a runnable `revdiff.exe`. No `//go:build` errors. `.\test.ps1` exits 0.
2. **Builds clean on Unix.** `make build && make test` on macOS and Linux exits 0 with no behavior change vs. master. The `~/.config/revdiff/` layout still works.
3. **Config paths route correctly.** On Windows, `revdiff.exe --dump-config` shows paths under `%APPDATA%\revdiff\`. On Unix, `revdiff --dump-config` shows `~/.config/revdiff/`. Both cases auto-create the themes directory with the five bundled themes on first run.
4. **TTY reattach works on Windows.** `Get-Content README.md | revdiff --stdin` reads input, reattaches keyboard via `CONIN$`, and accepts `q` / `j` / `k` / `/` interactions.
5. **Plugin launcher works on Windows.** Invoking the `revdiff` skill from a Claude Code session running inside a WezTerm tab spawns revdiff in a new tab in the same WezTerm window. Tab cleans up on revdiff exit.
6. **Planning plugin launcher works on Windows.** Same as #5 for `revdiff-planning`.
7. **Plugin works on Unix unchanged.** `launch-revdiff.sh` and `launch-plan-review.sh` are byte-identical to master. SKILL.md dispatch logic only adds an `$IsWindows` branch.
8. **Documentation reflects Windows.** README, site/docs.html, CLAUDE.md, and plugin reference docs all describe the Windows install + config layout. The site/docs.html ↔ README sync rule is satisfied.
9. **No new Go dependencies.** `go.mod` and `vendor/` diff against master shows no additions.
10. **Plugin version bumped once.** `plugin.json` and `marketplace.json` move to the next minor version exactly once at the end of implementation.

## Estimated Effort

Effort is described in scope, not time, per project conventions.

- **001 (Go core)**: small. ~50 lines of Go across 3 files + ~30 lines of test. Single contributor, single PR.
- **002 (PowerShell build/test)**: small. ~60 lines of PowerShell across 2 files. Single PR.
- **003 (revdiff launcher)**: medium. ~150 lines of PowerShell + a 5-line SKILL.md edit. The bash original is 290+ lines but most of that is non-WezTerm terminal handlers we don't need; the WezTerm path is the small slice we're porting.
- **004 (planning launcher)**: medium-small. Mirrors 003 but the bash original is shorter (~335 lines, similarly trimmed to just the WezTerm path).
- **005 (documentation)**: small. Targeted edits to ~6 markdown/HTML files. No new pages.
- **006 (validation)**: small in code, medium in time. Walk all 10 success criteria, fix any gaps, bump version. Validation work is hard to estimate because it depends on whether anything surfaced in the wild.

**Total scope**: 6 tasks, four of which can parallelize after task 001 lands. The biggest risk is ConPTY rendering surprises in bubbletea — flagged in mitigation above.

**Critical path**: 001 → (003 ∥ 004) → 006. Tasks 002 and 005 are off the critical path and can land any time after 001.

## Tasks Created
- [ ] #2 - Go core: Windows config paths and TTY reattach (parallel: true)
- [ ] #3 - PowerShell build and test scripts (parallel: true)
- [ ] #4 - revdiff plugin Windows launcher (PowerShell + WezTerm) (parallel: true)
- [ ] #5 - revdiff-planning plugin Windows launcher (PowerShell + WezTerm) (parallel: true)
- [ ] #6 - Documentation pass for Windows support (parallel: true)
- [ ] #7 - End-to-end Windows validation and plugin version bump (parallel: false, depends on #2 #3 #4 #5 #6)

Total tasks: 6
Parallel tasks: 5 (#2, #3, #4, #5, #6)
Sequential tasks: 1 (#7 — depends on all five above)
Estimated total scope: 4×S + 2×M (one M is validation, not code)
