# GitHub Issue Mapping — windows-support

Epic: **#1** — https://github.com/shtirlitsDva/revdiff/issues/1

Tasks (sub-issues of #1):

| Local file | Issue | Title                                                      |
|------------|-------|------------------------------------------------------------|
| 2.md       | #2    | Go core - Windows config paths and TTY reattach            |
| 3.md       | #3    | PowerShell build and test scripts                          |
| 4.md       | #4    | revdiff plugin Windows launcher (PowerShell + WezTerm)     |
| 5.md       | #5    | revdiff-planning plugin Windows launcher (PowerShell + WezTerm) |
| 6.md       | #6    | Documentation pass for Windows support                     |
| 7.md       | #7    | End-to-end Windows validation and plugin version bump      |

## Direct links

- Epic:   https://github.com/shtirlitsDva/revdiff/issues/1
- Task 2: https://github.com/shtirlitsDva/revdiff/issues/2
- Task 3: https://github.com/shtirlitsDva/revdiff/issues/3
- Task 4: https://github.com/shtirlitsDva/revdiff/issues/4
- Task 5: https://github.com/shtirlitsDva/revdiff/issues/5
- Task 6: https://github.com/shtirlitsDva/revdiff/issues/6
- Task 7: https://github.com/shtirlitsDva/revdiff/issues/7

## Dependency graph

```
#2 (Go core) ─────┐
#3 (build/test) ──┤
#4 (revdiff plg) ─┼──> #7 (validation + version bump)
#5 (planning plg) ┤
#6 (docs) ────────┘
```

Tasks #2–#6 are parallel and have no inter-dependencies. Task #7 is sequential and gates the epic.

Synced: 2026-04-07T07:59:48Z
Repository: shtirlitsDva/revdiff (fork — `gh-resolved=base` set in `.git/config`)
