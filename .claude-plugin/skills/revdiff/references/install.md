# Installation

This is the Windows-only fork of `umputun/revdiff`. Supported environment: **Windows 10/11** with **WezTerm** as the terminal, **PowerShell 7+ (pwsh)** for scripting. cmd.exe, Windows Terminal, ConEmu, mintty, and WSL are not validated.

## Install the binary

**Go install (recommended):**

```powershell
go install github.com/shtirlitsDva/revdiff/app@latest
```

This produces `revdiff.exe` in your Go bin directory (`$env:GOPATH\bin`, or `$env:USERPROFILE\go\bin` if `GOPATH` is unset). Make sure that directory is on `$env:PATH`.

**Build from source:**

```powershell
git clone https://github.com/shtirlitsDva/revdiff
cd revdiff
.\build.ps1
```

The `build.ps1` script is the PowerShell mirror of `make build` and produces `.\.bin\revdiff.exe`.

Verify the install:

```powershell
Get-Command revdiff
revdiff --version
```

## Claude Code Plugin

```
/plugin marketplace add shtirlitsDva/revdiff
/plugin install revdiff@revdiff
```

Use: `/revdiff [base] [against]` — opens the review session in a WezTerm split pane next to the pane running Claude Code.

The plugin's launcher requires WezTerm. `wezterm.exe` must be on `$env:PATH`, and the Claude Code session must itself run inside a WezTerm pane (the launcher reads `$env:WEZTERM_PANE` to anchor the split).

### Plan Review Plugin

Automatically opens revdiff when Claude exits plan mode for interactive annotation:

```
/plugin install revdiff-planning@revdiff
```

### Launcher scripts

The `revdiff` skill ships with these scripts under `.claude-plugin/skills/revdiff/scripts/`:

| Script | Purpose |
|---|---|
| `launch-revdiff.cmd` | **Preferred entry point.** Thin cmd.exe shim that forwards args via `pwsh -Command` to `launch-revdiff.ps1`. Always call this instead of the `.ps1` directly. |
| `launch-revdiff.ps1` | Spawns revdiff in a WezTerm split pane, captures the annotation output file, and prints annotations to stdout. Invoked by the `.cmd` shim — never call directly with `pwsh -File`. |
| `detect-ref.ps1` | Inspects the current git repo and emits structured fields describing what ref the skill should diff against. Takes no arguments, so it is safe to invoke directly. |

Invocation patterns differ by script:

```powershell
# launch-revdiff — always go through the .cmd shim so Windows paths round-trip:
& "$env:CLAUDE_SKILL_DIR\scripts\launch-revdiff.cmd" [revdiff args...]

# detect-ref — no args, so pwsh -File is safe here:
pwsh -NoProfile -ExecutionPolicy Bypass -File "$env:CLAUDE_SKILL_DIR\scripts\detect-ref.ps1"
```

**Never** invoke `launch-revdiff.ps1` via `pwsh -File`. PowerShell's `-File` parameter parser splits any arg containing a colon (`--compare-old=C:\path` becomes two args), silently breaking every Windows absolute-path flag (`--compare-old`, `--compare-new`, `--only`, `--annotations`, `--description-file`, `--config`, `--keys`, `--history-dir`, `--view`). The `.cmd` shim sidesteps this by forwarding into `pwsh -Command "& '<ps1>' %*"`, which preserves colons intact. For free-text values that contain spaces (`--description="hello world"`), use `--description-file=<path>` instead — the `%*` byte-string forwarding mangles strings that contain both spaces and embedded double quotes.

The launcher exits with an error if `wezterm.exe` is not on PATH or `$env:WEZTERM_PANE` is unset, so Claude Code must be running inside a WezTerm pane for the plugin to work.

The launcher also supports a **fork-only `--view=<path>` flag** (not a revdiff flag — it is intercepted by the launcher) that pipes the named file into `revdiff --stdin --stdin-name=<basename>`. This lets you annotate a tracked-clean file that has no diff, which `--only=<path>` cannot render.
