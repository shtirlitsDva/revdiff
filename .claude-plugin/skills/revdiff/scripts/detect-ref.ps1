# detect-ref.ps1 - Windows PowerShell port of detect-ref.sh.
#
# Smart ref detection for the revdiff skill. Outputs structured info about the
# current git state so the skill can decide what ref to use or whether to ask
# the user.
#
# Output format is byte-identical to detect-ref.sh so callers can parse either
# interchangeably:
#   branch: <current branch name>
#   main_branch: <detected main/master branch name>
#   is_main: true/false
#   has_uncommitted: true/false
#   suggested_ref: <ref to use, empty means uncommitted>
#   needs_ask: true/false
#
# See also: detect-ref.sh (POSIX sibling, keep logic in sync).

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown"
$branch = & git rev-parse --abbrev-ref HEAD 2>$null
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrEmpty($branch)) {
    $branch = 'unknown'
}

# detect main branch name from remote HEAD, fallback to master/main check
$main_branch = ''
$remote_head = & git symbolic-ref refs/remotes/origin/HEAD 2>$null
if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrEmpty($remote_head)) {
    # strip "refs/remotes/origin/" prefix (matches bash ${remote_head##refs/remotes/origin/})
    $main_branch = $remote_head -replace '^refs/remotes/origin/', ''
} else {
    & git show-ref --verify --quiet refs/heads/master 2>$null
    if ($LASTEXITCODE -eq 0) {
        $main_branch = 'master'
    } else {
        & git show-ref --verify --quiet refs/heads/main 2>$null
        if ($LASTEXITCODE -eq 0) {
            $main_branch = 'main'
        }
    }
}

$is_main = 'false'
if ($branch -eq $main_branch) {
    $is_main = 'true'
}

$has_uncommitted = 'false'
$status = & git status --porcelain 2>$null
if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrEmpty($status)) {
    $has_uncommitted = 'true'
}

# decision logic — mirrors detect-ref.sh line-for-line
$suggested_ref = ''
$needs_ask = 'false'

if ($is_main -eq 'true') {
    if ($has_uncommitted -eq 'true') {
        $suggested_ref = '' # uncommitted changes on main
    } else {
        $suggested_ref = 'HEAD~1' # last commit on main
    }
} else {
    if ($has_uncommitted -eq 'true') {
        $needs_ask = 'true' # ambiguous: uncommitted on feature branch
    } else {
        $suggested_ref = $main_branch # clean feature branch → diff against main
    }
}

Write-Output "branch: $branch"
Write-Output "main_branch: $main_branch"
Write-Output "is_main: $is_main"
Write-Output "has_uncommitted: $has_uncommitted"
Write-Output "suggested_ref: $suggested_ref"
Write-Output "needs_ask: $needs_ask"
