# launch-revdiff.ps1 - Windows PowerShell port of launch-revdiff.sh (WezTerm only).
#
# Launches revdiff in a WezTerm split pane of the current pane (the one
# running Claude Code) and captures annotation output from revdiff's --output
# file. Blocks until the split pane exits, then writes the captured
# annotations to stdout.
#
# Usage:  launch-revdiff.ps1 [ref] [--staged] [--only=file1 ...]
# Output: annotation text from revdiff's --output file (empty if none)
#
# Scope:
#   The bash sibling (launch-revdiff.sh) supports tmux, kitty, wezterm, cmux,
#   ghostty, iTerm2, and Emacs vterm. This PowerShell port targets WezTerm ONLY
#   because it is the only terminal emulator on Windows that exposes a
#   programmable tab/pane spawning CLI comparable to wezterm cli. Other
#   terminal integrations are intentionally NOT ported — add another script if
#   support is needed.
#
# Environment variables honored (same as bash sibling):
#   REVDIFF_CONFIG        - optional path to a revdiff config file; added as --config=
#   WEZTERM_PANE          - id of the current WezTerm pane (set by WezTerm);
#                           used as the --pane-id anchor for the split.
#   REVDIFF_POPUP_HEIGHT  - split pane size as percent of current pane (default 90);
#                           trailing '%' is stripped, matching bash sibling.
#   REVDIFF_POPUP_WIDTH   - accepted for signature parity with bash sibling; the
#                           bash wezterm branch uses only height for --percent, so
#                           width is ignored here too.
#
# See also: launch-revdiff.sh (POSIX sibling, keep logic in sync).

# param() must precede any executable statements (including Set-StrictMode).
# Accept all forwarded arguments as a single array; PowerShell binds positional
# and flag-like tokens transparently into $ForwardedArgs because of ValueFromRemainingArguments.
param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]] $ForwardedArgs
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if ($null -eq $ForwardedArgs) {
    $ForwardedArgs = @()
}

# ---------------------------------------------------------------------------
# Resolve revdiff.exe on PATH. Mirrors `command -v revdiff` in the bash sibling.
# ---------------------------------------------------------------------------
$revdiffCmd = Get-Command -Name 'revdiff.exe' -ErrorAction SilentlyContinue
if ($null -eq $revdiffCmd) {
    # fall back to bare name in case PATHEXT resolution differs
    $revdiffCmd = Get-Command -Name 'revdiff' -ErrorAction SilentlyContinue
}
if ($null -eq $revdiffCmd) {
    [Console]::Error.WriteLine('error: revdiff not found in PATH')
    [Console]::Error.WriteLine('install: go install github.com/umputun/revdiff/cmd/revdiff@latest')
    exit 1
}
$revdiffBin = $revdiffCmd.Source

# ---------------------------------------------------------------------------
# Ensure we are actually running under WezTerm. Mirrors the bash wezterm block
# guard (`if [ -n "${WEZTERM_PANE:-}" ]`).
# ---------------------------------------------------------------------------
$weztermPane = $env:WEZTERM_PANE
if ([string]::IsNullOrEmpty($weztermPane)) {
    [Console]::Error.WriteLine('error: WEZTERM_PANE is not set — this launcher only supports WezTerm on Windows')
    [Console]::Error.WriteLine('hint: run from inside a WezTerm session')
    exit 1
}

$weztermCmd = Get-Command -Name 'wezterm.exe' -ErrorAction SilentlyContinue
if ($null -eq $weztermCmd) {
    $weztermCmd = Get-Command -Name 'wezterm' -ErrorAction SilentlyContinue
}
if ($null -eq $weztermCmd) {
    [Console]::Error.WriteLine('error: wezterm CLI not found in PATH')
    exit 1
}
$weztermBin = $weztermCmd.Source

# ---------------------------------------------------------------------------
# Set up temp files for revdiff output + split-pane exit sentinel.
# Use proper Windows temp via GetTempFileName — never hard-coded /tmp/... paths.
# ---------------------------------------------------------------------------
$outputFile   = [System.IO.Path]::GetTempFileName()
$sentinelFile = [System.IO.Path]::GetTempFileName()
# sentinel must NOT exist when polling starts; the split-pane shell creates it on exit
Remove-Item -LiteralPath $sentinelFile -Force -ErrorAction SilentlyContinue

$exitCode      = 0
$paneId        = $null
$cmdScriptFile = $null

try {
    # -----------------------------------------------------------------------
    # Build the revdiff command line that will run inside the split pane.
    #
    # revdiff forwarded args come from the caller; we always append
    # --output=<outputFile> and, if REVDIFF_CONFIG points to an existing file,
    # --config=<path> (matches the bash block).
    # -----------------------------------------------------------------------
    $revdiffArgs = @()

    if (-not [string]::IsNullOrEmpty($env:REVDIFF_CONFIG) -and (Test-Path -LiteralPath $env:REVDIFF_CONFIG -PathType Leaf)) {
        $revdiffArgs += "--config=$($env:REVDIFF_CONFIG)"
    }

    $revdiffArgs += "--output=$outputFile"
    if ($ForwardedArgs.Count -gt 0) {
        $revdiffArgs += $ForwardedArgs
    }

    # -----------------------------------------------------------------------
    # Resolve split-pane height percent. Mirrors bash sibling:
    #   WEZTERM_PCT="${REVDIFF_POPUP_HEIGHT:-90%}"
    #   WEZTERM_PCT="${WEZTERM_PCT%%%}"
    # Accept "90" or "90%"; default to 90.
    # -----------------------------------------------------------------------
    $weztermPct = $env:REVDIFF_POPUP_HEIGHT
    if ([string]::IsNullOrEmpty($weztermPct)) {
        $weztermPct = '90'
    }
    $weztermPct = $weztermPct.TrimEnd('%')

    # -----------------------------------------------------------------------
    # Open revdiff in a WezTerm split pane via `wezterm cli split-pane`.
    # `--bottom --percent <N> --pane-id <current>` splits the pane containing
    # our Claude Code session horizontally, placing revdiff in the bottom
    # portion — guaranteed visible, no tab-creation semantics. Matches the
    # upstream bash sibling `launch-revdiff.sh` (wezterm branch, lines 83-108).
    #
    # IMPORTANT Windows quirk: `wezterm cli split-pane -- <PROG> <ARGS>...`
    # does NOT reliably forward multi-arg commands like `cmd.exe /c "..."`
    # to the spawned pane — the args get swallowed and cmd.exe starts in
    # interactive mode instead. The ONLY reliable pattern is to pass a single
    # executable/script as PROG with NO extra args. So we write the revdiff
    # invocation + sentinel-touch to a temp .cmd file and pass its path as
    # the sole PROG argument. cmd.exe runs it and exits cleanly. This is the
    # same workaround applied in launch-plan-review.ps1 — see commit c4667be
    # for the full diagnosis.
    # -----------------------------------------------------------------------
    $cwdForSpawn = (Get-Location).ProviderPath

    # Build the contents of the temp .cmd file. Each line runs one step:
    #   1. revdiff.exe with its args (paths are quoted — no embedded quotes
    #      possible since we control the values, except for ForwardedArgs
    #      which we also wrap in quotes; cmd.exe's double-quote rules are
    #      lenient enough that this is safe for normal ref names and paths).
    #   2. break > <sentinel> to signal completion (atomic empty-file create).
    # @echo off keeps cmd from echoing each command to the pane.
    $cmdScriptLines = @(
        '@echo off'
        '"' + $revdiffBin + '" ' + (($revdiffArgs | ForEach-Object { '"' + $_ + '"' }) -join ' ')
        'break > "' + $sentinelFile + '"'
    )

    $cmdScriptFile = [System.IO.Path]::GetTempFileName() + '.cmd'
    # Windows cmd.exe needs CRLF line endings; UTF-8 without BOM is safest for cmd.
    [System.IO.File]::WriteAllText(
        $cmdScriptFile,
        ($cmdScriptLines -join "`r`n") + "`r`n",
        [System.Text.UTF8Encoding]::new($false)
    )

    $wezArgs = @(
        'cli', 'split-pane',
        '--bottom',
        '--percent', $weztermPct,
        '--pane-id', $weztermPane,
        '--cwd', $cwdForSpawn,
        '--',
        $cmdScriptFile
    )

    # & with an array — no shell interpolation.
    $paneId = & $weztermBin @wezArgs
    if ($LASTEXITCODE -ne 0) {
        throw "wezterm cli split-pane failed with exit code $LASTEXITCODE"
    }

    # wezterm cli split-pane prints the new pane id on stdout; trim whitespace.
    if ($null -ne $paneId) {
        $paneId = ($paneId | Out-String).Trim()
    }

    # -----------------------------------------------------------------------
    # Wait for the split pane to exit. We poll the sentinel file (created by
    # `break > <sentinel>` when revdiff returns) — this is the Windows sibling
    # of the bash `while [ ! -f "$SENTINEL" ]; do sleep 0.3; done` pattern,
    # as required by the task spec (no mkfifo / named pipes).
    # -----------------------------------------------------------------------
    while (-not (Test-Path -LiteralPath $sentinelFile)) {
        Start-Sleep -Milliseconds 300
    }

    # -----------------------------------------------------------------------
    # Dump captured annotation output to stdout — matches `cat "$OUTPUT_FILE"`.
    # Use raw read so line endings and trailing newlines are preserved exactly.
    # -----------------------------------------------------------------------
    if (Test-Path -LiteralPath $outputFile) {
        $content = Get-Content -LiteralPath $outputFile -Raw -ErrorAction SilentlyContinue
        if (-not [string]::IsNullOrEmpty($content)) {
            # Write without adding an extra newline — Write-Host would add one.
            [Console]::Out.Write($content)
        }
    }
}
catch {
    [Console]::Error.WriteLine("error: $($_.Exception.Message)")
    $exitCode = 1
}
finally {
    # -----------------------------------------------------------------------
    # Cleanup: remove temp files. We intentionally do NOT try to close the
    # WezTerm split pane — revdiff exits cleanly and the cmd.exe wrapper
    # returns after `break > <sentinel>`, so the pane terminates on its own.
    # Temp-file cleanup runs even on crash/throw thanks to try/finally.
    # -----------------------------------------------------------------------
    if (Test-Path -LiteralPath $outputFile) {
        Remove-Item -LiteralPath $outputFile -Force -ErrorAction SilentlyContinue
    }
    if (Test-Path -LiteralPath $sentinelFile) {
        Remove-Item -LiteralPath $sentinelFile -Force -ErrorAction SilentlyContinue
    }
    if ($null -ne $cmdScriptFile -and (Test-Path -LiteralPath $cmdScriptFile)) {
        Remove-Item -LiteralPath $cmdScriptFile -Force -ErrorAction SilentlyContinue
    }
}

exit $exitCode
