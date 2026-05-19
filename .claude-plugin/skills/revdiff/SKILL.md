---
name: revdiff
description: Review diffs, files, and documents with inline annotations in a TUI overlay, or answer questions about revdiff usage, configuration, themes, and keybindings. Opens revdiff in a WezTerm split pane on Windows, captures annotations, and addresses them. Works in git, hg, and jj repos (auto-detected). Activates on "revdiff", "review diff", "review changes", "annotate diff", "git review with revdiff", "hg review with revdiff", "review jj change", "interactive diff review", "revdiff all files", "review all files", "browse all files", "revdiff <file>", "revdiff README.md", "review this file", "annotate this file", "review file with revdiff", "open this review in revdiff", "show review in revdiff", "review in revdiff", "revdiff config", "revdiff themes", "revdiff keybindings", "how to configure revdiff", "what themes does revdiff have".
argument-hint: 'optional: ref(s), "all files", or file path'
allowed-tools: [Bash, Read, Edit, Write, Grep, Glob]
---

# revdiff - TUI Diff Review (Windows fork)

Review diffs with inline annotations using revdiff TUI in a WezTerm split pane. Works in git, hg, and jj repos (auto-detected).

This is the Windows-only fork of `umputun/revdiff`. Terminal support is **WezTerm only**; shell is **PowerShell (pwsh)**.

## Activation Triggers

- "revdiff", "review diff", "review changes", "annotate diff"
- "revdiff HEAD~1", "revdiff main"
- "hg review with revdiff", "review jj change"
- "revdiff all files", "review all files", "browse all files"
- "revdiff all files exclude vendor"
- "revdiff README.md", "revdiff docs\plan.md", "revdiff $env:TEMP\notes.txt" — single-file review (`--only` mode)
- "review this file", "annotate this file", "review file with revdiff"
- "open this review in revdiff", "show review in revdiff", "review in revdiff" — open an in-session review (preload mode)
- "compare two files", "diff two files", "compare spec-v1 vs spec-v2", "diff old vs new", "before vs after", "revdiff compare <a> <b>" — two-file diff (`--compare-old`/`--compare-new` mode, no VCS required)

## Answering Questions

If the user asks a question about revdiff (configuration, themes, keybindings, installation, usage) rather than requesting a review session, consult the reference files in `references/` and answer directly. Do NOT launch the TUI for informational questions.

- `references/install.md` — installation methods and plugin setup
- `references/config.md` — config file, options, colors, chroma themes
- `references/usage.md` — examples, key bindings, output format

## Using Existing Review History

If the user says things like "locate my review", "use my latest revdiff annotations", "pull up the review I just did in another terminal", or "what did I annotate earlier" — the user ran revdiff outside this plugin flow and wants Claude to process the stored annotations. Find the most recent history file under `%APPDATA%\revdiff\history\<repo-name>\` (override via `$env:REVDIFF_HISTORY_DIR`) and process the annotations through Step 3.5 classification as if they had come from a fresh launcher call.

Each history file contains a header (path, refs, and — when available — a git commit hash), the annotations in `## file:line (type)` format, and the raw git diff for annotated files. The `commit:` line and diff block are captured from git only; in hg/jj repos the diff block will be empty and no commit hash is recorded. See `references/usage.md` "Review History" section for directory layout, stdin/only handling, and override options.

## Opening an In-Session Review

When the user asks to open an in-session review in revdiff (the conversation already contains review comments produced earlier in the session), write those comments to a temp file under `$env:TEMP` (e.g. `$env:TEMP\revdiff-review-<random>.md`) using the format documented in `references/usage.md` ("Output Format" section), then run the normal launcher flow (Step 1 ref detection, Step 2 invocation) with `--annotations=<temp-path>` appended. Step 3 onward handles the curated annotations as usual.

## How It Works

1. Launch revdiff in a WezTerm split pane next to the pane running Claude Code
2. User navigates the diff, adds annotations on specific lines
3. On quit, annotations are captured from the launcher's `--output` file and printed to stdout
4. Claude reads annotations and addresses each one
5. Loop: re-launch revdiff to verify fixes, user can add more annotations
6. Done when user quits without annotations

## Workflow

### Step 0: Verify Installation

```powershell
Get-Command revdiff
```

If not found, guide installation — see `references/install.md`:
- `go install github.com/shtirlitsDva/revdiff/app@latest`
- Or build from source with `.\build.ps1`

### Step 1: Determine Review Mode

**Compare mode**: If `$ARGUMENTS` (or the user's natural-language request) names two distinct file paths with a comparison verb ("compare A vs B", "diff <old> and <new>", "before vs after", "v1 vs v2"), use **compare mode**:
- Pass `--compare-old=<path-a> --compare-new=<path-b>` to the launcher (no positional refs, no `--only`)
- Compare mode is **mutually exclusive** with refs, `--staged`, `--only`, `--all-files`, `--stdin`, `--include`, `--exclude`, and `--annotations` — do not combine
- Works **outside** a git repo and on **untracked** files — revdiff calls `git diff --no-index` under the hood, so only `git` itself needs to be on PATH
- Pick which file is "old" vs "new" from the user's wording (`v1 vs v2` → old=v1, new=v2; "compare before.md and after.md" → old=before, new=after). If ambiguous, ask
- Skip ref detection entirely, go directly to Step 2

**All-files mode**: If `$ARGUMENTS` matches "all files", "all-files", or "browse all files" (with optional "exclude <prefix>" parts), use **all-files mode**:
- Pass `--all-files` to the launcher
- If user mentions exclude patterns (e.g., "exclude vendor", "exclude vendor and mocks"), pass each as `--exclude=<prefix>`
- Skip ref detection entirely, go directly to Step 2
- Example: "all files exclude vendor" → `--all-files --exclude=vendor`

**File review mode**: If `$ARGUMENTS` is a single token that points at a file on disk (e.g., `docs\plans\feature.md`, `$env:TEMP\notes.txt`, `README.md`, `main.go`, `file.blah`), treat it as file review:
- Decide with `Test-Path -LiteralPath $ARGUMENTS -PathType Leaf` — if the file exists, it's file review mode
- Also treat as file review if the token contains a drive letter (e.g. `C:\…`), starts with `.\` or `\`, or contains a path separator and has a file extension (e.g., `src\app.go`), even when the file is not yet reachable from the current directory
- Skip ref detection entirely
- Go directly to Step 2 with `--only=<filepath>` (no ref argument)
- Works both inside and outside a VCS repo — revdiff reads the file from disk as context-only
- Ambiguous token (e.g., `main` — both a branch name and a potential filename without extension) → prefer ref mode; ask the user only if neither `Test-Path` nor `git rev-parse --verify` resolves

**Ref mode**: If `$ARGUMENTS` contains explicit ref(s) (e.g., `HEAD~1`, `main`, or `main feature` for two-ref diff), use as-is.

**Auto-detect**: If no ref provided, run the smart detection script. (`detect-ref.ps1` accepts no arguments, so `pwsh -File` is safe here — the colon-splitting bug described in Step 2 only bites when args contain `:`.)

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File "$env:CLAUDE_SKILL_DIR\scripts\detect-ref.ps1"
```

The script outputs structured fields:
- `branch`, `main_branch`, `is_main`, `has_uncommitted`, `has_staged_only`
- `suggested_ref` — the ref to pass to revdiff (empty = uncommitted changes)
- `use_staged` — if `true`, pass `--staged` to the launcher (staged-only changes detected)
- `needs_ask` — if `true`, ask the user before proceeding

**When `use_staged: true`**, pass `--staged` to the launcher. This means all changes are in the index (staged) with nothing unstaged — without `--staged`, revdiff would show an empty diff.

**When `needs_ask: true`** (on a feature branch with uncommitted changes), use AskUserQuestion:
- **"Uncommitted only"** — pass no ref (review just working changes)
- **"Branch vs {main_branch}"** — pass main_branch as ref (full branch diff including uncommitted)

**When `needs_ask: false`**, use `suggested_ref` directly:
- On main + uncommitted → no ref (uncommitted changes)
- On main + staged only → no ref + `--staged` (staged changes)
- On main + clean → `HEAD~1` (last commit)
- On feature branch + clean → main branch name (full branch diff)

### Step 2: Launch Review

When you are launching revdiff for the user (e.g., right after a refactor or analysis), pass `--description="..."` so the info popup (`i` key) explains what the change is and what to look at — markdown is supported. For longer prose, write the markdown to a temp file and pass `--description-file=$env:TEMP\revdiff-desc-<random>.md`. The two flags are mutually exclusive; both are optional. Skip when there's no useful context to add.

**When the recent change likely created new untracked files** (new packages, new test files, new docs, new scripts that haven't been `git add`-ed yet), pass `--untracked` so those files appear in the tree. Use this in working-tree mode (no ref, no `--staged`); skip it for ref-to-ref reviews where untracked files are not part of the historical diff.

Run the bundled launcher via its `.cmd` shim — **never** invoke `launch-revdiff.ps1` directly with `pwsh -File`, because pwsh's `-File` argument parser splits any arg containing a colon (e.g. `--compare-old=C:\path` becomes two args), silently breaking every Windows absolute-path flag. The `.cmd` shim forwards args verbatim into `pwsh -Command`, which preserves them intact:

```powershell
& "$env:CLAUDE_SKILL_DIR\scripts\launch-revdiff.cmd" [base] [against] [--staged] [--untracked] [--only=file1] [--all-files] [--exclude=prefix] [--compare-old=path --compare-new=path] [--description=text|--description-file=path] [--view=path]
```

For `--description=` values containing spaces, write the markdown to a temp file and use `--description-file=$env:TEMP\revdiff-desc-<random>.md` instead. The shim's `%*` forwarding round-trips path-shaped args cleanly but mangles strings that contain both spaces and embedded double quotes.

The launcher requires WezTerm — `wezterm.exe` must be on PATH and `$env:WEZTERM_PANE` must be set (WezTerm sets this automatically inside its panes). The launcher spawns revdiff in a split pane of the current WezTerm pane and waits for it to exit.

**Launcher output protocol** (stdout, in arrival order):
1. `[revdiff:STARTED] pane-id=<id>` — proof the launcher reached the TUI-spawn step. If this line never arrives, the launcher crashed before that point.
2. `[revdiff:EXIT code=<n>]` — emitted **only** when revdiff itself returned a non-zero exit status. Surfaces inside-pane fast-failures (bad path, codepage mismatch, missing file). When this line arrives, treat it as a **hard error** — not as user-approval.
3. `[revdiff:STDERR] <line>` — emitted **only** together with a non-zero EXIT, one prefixed line per non-empty stderr line captured from revdiff. Tells you *why* revdiff failed (`unknown flag \`...\``, `cannot open <path>`, etc.). Surface these lines verbatim to the user when reporting the failure.
4. Annotation text from revdiff's `--output` file (empty if no annotations were written, which is the normal "quit without comments" case).

When parsing annotations, strip lines matching `^\[revdiff:` but check for an `EXIT` line first and surface both the exit code and any STDERR lines to the user.

**Fork-only `--view=<path>` flag**: pipes the named file into `revdiff --stdin --stdin-name=<basename>` so a tracked-clean file renders as a context-only scratch buffer. Use this when `--only=<path>` would yield "no files match" because the file has no git diff.

**IMPORTANT — long-running command**: The launcher blocks until the user finishes reviewing in the WezTerm split pane, which can exceed the default bash tool timeout. Set the bash timeout parameter to the **maximum your harness allows** (e.g. 1800000 or higher on OpenCode). Do NOT use `run_in_background` for this — background-task handling is unreliable for interactive TUI launchers (processes may be killed unprompted, and polling loops can leave the session idle after the review finishes). If the review outlasts the timeout cap, the fallback in Step 3 handles it.

The script:
- Verifies WezTerm is available and `$env:WEZTERM_PANE` is set
- Spawns a WezTerm split-pane that runs revdiff
- Captures annotation output to a temp file under `$env:TEMP`
- Prints captured annotations to stdout

### Step 3: Process Annotations

**Collecting launcher output**: In the normal case the launcher returns synchronously with annotations on stdout — process them as described below. If the bash tool instead reports a timeout (on Claude Code the task keeps running in the background after the 10-minute cap; on other harnesses it may be killed outright), revdiff is almost certainly still open in the WezTerm split. Do NOT retry the launcher. Use the fallback:

1. Tell the user: "The bash tool timed out, but revdiff may still be open. Let me know when you're done reviewing."
2. Wait for the user to reply. They cannot respond while the WezTerm split has focus, so their reply confirms revdiff has exited.
3. Read the most recent output file from `$env:TEMP`:
   ```powershell
   $f = Get-ChildItem -LiteralPath $env:TEMP -Filter 'revdiff-output-*' -File `
        | Sort-Object LastWriteTime -Descending | Select-Object -First 1
   if ($f) { Get-Content -LiteralPath $f.FullName -Raw }
   ```
4. If it has content, process as annotations below. If empty or no file, the user quit without annotating.

This fallback is safe because revdiff writes the output file atomically on exit — there is never a partial read.

If the script produces output, the user made annotations. The output format is:

```
## file.go:43 (+)
use errors.Is() instead of direct comparison

## store.go:18 (-)
don't remove this validation
```

Each annotation block has:
- `## filename:line (type)` — which file and line, `(+)` = added, `(-)` = removed, `(file-level)` = file note
- Comment text below — what the user wants changed

### Step 3.5: Classify Annotations

Split annotations into two categories:

**Explanation requests** — annotation matches either rule (case-insensitive):
- contains two or more consecutive question marks anywhere in the text (`??`, `???`, etc.) — a language-neutral shortcut for "please explain"
- OR starts with one of: `explain`, `remind`, `describe`, `what is`, `what are`, `how does`, `how do`, `clarify`

These are questions the user wants answered, not code changes.

**Code-change directives** — everything else. These are instructions to modify code.

**If explanation requests are found:**

1. Answer each explanation request — read the referenced code, generate a clear markdown explanation
2. If there are also code-change directives in the same batch, note them as pending (they carry over to Step 4 after the explanation loop)
3. Enter the **explanation loop**:

   a. Write the explanation to a temp markdown file (e.g., `$env:TEMP\revdiff-explain-<random>.md`)
   b. Launch revdiff with `--only=<temp-path>` via the launcher script — this opens the explanation as a scrollable markdown view with TOC sidebar
   c. **If user quits without annotations** → explanation accepted, clean up temp file, proceed:
      - If pending code-change directives exist → go to Step 4
      - Otherwise → go to Step 6 (re-launch revdiff with the original diff ref)
   d. **If user annotates the explanation** → these are follow-up questions or clarification requests. Read the annotations, refine/extend the explanation markdown, write updated temp file, go back to step (b)

The explanation loop continues until the user quits without annotating. This allows a natural back-and-forth dialogue where the user can ask for more detail or corrections on specific parts of the explanation.

**If no explanation requests** — all annotations are code-change directives, proceed directly to Step 4.

### Step 4: Plan Changes

Enter plan mode (EnterPlanMode) to analyze code-change annotations:
- List each annotation with file and line reference
- Describe the planned change for each
- Get user approval before modifying code

### Step 5: Address Annotations

After plan approval, fix the actual source code. Each annotation is a directive.

### Step 6: Loop

After fixing (or after "Continue review" from Step 3.5), run the launcher script again with the same ref. The user can:
- Add more annotations → go back to Step 3
- Quit without annotations → review complete (no output)

### Step 7: Done

When the script produces no output, the review is complete. Inform the user.

## Example Sessions

```
User: "revdiff HEAD~1"
→ launch revdiff in a WezTerm split pane with HEAD~1 diff
→ user annotates: "handler.go:43 - use errors.Is()"
→ user quits
→ annotations captured
→ enter plan mode: "add errors.Is() check at handler.go:43"
→ user approves
→ fix applied
→ re-launch revdiff HEAD~1
→ user sees fix, quits without annotations
→ "review complete"
```

```
User: "revdiff HEAD~3"
→ launch revdiff in a WezTerm split pane with HEAD~3 diff
→ user annotates: "server.go:72 - explain what this mutex protects"
→ user quits
→ annotation classified as explanation request (starts with "explain")
→ Claude reads server.go:72, generates markdown explanation
→ writes to $env:TEMP\revdiff-explain-<random>.md
→ launch revdiff --only=<temp-path> (explanation view with TOC)
→ user reads explanation, annotates: "what about the race condition on line 80?"
→ Claude refines explanation, rewrites temp file
→ re-launch revdiff --only=<temp-path>
→ user reads updated explanation, quits without annotations
→ explanation accepted, clean up temp file
→ re-launch revdiff HEAD~3 (back to diff review)
→ user quits without annotations
→ "review complete"
```

```
User: "revdiff all files exclude vendor"
→ launch revdiff with --all-files --exclude=vendor
→ user browses all tracked files, annotates as needed
→ same annotation loop as above
```

```
User: "revdiff docs\plans\feature.md"
→ Test-Path docs\plans\feature.md succeeds → file review mode
→ launch revdiff with --only=docs\plans\feature.md (context-only view, no ref)
→ user annotates prose: "section 'Open questions':3 - drop this, resolved"
→ user quits
→ same annotation loop as above (applies to the file content)
```

```
User: "compare spec-v1.md and spec-v2.md"
→ two file paths + comparison verb → compare mode
→ launch revdiff with --compare-old=spec-v1.md --compare-new=spec-v2.md (no VCS lookup, no ref)
→ user reviews the side-by-side diff (works even on untracked files)
→ user annotates: "spec-v2.md (file-level) - add migration note about deprecated /refresh endpoint"
→ user quits
→ same annotation loop as above (applies to the v2 file content)
```
