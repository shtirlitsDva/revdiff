# Installation

**Homebrew (macOS/Linux):**
```bash
brew install umputun/apps/revdiff
```

**Go install:**
```bash
go install github.com/umputun/revdiff/cmd/revdiff@latest
```

**Binary releases:** download from [GitHub Releases](https://github.com/umputun/revdiff/releases) (deb, rpm, archives for linux/darwin amd64/arm64).

**Windows (10/11 + WezTerm):** install via Go from PowerShell:

```powershell
go install github.com/umputun/revdiff/cmd/revdiff@latest
```

This produces `revdiff.exe` under `%USERPROFILE%\go\bin` (or `%GOPATH%\bin`). Make sure that directory is on your `PATH`. Or build from a local checkout with `.\build.ps1` (output: `.bin\revdiff.exe`), run the tests with `.\test.ps1`. `git` must be on `PATH`. WezTerm is the only validated Windows terminal; cmd.exe, Windows Terminal, ConEmu, and mintty are not tested.

## Claude Code Plugin

```bash
/plugin marketplace add umputun/revdiff
/plugin install revdiff@umputun-revdiff
```

Use: `/revdiff [base] [against]` — opens review session in a terminal overlay (tmux, kitty, wezterm, cmux, ghostty, iTerm2, or Emacs vterm).

On Windows, both the `revdiff` and `revdiff-planning` plugins work under WezTerm. The Windows launcher uses `wezterm cli spawn --new-tab` to open revdiff in a new tab of the current WezTerm window. WezTerm is the only supported Windows terminal.

### Plan Review Plugin

Automatically opens revdiff when Claude exits plan mode for interactive annotation:

```bash
/plugin install revdiff-planning@umputun-revdiff
```
