<windows-support>

<overview>

This fork of `umputun/revdiff` carries Windows + WezTerm support.
Upstream has explicitly declined to accept Windows patches, so these
changes live only here and are never pushed back to `umputun/revdiff`.

This document is the single source of truth for **what we changed,
where, and why** — so future maintainers (and Claude Code sessions)
can reason about Windows behavior without re-deriving it from the git
log, and so upstream merges stay tractable.

**Scope of Windows support**: Windows 10/11 with **WezTerm** as the
terminal. PowerShell 7+ (pwsh) for scripting. cmd.exe, Windows
Terminal, ConEmu, mintty, and WSL are **not** validated. Git Bash
(MSYS2) is used for running the Go build/test tooling but is not
required at runtime.

**Guiding principles**:
- Keep divergence from upstream small and localized.
- Prefer **new files** (additive divergence) over edits to
  upstream-tracked files (merge-conflict surface).
- When we must edit an upstream-tracked file, isolate the Windows
  branch behind `runtime.GOOS == "windows"` / `sys.platform == "win32"`
  / `$IsWindows` checks so the merge diff stays tiny.
- Document every divergence here.

</overview>

<fork-versioning>

<overview>

Plugin `version` fields in `.claude-plugin/plugin.json` and
`.claude-plugin/marketplace.json` follow SemVer 2.0.0 build-metadata
suffixes to signal fork divergence from upstream:

```
<upstream-version>+win.<N>
```

Examples: `0.6.0+win.1`, `0.6.0+win.2`, then `0.7.0+win.1` after
the next upstream catch-up.

</overview>

<rules>

1. **Pin to the upstream version we diverged from.** When we're in
   sync with upstream `0.6.0`, our fork versions are `0.6.0+win.N`.
2. **Bump `N` on every fork-only change** that touches plugin
   files (`.claude-plugin/...`, `plugins/.../scripts/*.ps1`, etc.).
   Start at `+win.1`; no gaps, no skips.
3. **Reset `N` to 1 after merging upstream.** When we pull
   upstream `0.7.0`, the next fork release becomes `0.7.0+win.1`,
   not `0.7.0+win.<old+1>`.
4. **Do NOT bump when merging upstream without fork edits.** A pure
   upstream sync should adopt upstream's version verbatim — the
   `+win.N` suffix is reserved for changes that originate in this
   fork.
5. **Per-plugin counters.** `revdiff` and `revdiff-planning` have
   independent `N` counters; bumping one does not bump the other.

</rules>

<rationale>

SemVer treats `+` as build metadata — ignored for version
precedence, so tooling still considers `0.6.0+win.1` equivalent to
upstream `0.6.0` for dependency resolution. Humans and marketplace
listings see the suffix, so "the fork has moved" is visible without
claiming a version number that collides with upstream's release
track.

</rationale>

</fork-versioning>

<new-files>

<overview>

Files that exist only in this fork. Adding new files never produces
merge conflicts when pulling upstream — these are pure additive
divergence.

</overview>

<build-scripts>

- `build.ps1` — PowerShell mirror of `make build`. Produces
  `.bin/revdiff.exe`. Added in commit `dcd6126` (Issue #3).
- `test.ps1` — PowerShell mirror of `make test`. Runs the Go test
  suite with race detector and coverage. Added in commit `eecf317`
  (Issue #3).

</build-scripts>

<tty-split>

<example>

- `cmd/revdiff/tty_unix.go` — POSIX TTY reattach logic (opens
  `/dev/tty`). Carries the build tag `//go:build !windows`.
- `cmd/revdiff/tty_windows.go` — Windows TTY reattach logic (opens
  `CONIN$` with `O_RDWR` so bubble tea can enter raw mode). Carries
  the build tag `//go:build windows`.

</example>

These replace the single-file TTY reattach that used to live inline
in `cmd/revdiff/main.go`. Added in commit `1315028` (Issue #2) and
further fixed in `ec4ec58` (`O_RDWR` flag — without it, bubble tea
cannot put the Windows console into raw mode and `--stdin` mode
breaks).

</tty-split>

<ps1-launchers>

<overview>

PowerShell companions to upstream's bash launchers. WezTerm-only.
These are sibling files — the bash launchers are untouched.

</overview>

- `.claude-plugin/skills/revdiff/scripts/detect-ref.ps1` — Windows
  sibling of `detect-ref.sh`. Detects the current git ref for the
  main revdiff plugin's SKILL.md guidance. Added in commit
  `4edfafa` (Issue #4).
- `.claude-plugin/skills/revdiff/scripts/launch-revdiff.ps1` —
  Windows sibling of `launch-revdiff.sh`. Spawns revdiff in a
  WezTerm split pane for the main revdiff plugin's "review diff"
  flow. Added in commit `658d21d` (Issue #4). This launcher also
  carries a **fork-only `--view=<path>` flag** (not present in the
  bash sibling) which pipes the named file into `revdiff --stdin
  --stdin-name=<basename>`, enabling plain-file annotation mode
  even for tracked-clean files that `--only` cannot render. Added
  after this file's initial landing.
- `plugins/revdiff-planning/scripts/launch-plan-review.ps1` —
  Windows sibling of `launch-plan-review.sh`. Spawns revdiff in a
  WezTerm split pane so the user can review a plan file during
  `ExitPlanMode` hook processing. Added in commit `cac9678`
  (Issue #5).

All three PS1 launchers hard-require WezTerm — they exit with an
error if `WEZTERM_PANE` is unset or `wezterm.exe` is not on PATH.

</ps1-launchers>

</new-files>

<modified-upstream-files>

<overview>

Files that existed in upstream and were edited to add Windows
branches. These are the **merge-conflict-prone** divergences.
Each one isolates the Windows behavior behind a platform check to
minimize the diff.

</overview>

<main-go>

<example>

`cmd/revdiff/main.go` — default config path resolution.

On Windows, `defaultConfigPath()`, `defaultKeysPath()`, and
`defaultThemesDir()` route through `os.UserConfigDir()` (which
returns `%APPDATA%`, typically `C:\Users\<you>\AppData\Roaming`)
instead of the POSIX `~/.config/revdiff/`. Unix paths are
unchanged.

Branching lives behind `runtime.GOOS == "windows"` checks. See
commit `bf9201c` (Issue #2).

</example>

Resulting paths on Windows:
- Config: `%APPDATA%\revdiff\config`
- Keybindings: `%APPDATA%\revdiff\keybindings`
- Themes: `%APPDATA%\revdiff\themes\`

</main-go>

<main-test>

`cmd/revdiff/main_test.go` — table-driven cross-platform path
tests added in commit `8ec888e` (Issue #2). Additional Windows-
friendly tweaks in `3c5a4f2` so the pre-existing test suite passes
when `go test` runs on Windows.

</main-test>

<ui-only-flag>

`ui/diffview.go` (and related) — `--only` absolute-path pattern
matching was normalizing on `/` separators only. Commit `fb71dfc`
normalizes path separators so Windows-style `C:\path\to\file.md`
patterns resolve correctly.

</ui-only-flag>

<plan-review-hook-py>

<example>

`plugins/revdiff-planning/scripts/plan-review-hook.py` — platform
dispatch.

On Windows, the hook invokes `launch-plan-review.ps1` via
`pwsh -NoProfile -ExecutionPolicy Bypass -File <ps1> <plan>`.
On other platforms, it invokes `launch-plan-review.sh` as before.

The dispatch is a single `if sys.platform == "win32":` branch
added in commit `1b8106e` (Issue #5).

</example>

</plan-review-hook-py>

<hooks-json>

<example>

`plugins/revdiff-planning/hooks/hooks.json` — one-word change.

Upstream:
```json
"command": "python3 ${CLAUDE_PLUGIN_ROOT}/scripts/plan-review-hook.py"
```

Our fork:
```json
"command": "py -3 ${CLAUDE_PLUGIN_ROOT}/scripts/plan-review-hook.py"
```

**Why**: On Windows with pyenv-win installed, `python3` resolves
to `python3.bat`, a batch shim that loses stdin forwarding when a
parent process pipes data (the PreToolUse hook pipes the tool-call
JSON to the hook on stdin). The `py -3` invocation uses the
Python Launcher for Windows (`py.exe`) directly, which is a native
executable with no batch shim and forwards stdin correctly.

`py -3` does not exist on Linux/macOS, so this is a true Windows-
only divergence. Fixed in commit `5f00130`.

</example>

</hooks-json>

<skill-md>

`.claude-plugin/skills/revdiff/SKILL.md` — platform dispatch for
the launcher section (commit `998ab7f`, Issue #4).

</skill-md>

<docs>

The following docs were updated to describe the Windows install
paths alongside the POSIX paths. No behavior changes, just
documentation sync:

- `README.md` (commit `cacffd8`, Issue #6)
- `CLAUDE.md` (commit `04f8273`, Issue #6)
- `site/docs.html` (commit `ccb45d7`, Issue #6)
- Plugin reference docs under
  `.claude-plugin/skills/revdiff/references/` (commit `c4161fc`,
  Issue #6)

</docs>

</modified-upstream-files>

<known-quirks>

<overview>

Behaviors we discovered the hard way while building the Windows
overlay. Document them here so we don't re-learn them.

</overview>

<pyenv-win-stdin>

<example>

**Symptom**: `ExitPlanMode` hook hangs forever; the Python hook
never reads the plan content from stdin.

**Root cause**: `python3` on Windows (with pyenv-win installed)
resolves to `python3.bat`, a batch shim that invokes
`cmd.exe /C call pyenv.bat exec python3 ...`. That cmd.exe chain
loses stdin forwarding when a parent process pipes data in.

**Fix**: Invoke Python via `py -3` instead of `python3`. The
Python Launcher for Windows (`py.exe`) is a direct executable, not
a batch shim, and forwards stdin correctly. Applied in
`plugins/revdiff-planning/hooks/hooks.json` (commit `5f00130`).

</example>

</pyenv-win-stdin>

<wezterm-prog-forwarding>

<example>

**Symptom**: `wezterm cli split-pane -- cmd.exe /c "..."` creates
a new split pane but cmd.exe starts in **interactive mode** — the
`/c "..."` arguments are silently dropped. The pane just shows a
fresh `C:\...\>` prompt and our intended command never runs.

**Consequence**: the sentinel file we rely on to signal completion
is never written, and the launcher's poll loop hangs forever.

**Root cause**: wezterm's `cli spawn` / `cli split-pane` PROG-
argument forwarding on Windows does not reliably pass multi-argv
commands through to the spawned process. Passing a single
executable/script as PROG with no extra args works fine; passing
`cmd.exe /c "complex string"` or `powershell.exe -Command "..."`
gets the args dropped.

**Fix**: Build a temp `.cmd` file containing the real commands
we want to run (with `@echo off`, the revdiff invocation, and the
`break > "<sentinel>"` touch), then pass the `.cmd` file path as
the **sole** PROG argument to `wezterm cli split-pane`. cmd.exe
runs the script and exits cleanly — no quoting hell, no arg
forwarding bugs. The `.cmd` file is cleaned up in the launcher's
`finally` block alongside the other temp files.

See `plugins/revdiff-planning/scripts/launch-plan-review.ps1` —
search for `$cmdScriptFile`.

</example>

</wezterm-prog-forwarding>

<wezterm-pane-visibility>

<example>

**Symptom**: The launcher reports success, `wezterm cli list`
shows a new pane was created, but the user sees no revdiff pane.

**Root cause**: `wezterm cli spawn --pane-id X` creates a new
**tab** in the window containing pane X. If the user is currently
viewing a different tab, they never notice the new tab appearing.

**Fix**: Use `wezterm cli split-pane --bottom --percent 90
--pane-id X` instead of `cli spawn`. `split-pane` divides pane X
**in place**, so the new pane is always visible within the tab
the user was already looking at — matching upstream bash's
behavior exactly (`launch-plan-review.sh` lines 73–96).

</example>

</wezterm-pane-visibility>

<msys-colon-in-revspec>

<example>

When running `git` commands from Git Bash on Windows, revspecs
containing a colon (e.g. `upstream/master:path/to/file.md`) get
mangled by MSYS path conversion — Git Bash interprets the colon
as a drive-letter separator.

**Fix**: Prefix the command with `MSYS_NO_PATHCONV=1` and quote
the revspec:
```bash
MSYS_NO_PATHCONV=1 git show 'upstream/master:plugins/revdiff-planning/hooks/hooks.json'
```

</example>

</msys-colon-in-revspec>

</known-quirks>

<merging-upstream>

<overview>

Upstream is pulled via the `upstream` remote:
```
upstream  https://github.com/umputun/revdiff.git (fetch)
upstream  https://github.com/umputun/revdiff.git (push)
```

We **never push** to upstream — the author has declined to accept
Windows support patches.

</overview>

<conflict-prone-files>

When merging or rebasing from upstream, watch for conflicts in the
files listed under `<modified-upstream-files>` above. The new
files under `<new-files>` are additive and should not conflict.

The most conflict-prone files today:

1. `cmd/revdiff/main.go` — config path resolution (Issue #2).
2. `cmd/revdiff/main_test.go` — table-driven cross-platform tests.
3. `plugins/revdiff-planning/scripts/plan-review-hook.py` —
   platform dispatch.
4. `plugins/revdiff-planning/hooks/hooks.json` — the `py -3`
   single-word change.
5. `.claude-plugin/skills/revdiff/SKILL.md` — platform dispatch.

</conflict-prone-files>

<resolution-strategy>

When upstream touches one of these files:

1. Prefer `git merge` over `git rebase` for complex upstream
   catch-ups — it preserves the history and makes conflicts easier
   to reason about.
2. On conflict, keep the Windows branch intact (`if runtime.GOOS
   == "windows"` / `if sys.platform == "win32"` / etc.) and replay
   upstream's changes in the non-Windows branch. Never drop the
   Windows branch to take upstream wholesale — that would
   reintroduce the bugs this document describes.
3. After resolving, run `test.ps1` (Windows) and `make test`
   (Linux/macOS if available) to verify both code paths still
   work.

</resolution-strategy>

<known-pending-work>

As of the most recent sync, this fork is behind upstream by a
significant margin — including a large restructure that moved
`ui/`, `theme/`, etc. into an `app/` subdirectory. A full catch-up
merge is deferred and tracked outside this file.

When that catch-up lands, expect all of the `<modified-upstream-files>`
entries to need rebasing into the new directory layout. The new
files under `<new-files>` and their relative paths may also need
to move.

</known-pending-work>

</merging-upstream>

</windows-support>
