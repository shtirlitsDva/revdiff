# ------------------------------------------------------------------------------
# build.ps1 — Windows PowerShell mirror of the Makefile `build` target.
#
# Purpose:
#   Produce `.bin\revdiff.exe` (and a branch-suffixed sibling) from
#   `cmd\revdiff` using the same `go build` flags the Makefile uses, so
#   Windows contributors can build without Git Bash, MinGW, or `make`.
#
# Makefile counterpart (see ./Makefile, `build` target):
#   cd cmd/revdiff && go build -ldflags "-X main.revision=$(REV) -s -w" \
#                              -o ../../.bin/revdiff.$(BRANCH)
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
#   .\build.ps1                  # use current branch
#   $env:VERSION = 'v0.14.0'; .\build.ps1   # override branch/tag component
#
# Exit codes:
#   0 on success; non-zero on any failure. `$ErrorActionPreference = 'Stop'`
#   plus explicit `$LASTEXITCODE` checks ensure we throw on external failures.
# ------------------------------------------------------------------------------

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repoRoot = $PSScriptRoot
$binDir   = Join-Path $repoRoot '.bin'
$cmdDir   = Join-Path $repoRoot (Join-Path 'cmd' 'revdiff')

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

# Build from cmd\revdiff, output to the branch-suffixed binary in .bin\.
# Using an absolute output path keeps us robust against the Push-Location.
Push-Location -LiteralPath $cmdDir
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
