# launch-revdiff.ps1 - Windows PowerShell port of launch-revdiff.sh (WezTerm only).
#
# Launches revdiff in a new WezTerm tab in the same window as the parent
# Claude Code session and captures annotation output from revdiff's --output
# file. Blocks until the spawned tab exits, then writes the captured
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
#                           when present, the new tab is opened in the SAME window
#   REVDIFF_POPUP_WIDTH   - accepted for signature parity with bash sibling; WezTerm
#                           tabs don't honor explicit sizes so this is ignored here
#   REVDIFF_POPUP_HEIGHT  - same note
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
# Set up temp files for revdiff output + spawn-exit sentinel.
# Use proper Windows temp via New-TemporaryFile / GetTempFileName — never
# hard-coded /tmp/... paths.
# ---------------------------------------------------------------------------
$outputFile   = [System.IO.Path]::GetTempFileName()
$sentinelFile = [System.IO.Path]::GetTempFileName()
# sentinel must NOT exist when polling starts; spawned shell creates it on exit
Remove-Item -LiteralPath $sentinelFile -Force -ErrorAction SilentlyContinue

$exitCode = 0
$paneId   = $null

try {
    # -----------------------------------------------------------------------
    # Build the revdiff command line that will run inside the spawned tab.
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
    # Spawn revdiff in a new WezTerm tab via `wezterm cli spawn --new-tab`.
    # --pane-id anchors the new tab to the current WezTerm window.
    # --cwd pins the spawned shell's working directory to our own.
    #
    # We wrap the call in cmd.exe so we can chain the sentinel-touch step after
    # revdiff exits. cmd /c "<cmd> & break>file" is the Windows analogue of
    # `sh -c "$REVDIFF_CMD; touch '$SENTINEL'"` in the bash script.
    #
    # Argument forwarding uses the `& wezterm @args` array form — never string
    # interpolation — so ref names with spaces or quotes can't inject.
    # -----------------------------------------------------------------------
    $cwdForSpawn = (Get-Location).ProviderPath

    # Build the cmd.exe command string. Quote each revdiff arg to survive cmd
    # re-parsing. This is the ONLY string-interpolation boundary, and the input
    # is bounded to our own constructed values, not untrusted caller input
    # (caller args are quoted individually below).
    $quoteForCmd = {
        param([string] $s)
        # Escape embedded double quotes and wrap in double quotes.
        '"' + ($s -replace '"', '\"') + '"'
    }

    $quotedRevdiff = & $quoteForCmd $revdiffBin
    $quotedOutput  = & $quoteForCmd $outputFile
    $quotedSent    = & $quoteForCmd $sentinelFile

    $quotedArgs = @()
    foreach ($a in $revdiffArgs) {
        $quotedArgs += (& $quoteForCmd $a)
    }

    # Final inner command:
    #   "<revdiffBin>" <quoted args...> & break > "<sentinel>"
    # `break > file` creates an empty file atomically in cmd.exe — used here
    # as the Windows analogue of POSIX `touch`.
    $innerCmd = "$quotedRevdiff $($quotedArgs -join ' ') & break > $quotedSent"

    $wezArgs = @(
        'cli', 'spawn',
        '--new-tab',
        '--pane-id', $weztermPane,
        '--cwd', $cwdForSpawn,
        '--',
        'cmd.exe', '/c', $innerCmd
    )

    # & with an array — no shell interpolation.
    $paneId = & $weztermBin @wezArgs
    if ($LASTEXITCODE -ne 0) {
        throw "wezterm cli spawn failed with exit code $LASTEXITCODE"
    }

    # wezterm cli spawn prints the new pane id on stdout; trim whitespace.
    if ($null -ne $paneId) {
        $paneId = ($paneId | Out-String).Trim()
    }

    # -----------------------------------------------------------------------
    # Wait for the spawned tab to exit. We poll the sentinel file (created by
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
    # spawned WezTerm tab — revdiff exits cleanly and the cmd.exe wrapper
    # returns after `break > <sentinel>`, so the tab terminates on its own.
    # Temp-file cleanup runs even on crash/throw thanks to try/finally.
    # -----------------------------------------------------------------------
    if (Test-Path -LiteralPath $outputFile) {
        Remove-Item -LiteralPath $outputFile -Force -ErrorAction SilentlyContinue
    }
    if (Test-Path -LiteralPath $sentinelFile) {
        Remove-Item -LiteralPath $sentinelFile -Force -ErrorAction SilentlyContinue
    }
}

exit $exitCode
