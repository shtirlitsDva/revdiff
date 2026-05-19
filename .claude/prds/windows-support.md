---
name: windows-support
description: Make revdiff and its Claude Code plugins build, run, and integrate cleanly on Windows + WezTerm
status: backlog
created: 2026-04-07T07:36:29Z
---

# PRD: windows-support

## Executive Summary

revdiff is a Go bubbletea TUI for reviewing diffs and files, plus a Claude Code plugin that launches it from inside an agent session. Today the binary, the config layout, and the plugin's bash launchers all assume a POSIX environment. This PRD scopes the work needed so a Windows user running WezTerm can `go install` revdiff, run every feature (including `--stdin`), and use both the `revdiff` and `revdiff-planning` Claude Code plugins with no functional gaps.

The target environment is **Windows 10/11 with WezTerm as the only terminal**. We are *not* targeting cmd.exe, Windows Terminal, ConEmu, mintty, or WSL — those may incidentally work but are not validated. We are *not* shipping pre-built Windows binaries in this epic; users build from source via `go install` or the new `build.ps1`.

## Problem Statement

### What problem are we solving?

The repository owner uses Windows with WezTerm as a daily driver and currently cannot use revdiff or its plugin on that machine. The audit identified five categories of breakage:

1. **TTY reattach** — `cmd/revdiff/main.go:329` hardcodes `os.Open("/dev/tty")` so `--stdin` mode crashes immediately on Windows.
2. **Config paths** — `defaultConfigPath`, `defaultKeysPath`, and `defaultThemesDir` all build paths under `~/.config/revdiff`, which is not where Windows users expect app config to live.
3. **Build tooling** — the `Makefile` calls `cp`, `grep -v`, and writes the binary to `.bin/revdiff` with no `.exe` suffix; none of this works in PowerShell or cmd.exe.
4. **Plugin launchers** — `.claude-plugin/skills/revdiff/scripts/launch-revdiff.sh` and `plugins/revdiff-planning/scripts/launch-plan-review.sh` are bash scripts that use `mktemp /tmp/...`, `mkfifo`, `osascript`, and overlay tricks for tmux/kitty/wezterm/iTerm2 — none portable to Windows shells.
5. **Documentation** — README, site/docs.html, and the plugin reference docs only describe Unix install paths and Unix config locations.

### Why is this important now?

The author actively uses revdiff on macOS but switches to a Windows machine for other work. Without Windows support, the entire workflow (especially the Claude Code plugin which is the canonical entry point for diff review during agent sessions) is unavailable on half of the author's setups. The plugin v0.5.0 was just released (commit `b302fc9`), and adding Windows now — before more terminal-specific complexity accretes in `launch-revdiff.sh` — keeps the surface area small.

## User Stories

### Persona: Windows-based revdiff author/contributor

**Story 1 — Install and build from source.**
As a Windows developer with Go installed, I want `go install github.com/shtirlitsDva/revdiff/cmd/revdiff@latest` to produce a working `revdiff.exe` on my `PATH` so I can use the tool without hunting for binaries.

*Acceptance criteria:*
- `go install` succeeds with no `//go:build` errors and produces `revdiff.exe` in `%GOPATH%\bin` (or `%USERPROFILE%\go\bin`).
- Running `revdiff.exe --version` from any PowerShell prompt prints the embedded version string.
- `revdiff.exe --help` renders without garbled ANSI in WezTerm.

**Story 2 — Build the local repo on Windows.**
As a contributor working on the repo from a Windows checkout, I want a single command that mirrors `make build` so I can iterate without needing Git Bash/MinGW just to run the Makefile.

*Acceptance criteria:*
- Running `.\build.ps1` in PowerShell produces `.bin\revdiff.exe` and (on a tagged build) embeds the same version metadata `make build` does.
- Running `.\test.ps1` runs the same `go test` invocation as `make test` (race detector + coverage, mocks excluded) and exits non-zero on failure.
- The existing `Makefile` is untouched; macOS/Linux contributors see no change.

**Story 3 — Review a diff against HEAD in the local checkout.**
As a Windows user inside a git repo, I want to run `revdiff` (no args) inside WezTerm and get the same TUI experience macOS users get: file tree, diff pane, syntax highlighting, all key bindings, theme loading, blame gutter.

*Acceptance criteria:*
- `revdiff` invoked in a WezTerm tab inside a git repo opens the bubbletea TUI with no rendering artifacts.
- Theme directory `%APPDATA%\revdiff\themes\` is auto-created on first run with the five bundled themes, and `--theme dracula` (or any other bundled name) loads cleanly.
- `--dump-config`, `--dump-theme`, `--dump-keys`, `--list-themes`, and `--init-themes` all work and reference the `%APPDATA%\revdiff\` paths in their output.
- The blame gutter (`B` toggle) works because git is on `PATH`.
- `git rev-parse --show-toplevel`, `git diff`, `git ls-files -z`, and `git blame` all execute via the existing `exec.CommandContext("git", ...)` calls without modification.

**Story 4 — Review piped input via `--stdin`.**
As a Windows user, I want to pipe arbitrary text into revdiff (`Get-Content notes.md | revdiff --stdin`) and get the interactive viewer reattached to my keyboard, just like the Unix flow.

*Acceptance criteria:*
- `Get-Content file.txt | revdiff.exe --stdin` reads the piped payload, then re-attaches keyboard input via `CONIN$` and shows the TUI.
- All key bindings (q, j/k, /, n/N, etc.) work after reattach.
- Running `revdiff.exe --stdin` with no piped input prints the same "stdin must not be a TTY" error as on Unix and exits non-zero.

**Story 5 — Use the Claude Code plugin from a WezTerm-hosted Claude session.**
As a Windows user running Claude Code inside WezTerm, I want the `/revdiff` skill to spawn revdiff in a new WezTerm tab next to my Claude session, just like the kitty/wezterm overlay flow on macOS.

*Acceptance criteria:*
- Invoking the revdiff skill from a Claude Code session inside WezTerm on Windows opens a new WezTerm tab in the **same window** running revdiff against the appropriate ref.
- When revdiff exits, the new tab closes (or returns to a clean prompt) and any annotations are written back to the location the skill expects.
- The skill works whether the user invokes `/revdiff`, `/revdiff staged`, or `/revdiff <ref>`.
- The Unix skill flow on macOS/Linux is unchanged — no regressions in `launch-revdiff.sh`.

**Story 6 — Use the revdiff-planning plugin on Windows.**
As above for the `revdiff-planning` plugin: `/plan-review` (or whatever entry point the plugin exposes) spawns the planning review session in a new WezTerm tab on Windows.

*Acceptance criteria:*
- `launch-plan-review.ps1` is the Windows sibling of `launch-plan-review.sh` and behaves identically from the user's perspective.
- The plugin's SKILL.md (or the launching code) selects `.ps1` on Windows and `.sh` elsewhere.

## Functional Requirements

### FR-1 — Cross-platform TTY reattach
- Split `openTTY()` in `cmd/revdiff/main.go` into two build-tagged files:
  - `tty_unix.go` (`//go:build !windows`) calling `os.Open("/dev/tty")`.
  - `tty_windows.go` (`//go:build windows`) calling `os.Open("CONIN$")`.
- Caller signature unchanged. `prepareStdinMode()` and `validateStdinInput()` need no edits beyond build tag bookkeeping.
- New unit tests cover the Windows path under a Windows runner (or are gated off on non-Windows hosts).

### FR-2 — Windows-aware config/themes/keybindings paths
- `defaultConfigPath()`, `defaultKeysPath()`, and `defaultThemesDir()` consult `runtime.GOOS`. On Windows they return paths under `%APPDATA%\revdiff\` (resolved via `os.UserConfigDir()` which already returns `%APPDATA%` on Windows). On Unix they keep returning `~/.config/revdiff/` to avoid breaking existing installs.
- All three functions remain free of hardcoded separators (already use `filepath.Join`).
- Unit tests in `cmd/revdiff/main_test.go` cover both platforms via dependency injection or build tags.
- `--dump-config`, `--dump-keys`, `--dump-theme`, `--init-themes`, and `--list-themes` reflect the new locations on Windows automatically because they all read from these helpers.

### FR-3 — Windows build/test scripts
- Add `build.ps1` at repo root mirroring `make build`:
  - Runs `go build -trimpath -ldflags "-X main.version=<branch>"` from `cmd\revdiff`.
  - Outputs `.bin\revdiff.exe`.
  - Honors `$env:VERSION` override the same way the Makefile honors `$(BRANCH)`.
- Add `test.ps1` at repo root mirroring `make test`:
  - Runs `go test -race -coverprofile=coverage.out` over all packages excluding `*_mock.go` files.
  - Exits non-zero on failure.
- Neither script invokes external Unix tools (`grep`, `cp`, `chmod`).
- Existing `Makefile` is **not** modified.

### FR-4 — PowerShell launcher for the revdiff plugin
- Add `.claude-plugin/skills/revdiff/scripts/launch-revdiff.ps1` next to the existing `.sh`.
- Spawn revdiff via `wezterm cli spawn --new-tab -- revdiff <args...>` so the new pane is a tab in the current WezTerm window.
- Capture the new pane id from `wezterm cli spawn`'s stdout so the script can wait on the tab and clean up.
- Use a Windows temp directory (`$env:TEMP`) for any sentinel/output files instead of `/tmp`. Prefer `New-TemporaryFile` over hand-rolled paths.
- Use a sentinel file (polled via `Test-Path`/`Wait-Event`) instead of `mkfifo`, since named pipes on Windows are a different beast.
- Forward the same arguments the bash script forwards (ref, --staged, annotation file path, etc.).
- Return the same exit codes the bash script returns.
- Update `.claude-plugin/skills/revdiff/SKILL.md` so the script-selection logic picks `.ps1` when `$IsWindows` (or equivalent runtime check the skill harness provides) and `.sh` otherwise. Keep the bash path the default to avoid surprising existing users.

### FR-5 — PowerShell launcher for the revdiff-planning plugin
- Same treatment as FR-4 but for `plugins/revdiff-planning/scripts/launch-plan-review.ps1`.
- Mirror whatever flags/args the bash version takes today; do not invent new behavior.
- Update the planning plugin's SKILL.md (or equivalent loader) to dispatch on platform.

### FR-6 — Detection helper portable to Windows
- `.claude-plugin/skills/revdiff/scripts/detect-ref.sh` is a bash helper used by the launcher. Provide a `detect-ref.ps1` sibling that runs the same `git symbolic-ref` / `git show-ref` / `git status --porcelain` checks via PowerShell-friendly invocation (`& git ...` and `$LASTEXITCODE`).
- Output format must match the bash version exactly so callers can parse it identically.

### FR-7 — Documentation updates
- README.md: add a "Windows" subsection under Installation with `go install` instructions and a one-paragraph note that WezTerm is the supported terminal.
- README.md: in the Config section, add a callout that on Windows the paths are under `%APPDATA%\revdiff\` instead of `~/.config/revdiff/`.
- README.md: in the Plugin section, note that the plugin works on Windows under WezTerm.
- `site/docs.html`: mirror the README updates (CLAUDE.md flags this file as "must stay in sync with README.md").
- `site/index.html`: optional — add a small mention to the features grid only if it fits naturally; not a blocker.
- `.claude-plugin/skills/revdiff/references/install.md`: add Windows install steps.
- `.claude-plugin/skills/revdiff/references/config.md`: add Windows path mapping.
- `.claude-plugin/skills/revdiff/references/usage.md`: no changes expected — keys and flags are platform-neutral.
- `CLAUDE.md`: add Windows config path notes alongside the existing `~/.config/revdiff/` documentation so future agent sessions stay consistent.
- Plugin version bump: per CLAUDE.md "After any plugin file change, ask user if they want to bump the plugin version" — defer the bump decision to the end of the implementation epic, not this PRD.

### FR-8 — No regressions on macOS/Linux
- All existing tests still pass on Linux/macOS runners.
- The Makefile still works.
- `launch-revdiff.sh` and `launch-plan-review.sh` are unchanged in behavior; only the SKILL.md dispatch logic gains an `$IsWindows` branch.
- `~/.config/revdiff/` remains the config root on Unix.

## Non-Functional Requirements

### Reliability
- The Windows launcher must not leave orphan WezTerm tabs if revdiff crashes. Use the same trap/cleanup pattern as the bash script (PowerShell `try/finally` around the pane id).
- Sentinel and temp files must be cleaned up in success and failure paths.

### Compatibility
- Minimum Go version: whatever the existing `go.mod` requires today. No bump required.
- Minimum Windows version: Windows 10 1809+ (when ConPTY became stable). No special detection — bubbletea/lipgloss already handle ConPTY.
- WezTerm version: any release where `wezterm cli spawn --new-tab` is supported (≥ 20220319-142410-0fcdea07, i.e., spring 2022). No version pinning in our scripts; if the user has an ancient WezTerm they get a clear `wezterm` error.

### Maintainability
- All platform-specific Go code lives behind `//go:build` tags in dedicated files (`tty_windows.go`, `tty_unix.go`, possibly `paths_windows.go` / `paths_unix.go` if needed). No `runtime.GOOS` checks scattered through business logic.
- The PowerShell scripts read and feel like their bash counterparts so future edits to one are easy to mirror in the other. Add a comment block at the top of each `.ps1` linking to its `.sh` sibling.
- Tests for the new Go code use existing testing patterns (testify + table-driven). No new test framework.

### Performance
- No measurable performance impact expected — all changes are I/O setup or build tooling.
- The Windows TTY reattach goes through `os.Open("CONIN$")` which is constant-time.

### Security
- No new secrets, no new network calls.
- PowerShell scripts must use `Set-StrictMode -Version Latest` and proper quoting to avoid argument-injection issues when forwarding user-supplied refs/paths to `wezterm cli spawn`.

## Success Criteria

This work is done when **all** of the following are demonstrably true on a Windows 11 machine running WezTerm:

1. `go install github.com/shtirlitsDva/revdiff/cmd/revdiff@latest` produces a working `revdiff.exe`.
2. `.\build.ps1` produces `.bin\revdiff.exe` from a fresh checkout.
3. `.\test.ps1` runs the test suite green.
4. `revdiff` (no args) inside a git repo in a WezTerm tab opens the TUI and renders correctly.
5. `revdiff --dump-config` shows config paths under `%APPDATA%\revdiff\`.
6. `Get-Content README.md | revdiff --stdin` reads the file and reattaches the keyboard.
7. The Claude Code `revdiff` skill, invoked from a Claude session running inside WezTerm on Windows, spawns revdiff in a new tab in the same WezTerm window.
8. The Claude Code `revdiff-planning` skill works the same way.
9. macOS/Linux smoke test (`make build && make test && launch-revdiff.sh` from a kitty/wezterm session) still works with no regressions.
10. README and the plugin reference docs include Windows instructions.

## Constraints & Assumptions

### Constraints
- **Terminal target is WezTerm only.** No effort spent on cmd.exe rendering, Windows Terminal pane spawning, ConEmu, or mintty. If they happen to work, great; we don't validate or document them.
- **No CI/CD changes.** Adding `windows-latest` to GitHub Actions matrix and shipping signed `revdiff.exe` artifacts is explicitly out of scope for this epic — see "Out of Scope" below.
- **Bash launcher stays as-is.** We add PowerShell siblings, we do not rewrite `launch-revdiff.sh`. The two scripts must stay behaviorally aligned, but they're separate files maintained side by side.
- **No new Go runtime dependencies.** Use `runtime.GOOS`, `os.UserConfigDir`, build tags, and the standard library. Do not pull in `golang.org/x/term` or other helpers unless absolutely required.
- **Vendoring stays intact.** Run `go mod vendor` after any indirect dependency tweak.

### Assumptions
- The author has WezTerm installed on the Windows machine and `wezterm.exe` is on `PATH`.
- Git is on `PATH` on the Windows machine (same assumption as on Unix; revdiff already shells out to `git`).
- The Claude Code harness on Windows can execute `.ps1` scripts via `powershell.exe` or `pwsh.exe` without extra elevation.
- The author is comfortable running `Set-ExecutionPolicy -Scope CurrentUser RemoteSigned` once if needed; we don't need to ship a signed PowerShell module.
- `bubbletea v1.3.10` and `lipgloss v1.1.0` (already vendored) handle ConPTY correctly without further configuration. The audit confirmed these are recent enough.

## Out of Scope

The following are explicitly **not** part of this epic. They may become follow-up issues but won't block "windows-support" from closing.

- **Windows binary releases.** No GoReleaser changes, no signing, no chocolatey/winget packaging.
- **GitHub Actions Windows job.** No `windows-latest` runner added to the test matrix in this epic.
- **WSL support.** WSL is a Linux environment under the hood; if it works, it's because the Linux build works. No WSL-specific tooling.
- **Terminals other than WezTerm on Windows.** No detection or fallback for Windows Terminal, ConEmu, mintty, Hyper, Cmder, etc.
- **cmd.exe / batch (.bat) build scripts.** PowerShell only.
- **Cross-platform unification of launch-revdiff.sh + .ps1 into a Go binary.** Considered and rejected during planning — would expand scope and risk regressions on macOS.
- **XDG_CONFIG_HOME support on Linux.** A pre-existing gap unrelated to Windows; track separately.
- **Cygwin / MSYS2 / Git Bash support paths.** Not validated. The Unix scripts may work under those shells incidentally.
- **Replacing every `/tmp` reference in the bash scripts.** They stay; we only fix the PowerShell siblings.

## Dependencies

### External
- WezTerm (with `wezterm cli` subcommand) installed on the user's Windows machine.
- Git for Windows installed and on `PATH`.
- Go 1.x (matching `go.mod`) available for `go install` / `build.ps1`.
- PowerShell 5.1+ (preinstalled on Windows 10/11) or PowerShell 7.

### Internal
- The audit findings (in this conversation) covering every `/dev/tty`, `~/.config`, Makefile, and bash script touchpoint. The implementing engineer should re-read the audit before starting.
- CLAUDE.md's "must stay in sync with README.md" rule for `site/docs.html` — must be honored when documentation is updated.
- CLAUDE.md's "after any plugin file change, ask user if they want to bump the plugin version" — applies once both `.ps1` files land.

### Soft dependencies (not required, but worth knowing)
- `bubbletea v1.3.10` ConPTY behavior is undocumented in our codebase. If rendering anomalies appear on Windows, an upstream upgrade may be needed — but the audit found no concrete issues, so we proceed assuming the current version is sufficient and revisit only if validation fails.
