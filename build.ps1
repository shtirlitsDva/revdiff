# ------------------------------------------------------------------------------
# build.ps1 — Windows PowerShell mirror of the Makefile `build` target.
#
# Purpose:
#   Produce `.bin\revdiff.exe` (and a branch-suffixed sibling) from
#   `app\` using the same `go build` flags the Makefile uses, so
#   Windows contributors can build without Git Bash, MinGW, or `make`.
#
# Makefile counterpart (see ./Makefile, `build` target):
#   go build -ldflags "-X main.revision=$(REV) -s -w" \
#            -o .bin/revdiff.$(BRANCH) ./app
#   cp .bin/revdiff.$(BRANCH) .bin/revdiff
#
# Revision string format (matches Makefile REV):
#   <BRANCH>-<HASH>-<TIMESTAMP>
#     BRANCH    = exact git tag on HEAD, or current branch name
#                 (overridable via $env:VERSION, like Makefile BRANCH override)
#     HASH      = `git rev-parse --short=7 HEAD`
#     TIMESTAMP = HEAD commit time in UTC, formatted yyyyMMddTHHmmss
#   If git data is unavailable, REV falls back to "latest" (matches Makefile).
#
# Usage:
#   .\build.ps1                       # build only -> .bin\revdiff.exe
#   .\build.ps1 -Install              # build, then copy onto the on-PATH dir
#   .\build.ps1 -Install -InstallDir 'C:\tools\bin'   # install to a chosen dir
#   $env:VERSION = 'v0.14.0'; .\build.ps1             # override branch/tag component
#
# Install dir resolution (only when -Install is passed):
#   -InstallDir argument  >  $env:REVDIFF_INSTALL_DIR  >  $HOME\.local\bin
#   Default is $HOME\.local\bin because that dir (unlike $HOME\bin) is on the
#   persistent Windows PATH, so cmd, PowerShell AND git-bash all resolve the
#   same binary. $HOME\bin is git-bash-only (prepended by /etc/profile.d), which
#   is exactly how a stale copy there ends up shadowing the real one in bash.
#   The script warns if the install dir is not on PATH, or if other revdiff.exe
#   copies on PATH would shadow the one just installed. Keeping a single,
#   current binary on PATH is what prevents the cryptic "unknown flag" failures
#   that occur when a stale binary shadows a newer skill/launcher.
#
# Exit codes:
#   0 on success; non-zero on any failure. `$ErrorActionPreference = 'Stop'`
#   plus explicit `$LASTEXITCODE` checks ensure we throw on external failures.
# ------------------------------------------------------------------------------

# param() must be the first executable statement (before Set-StrictMode).
param(
    [switch] $Install,
    [string] $InstallDir
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repoRoot = $PSScriptRoot
$binDir   = Join-Path $repoRoot '.bin'
$appDir   = Join-Path $repoRoot 'app'

if (-not (Test-Path -LiteralPath $binDir)) {
    New-Item -ItemType Directory -Path $binDir | Out-Null
}

# Invoke-Git: run a git command silently, return trimmed stdout, or $null on
# non-zero exit. Swallows stderr so missing-tag / detached-HEAD noise does not
# leak to the console (matches Makefile's `2>/dev/null` shell redirects).
# Note: parameter is named $GitArgs (not $Args) to avoid shadowing the
# PowerShell automatic variable.
function Invoke-Git {
    param([Parameter(Mandatory)][string[]]$GitArgs)
    try {
        $out = & git @GitArgs 2>$null
    } catch {
        return $null
    }
    if ($LASTEXITCODE -ne 0) { return $null }
    if ($null -eq $out) { return '' }
    return ($out -join "`n").Trim()
}

# BRANCH: prefer $env:VERSION, else exact tag on HEAD, else current branch.
if ($env:VERSION) {
    $branch = $env:VERSION
} else {
    $tag = Invoke-Git @('describe', '--tags', '--abbrev=0', '--exact-match')
    if ($tag) {
        $branch = $tag
    } else {
        $branchOut = Invoke-Git @('rev-parse', '--abbrev-ref', 'HEAD')
        $branch = if ($branchOut) { $branchOut } else { '' }
    }
}

$hash = Invoke-Git @('rev-parse', '--short=7', 'HEAD')
if (-not $hash) { $hash = '' }

$timestamp = ''
$epoch = Invoke-Git @('log', '-1', '--format=%ct', 'HEAD')
if ($epoch) {
    try {
        $epochInt = [int64]$epoch
        $timestamp = [DateTimeOffset]::FromUnixTimeSeconds($epochInt).UtcDateTime.ToString('yyyyMMddTHHmmss')
    } catch {
        $timestamp = ''
    }
}

$gitRev = "$branch-$hash-$timestamp"
# Matches Makefile: `REV=$(if $(filter --,$(GIT_REV)),latest,$(GIT_REV))`.
# When all three components are empty the joined string is exactly "--".
if ($gitRev -eq '--') {
    $rev = 'latest'
    $branchForFilename = 'latest'
} else {
    $rev = $gitRev
    $branchForFilename = if ($branch) { $branch } else { 'latest' }
}

# Sanitize the branch component for use in a filename (branch names may
# legally contain `/` which is invalid in Windows file names).
$invalidChars = [System.IO.Path]::GetInvalidFileNameChars()
foreach ($c in $invalidChars) {
    $branchForFilename = $branchForFilename.Replace([string]$c, '_')
}

$branchBinary    = Join-Path $binDir ("revdiff.$branchForFilename.exe")
$canonicalBinary = Join-Path $binDir 'revdiff.exe'

# Build from app, output to the branch-suffixed binary in .bin\.
# Using an absolute output path keeps us robust against the Push-Location.
Push-Location -LiteralPath $appDir
try {
    $ldflags = "-X main.revision=$rev -s -w"
    & go build -ldflags $ldflags -o $branchBinary
    if ($LASTEXITCODE -ne 0) {
        throw "go build failed with exit code $LASTEXITCODE (ldflags: $ldflags)"
    }
} finally {
    Pop-Location
}

# Mirror the Makefile's `cp .bin/revdiff.$(BRANCH) .bin/revdiff` step so the
# canonical output path is always `.bin\revdiff.exe`.
Copy-Item -LiteralPath $branchBinary -Destination $canonicalBinary -Force

Write-Host "Built $canonicalBinary"
Write-Host "  (also: $branchBinary)"
Write-Host "  revision: $rev"

# ------------------------------------------------------------------------------
# Optional install: copy the freshly built binary onto a PATH directory so the
# launcher (which resolves `revdiff` from PATH) always runs the current build.
# Without this, .bin\revdiff.exe stays inside the repo and the on-PATH copy
# silently rots, which is how an old binary ends up rejecting flags a newer
# skill passes.
# ------------------------------------------------------------------------------
if ($Install) {
    # Resolve install dir: -InstallDir > $env:REVDIFF_INSTALL_DIR > $HOME\bin.
    if ([string]::IsNullOrWhiteSpace($InstallDir)) {
        $InstallDir = $env:REVDIFF_INSTALL_DIR
    }
    if ([string]::IsNullOrWhiteSpace($InstallDir)) {
        $InstallDir = Join-Path (Join-Path $HOME '.local') 'bin'
    }

    if (-not (Test-Path -LiteralPath $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir | Out-Null
    }

    $installTarget = Join-Path $InstallDir 'revdiff.exe'
    Copy-Item -LiteralPath $canonicalBinary -Destination $installTarget -Force
    $installTargetResolved = (Resolve-Path -LiteralPath $installTarget).ProviderPath
    Write-Host "Installed $installTargetResolved"

    # PATH diagnostics: a current binary only helps if PATH resolves it first.
    # Warn on (a) install dir not on PATH and (b) other revdiff.exe copies that
    # could shadow the freshly installed one.
    $pathDirs = @($env:PATH -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })

    $installDirResolved = (Resolve-Path -LiteralPath $InstallDir).ProviderPath
    $installDirOnPath = $false
    foreach ($d in $pathDirs) {
        $dr = (Resolve-Path -LiteralPath $d -ErrorAction SilentlyContinue)
        if ($null -ne $dr -and $dr.ProviderPath -eq $installDirResolved) { $installDirOnPath = $true; break }
    }
    if (-not $installDirOnPath) {
        Write-Warning "$installDirResolved is not on PATH - the installed binary will not be found until you add it."
    }

    $others = @()
    foreach ($d in $pathDirs) {
        $candidate = Join-Path $d 'revdiff.exe'
        if (Test-Path -LiteralPath $candidate) {
            $cr = (Resolve-Path -LiteralPath $candidate).ProviderPath
            if ($cr -ne $installTargetResolved) { $others += $cr }
        }
    }
    $others = @($others | Select-Object -Unique)
    if ($others.Count -gt 0) {
        Write-Warning "Other revdiff.exe on PATH may shadow the one just installed - remove these (or ensure $installDirResolved comes first):"
        foreach ($o in $others) { Write-Warning "  $o" }
    }
}
