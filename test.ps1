# ------------------------------------------------------------------------------
# test.ps1 — Windows PowerShell mirror of the Makefile `test` target.
#
# Purpose:
#   Run the Go test suite with the race detector and coverage, excluding
#   generated mocks from the coverage report, exactly like `make test` does.
#   Lets Windows contributors run tests without Git Bash, grep, or `make`.
#
# Makefile counterpart (see ./Makefile, `test` target):
#   go clean -testcache
#   go test -race -coverprofile=coverage.out ./...
#   grep -v "_mock.go" coverage.out | grep -v mocks > coverage_no_mocks.out
#   go tool cover -func=coverage_no_mocks.out
#   rm coverage.out coverage_no_mocks.out
#
# PowerShell differences vs. Makefile:
#   - `grep -v` is replaced with `Where-Object -notmatch` on the raw coverage
#     file contents, per the task acceptance criteria (no Unix tool calls).
#   - Package selection also filters out mock packages via `go list ./...`
#     piped through `Where-Object`, so `go test` never has to visit them.
#   - Output file paths use `Join-Path` — no hardcoded `\` separators.
#
# Usage:
#   .\test.ps1
#
# Exit codes:
#   0 on success; non-zero on any failure. `$ErrorActionPreference = 'Stop'`
#   plus explicit `$LASTEXITCODE` checks ensure we throw on external failures.
# ------------------------------------------------------------------------------

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repoRoot    = $PSScriptRoot
$coverageRaw = Join-Path $repoRoot 'coverage.out'
$coverageOut = Join-Path $repoRoot 'coverage_no_mocks.out'

Push-Location -LiteralPath $repoRoot
try {
    # 1. Clear any stale cached test results.
    & go clean -testcache
    if ($LASTEXITCODE -ne 0) { throw "go clean -testcache failed with exit code $LASTEXITCODE" }

    # 2. Enumerate all packages, then drop generated mock packages.
    #    `go list` is the cross-platform equivalent of the Makefile's `./...`
    #    glob; filtering in PowerShell replaces the Unix `grep -v` pipeline.
    $allPackages = & go list ./...
    if ($LASTEXITCODE -ne 0) { throw "go list ./... failed with exit code $LASTEXITCODE" }

    $packages = @($allPackages | Where-Object {
        ($_ -notmatch '_mock(\.go)?$') -and ($_ -notmatch '/mocks(/|$)')
    })
    if ($packages.Count -eq 0) {
        throw 'No Go packages found to test after filtering mocks.'
    }

    # 3. Run the suite with race detector + coverage, mirroring Makefile flags.
    & go test -race "-coverprofile=$coverageRaw" @packages
    if ($LASTEXITCODE -ne 0) { throw "go test failed with exit code $LASTEXITCODE" }

    # 4. Strip mock-related lines from the coverage profile (matches the
    #    Makefile `grep -v "_mock.go" | grep -v mocks` pipeline).
    if (-not (Test-Path -LiteralPath $coverageRaw)) {
        throw "coverage profile not produced at $coverageRaw"
    }
    $filtered = Get-Content -LiteralPath $coverageRaw | Where-Object {
        ($_ -notmatch '_mock\.go') -and ($_ -notmatch 'mocks')
    }
    Set-Content -LiteralPath $coverageOut -Value $filtered -Encoding ASCII

    # 5. Print function-level coverage summary, same as Makefile.
    & go tool cover "-func=$coverageOut"
    if ($LASTEXITCODE -ne 0) { throw "go tool cover failed with exit code $LASTEXITCODE" }
}
finally {
    Pop-Location
    # 6. Clean up coverage artifacts regardless of success/failure, matching
    #    the Makefile's final `rm` step. Use -ErrorAction SilentlyContinue so
    #    we never mask an earlier real failure with a cleanup error.
    foreach ($path in @($coverageRaw, $coverageOut)) {
        if (Test-Path -LiteralPath $path) {
            Remove-Item -LiteralPath $path -Force -ErrorAction SilentlyContinue
        }
    }
}
