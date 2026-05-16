---
name: revdiff
description: Review diffs, files, and documents with inline annotations in a TUI overlay, or answer questions about revdiff usage, configuration, themes, and keybindings. Opens revdiff in tmux/kitty/wezterm/cmux/ghostty/iterm2/emacs-vterm, captures annotations, and addresses them. Activates on "revdiff", "review diff", "annotate diff", "git review with revdiff", "interactive diff review", "revdiff all files", "review all files", "browse all files", "revdiff config", "revdiff themes", "revdiff keybindings", "how to configure revdiff", "what themes does revdiff have".
argument-hint: 'optional: git ref(s), "all files", or file path'
allowed-tools: [Bash, Read, Edit, Write, Grep, Glob]
---

<title>revdiff — TUI Diff Review (Windows-only fork)</title>

<platform-scope>
This fork is **Windows-only** for the launch flow. The PowerShell launcher (`launch-revdiff.ps1`) called through `pwsh -NoProfile -Command` is the only supported path. The POSIX `.sh` siblings exist for upstream parity but MUST NOT be used on this machine — they detect terminals (tmux/kitty/iTerm2) that don't exist on Windows and fall through silently, producing the empty-stdout / silent-failure mode described below.
</platform-scope>

<activation-triggers>
- "revdiff", "review diff", "annotate diff"
- "revdiff HEAD~1", "revdiff main"
- "revdiff all files", "review all files", "browse all files"
- "revdiff all files exclude vendor"
</activation-triggers>

<answering-questions>
If the user asks a question about revdiff (configuration, themes, keybindings, installation, usage) rather than requesting a review session, consult the reference files in `references/` and answer directly. Do NOT launch the TUI for informational questions.

- `references/install.md` — installation methods and plugin setup
- `references/config.md` — config file, options, colors, chroma themes
- `references/usage.md` — examples, key bindings, output format
</answering-questions>

<how-it-works>
1. Launch revdiff in a WezTerm split pane of the current pane.
2. User navigates the content, adds annotations on specific lines.
3. On quit, annotations are captured to a temp file and streamed to stdout.
4. Claude reads annotations and addresses each one.
5. Loop: re-launch revdiff to verify fixes; user can add more annotations.
6. Done when user quits without annotations (empty stream after `[revdiff:STARTED]`).
</how-it-works>

<launch-rules-must-follow>
Before invoking revdiff, **verify every rule below applies**. Any one missed produces the failure mode "TUI exits immediately, agent sees empty stdout, assumes user approved with zero annotations". Real fix is to follow all rules; do not interpret a silent exit as success.

1. **Use `pwsh -NoProfile -Command`, never `-File`.** With `-File`, PowerShell's argument binder splits paths on `:`. `--view=H:\foo` becomes `--view=H` and the rest is dropped. revdiff then crashes with "file not found". `-Command` with a single-quoted call operator preserves the full string.

2. **Single-quote every argument inside the `-Command` string.** Single quotes are PowerShell literals — no escaping of `\`, `:`, or `=` needed. Either backslash or forward-slash paths work inside the quotes; pick one and stick with it for an invocation.

3. **Use the `Monitor` tool, not `Bash run_in_background`.** Monitor streams each stdout line as a real-time event and only completes when the launcher exits. `Bash run_in_background` returns a task ID immediately, producing the false-positive "agent thinks it's done".

4. **Pass `persistent: true` to Monitor. NEVER set `timeout_ms`.** Default Monitor timeout is 300s; real reviews routinely exceed that — the user may step away, read slowly, re-open the TUI. A timeout kills the launcher and discards every annotation written after the kill. Call TaskStop to clean up early if needed.

5. **Use the `.ps1` launcher, never the `.sh` sibling.** The bash launcher detects POSIX-only terminals on Windows, finds none, and falls through silently.

6. **For file view, pass `--view=<absolute-path>`. NEVER use `--only=<path>`.** `--only=` filters git diff output; on a tracked-clean file it produces zero diff and revdiff exits with "no files match --only filter" — same false-positive symptom. `--view=` is a launcher-side flag that feeds the file as stdin context, working regardless of git state.

7. **Wait for the `[revdiff:STARTED]` Monitor event.** The launcher emits this sentinel to stdout immediately after the WezTerm split-pane is successfully spawned. **If the Monitor task completes without ever emitting `[revdiff:STARTED]`, the TUI never started — treat that as a hard error, not as "user approved".** When parsing annotations, strip any line matching `^\[revdiff:` before processing.

8. **Never pass `--output=` or `-o`.** The launcher owns the output file. Caller-supplied `--output` is hard-rejected with a thrown error.
</launch-rules-must-follow>

<failure-modes-quick-reference>
| Symptom | Likely cause | Fix |
|---|---|---|
| Monitor completes with empty stdout, no `[revdiff:STARTED]` | Launcher crashed before TUI spawned | Read task output file for stderr; usually wrong pwsh invocation form (rule 1/2). |
| Monitor times out | `timeout_ms` was set | Reissue with `persistent: true` and NO `timeout_ms` (rule 4). |
| Monitor "completes" instantly with no events | Used Bash, not Monitor | Switch to Monitor (rule 3). |
| Got `[revdiff:STARTED]` then long silence | revdiff is running; user is annotating | Wait. Do NOT assume done. Monitor will emit more events when revdiff exits. |
| Stdout contains "no files match --only filter" | Used `--only=` on a tracked-clean file | Switch to `--view=` (rule 6). |
| `<PLUGIN_ROOT>` literal in error message | Did not expand `${CLAUDE_PLUGIN_ROOT}` | The skill harness expands `${CLAUDE_PLUGIN_ROOT}` for you when used inside the Bash tool; use that exact form. |
</failure-modes-quick-reference>

<step-0-verify-installation>
```bash
where.exe revdiff.exe
```

If not found, install:
```bash
go install github.com/umputun/revdiff/cmd/revdiff@latest
```
</step-0-verify-installation>

<step-1-determine-review-mode>

<mode-all-files>
If `$ARGUMENTS` matches "all files", "all-files", or "browse all files" (with optional "exclude <prefix>" parts):
- Forward `--all-files` to the launcher.
- For each "exclude <prefix>" part, forward `--exclude=<prefix>`.
- Skip ref detection entirely → go to Step 2.

Example: "all files exclude vendor" → `--all-files --exclude=vendor`.
</mode-all-files>

<mode-file-view>
If `$ARGUMENTS` is a file path (e.g. `docs/notes.md`, `H:\path\to\file.txt`, or any tracked-clean / untracked file):
- Forward `--view=<absolute-path>` to the launcher.
- Skip ref detection entirely → go to Step 2.
- **Do NOT use `--only=`** (see rule 6).
</mode-file-view>

<mode-ref>
If `$ARGUMENTS` contains explicit git ref(s) (`HEAD~1`, `main`, or `main feature` for a two-ref diff):
- Forward refs as-is.
- Skip ref detection → go to Step 2.
</mode-ref>

<mode-auto-detect>
If no arguments, run detect-ref:
```bash
pwsh -NoProfile -Command "& '${CLAUDE_PLUGIN_ROOT}/.claude-plugin/skills/revdiff/scripts/detect-ref.ps1'"
```

Returns these structured fields:
- `branch`, `main_branch`, `is_main`, `has_uncommitted`
- `suggested_ref` — ref to pass to revdiff (empty = uncommitted only)
- `needs_ask` — `true` if the agent should ask the user before proceeding

**When `needs_ask: true`** (feature branch with uncommitted changes), use AskUserQuestion:
- "Uncommitted only" — pass no ref (review just working changes)
- "Branch vs {main_branch}" — pass `main_branch` as ref (full branch diff including uncommitted)

**When `needs_ask: false`**, use `suggested_ref` directly:
- On main + uncommitted → no ref
- On main + clean → `HEAD~1`
- On feature branch + clean → main branch name
</mode-auto-detect>

</step-1-determine-review-mode>

<step-2-launch>

<canonical-invocation>
```bash
pwsh -NoProfile -Command "& '${CLAUDE_PLUGIN_ROOT}/.claude-plugin/skills/revdiff/scripts/launch-revdiff.ps1' '<arg1>' '<arg2>' ..."
```

Wrap each argument in single quotes inside the `-Command` string. Pass this as the `command` field to the `Monitor` tool with:
- `persistent: true`
- `timeout_ms`: **omitted** (no timeout)
- `description`: human-readable label, e.g. `"revdiff review of foo.md"`
</canonical-invocation>

<concrete-examples>

File view of `H:\path\to\notes.md`:
```bash
pwsh -NoProfile -Command "& '${CLAUDE_PLUGIN_ROOT}/.claude-plugin/skills/revdiff/scripts/launch-revdiff.ps1' '--view=H:\path\to\notes.md'"
```

Diff against `HEAD~1`:
```bash
pwsh -NoProfile -Command "& '${CLAUDE_PLUGIN_ROOT}/.claude-plugin/skills/revdiff/scripts/launch-revdiff.ps1' 'HEAD~1'"
```

All tracked files, excluding `vendor/`:
```bash
pwsh -NoProfile -Command "& '${CLAUDE_PLUGIN_ROOT}/.claude-plugin/skills/revdiff/scripts/launch-revdiff.ps1' '--all-files' '--exclude=vendor'"
```
</concrete-examples>

<expected-monitor-event-stream>
1. First event: `[revdiff:STARTED] pane-id=<id>` — TUI launched successfully.
2. (Optional intermediate events: none under normal operation.)
3. Final event(s): annotation block (if user wrote annotations); empty if user quit without annotating.

If event 1 never arrives before Monitor exits, the launcher crashed — read the task output file for stderr. **Do not interpret missing STARTED as user-approval.**
</expected-monitor-event-stream>

<launcher-behavior>
The launcher:
- Validates `WEZTERM_PANE` env var is set (errors out if not in WezTerm)
- Spawns revdiff in a WezTerm split-pane via `wezterm cli split-pane --bottom --percent <REVDIFF_POPUP_HEIGHT:-90> --pane-id $WEZTERM_PANE`
- Emits `[revdiff:STARTED] pane-id=<id>` to stdout (Monitor event 1)
- Blocks polling a sentinel file until revdiff exits
- Streams captured annotations from the temp output file to stdout
- Exits with status 0 on success, 1 on any error
</launcher-behavior>

</step-2-launch>

<step-3-process-annotations>
Read the Monitor task's output file. Strip any line matching `^\[revdiff:` (these are launcher sentinels, not user annotations). The remaining content is the annotation block.

Format:
```
## file.go:43 (+)
use errors.Is() instead of direct comparison

## store.go:18 (-)
don't remove this validation

## docs/notes.md (file-level)
overall: rephrase the intro
```

Each block:
- `## filename:line (type)` — `(+)` = added line, `(-)` = removed line, `(file-level)` = whole-file note
- Comment text below — what the user wants changed
</step-3-process-annotations>

<step-4-plan-changes>
Enter plan mode (EnterPlanMode) to analyze annotations:
- List each annotation with file and line reference
- Describe the planned change for each
- Get user approval before modifying code
</step-4-plan-changes>

<step-5-address-annotations>
After plan approval, fix the source. Each annotation is a directive.
</step-5-address-annotations>

<step-6-loop>
Re-launch revdiff with the same arguments. User can:
- Add more annotations → back to Step 3
- Quit without annotations → review complete (only `[revdiff:STARTED]` in output)
</step-6-loop>

<step-7-done>
When the captured output contains only `[revdiff:STARTED]` and no annotation blocks, the review is complete. Inform the user.
</step-7-done>

<example-sessions>

```
User: "revdiff HEAD~1"
→ Launch via pwsh + Monitor(persistent:true)
→ Event: [revdiff:STARTED] pane-id=8
→ user annotates: "handler.go:43 - use errors.Is()"
→ user quits
→ Monitor emits annotation block, task completes
→ Enter plan mode: "add errors.Is() check at handler.go:43"
→ User approves; fix applied
→ Re-launch revdiff HEAD~1
→ Event: [revdiff:STARTED], then empty
→ "Review complete."
```

```
User: "revdiff all files exclude vendor"
→ Launch with --all-files --exclude=vendor
→ Event: [revdiff:STARTED]
→ User browses, annotates, quits
→ Annotations captured → same loop as above
```
</example-sessions>

<appendix-cross-platform>
The repository's POSIX siblings `launch-revdiff.sh` and `detect-ref.sh` exist for parity with the upstream `umputun/revdiff` plugin on macOS / Linux installs. They are not used on Windows and MUST NOT be invoked from Windows agents — they have different terminal-detection logic and do not emit the `[revdiff:STARTED]` sentinel.

The fork-only `--view=<path>` flag is implemented in `launch-revdiff.ps1` only. The bash sibling does not understand it.
</appendix-cross-platform>
