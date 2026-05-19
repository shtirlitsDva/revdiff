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

The `revdiff` skill ships with two PowerShell launcher scripts under `.claude-plugin/skills/revdiff/scripts/`:

| Script | Purpose |
|---|---|
| `launch-revdiff.ps1` | Spawns revdiff in a WezTerm split pane, captures the annotation output file, and prints annotations to stdout. |
| `detect-ref.ps1` | Inspects the current git repo and emits structured fields describing what ref the skill should diff against. |

Both scripts are invoked via:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File "$env:CLAUDE_SKILL_DIR\scripts\<script>.ps1" [args...]
```

The launcher exits with an error if `wezterm.exe` is not on PATH or `$env:WEZTERM_PANE` is unset, so Claude Code must be running inside a WezTerm pane for the plugin to work.

The launcher also supports a **fork-only `--view=<path>` flag** (not a revdiff flag — it is intercepted by the launcher) that pipes the named file into `revdiff --stdin --stdin-name=<basename>`. This lets you annotate a tracked-clean file that has no diff, which `--only=<path>` cannot render.
