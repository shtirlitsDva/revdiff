#!/usr/bin/env python3
"""plan-review-hook.py - PreToolUse hook for ExitPlanMode.

intercepts ExitPlanMode and opens plan for user review in revdiff TUI.
if revdiff is not installed, passes through to normal confirmation.

hook receives JSON on stdin with the plan content in tool_input.plan field.
returns PreToolUse hook JSON response with permissionDecision:
  - "ask"  → no changes/annotations, proceed to normal confirmation
  - "deny" → feedback found, sent as denial reason

requirements:
  - revdiff binary in PATH
  - tmux, kitty, wezterm, cmux, or ghostty (macOS) terminal
    (on Windows: WezTerm only, via launch-plan-review.ps1)
"""

import json
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path


def read_plan_from_stdin() -> str:
    """read plan content from hook event JSON on stdin."""
    raw = sys.stdin.read()
    if not raw.strip():
        return ""
    try:
        event = json.loads(raw)
        return event.get("tool_input", {}).get("plan", "")
    except json.JSONDecodeError:
        return ""


def make_response(decision: str, reason: str = "") -> None:
    """output PreToolUse hook response and exit with appropriate code.
    deny: plain text to stderr + exit 2 (Claude Code blocks the tool and shows the text).
    ask/allow: JSON to stdout + exit 0."""
    if decision == "deny":
        print(reason, file=sys.stderr)
        sys.exit(2)
    resp: dict = {
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": decision,
        }
    }
    if reason:
        resp["hookSpecificOutput"]["permissionDecisionReason"] = reason
    print(json.dumps(resp, indent=2))


def main() -> None:
    plan_content = read_plan_from_stdin()
    if not plan_content:
        make_response("ask", "no plan content in hook event")
        return

    plugin_root = os.environ.get("CLAUDE_PLUGIN_ROOT", "")
    if not plugin_root:
        make_response("ask", "CLAUDE_PLUGIN_ROOT not set")
        return

    if not shutil.which("revdiff"):
        make_response("ask", "revdiff not installed, skipping plan review")
        return

    # Platform dispatch: on Windows use the PowerShell sibling launch-plan-review.ps1
    # (WezTerm only). On macOS/Linux the bash launcher remains the documented default
    # and keeps full multi-terminal support (tmux/kitty/wezterm/cmux/ghostty/iTerm2/
    # Emacs vterm). The two scripts accept the same positional argument signature:
    # a single plan-file path.
    scripts_dir = Path(plugin_root) / "scripts"
    if sys.platform == "win32":
        launcher = scripts_dir / "launch-plan-review.ps1"
        if not launcher.exists():
            make_response("ask", "launch-plan-review.ps1 not found")
            return
        # Prefer pwsh (PowerShell 7+) then fall back to Windows PowerShell 5.1.
        pwsh_bin = shutil.which("pwsh") or shutil.which("powershell")
        if not pwsh_bin:
            make_response("ask", "PowerShell (pwsh or powershell) not found in PATH")
            return
        launcher_cmd = [
            pwsh_bin,
            "-NoProfile",
            "-ExecutionPolicy", "Bypass",
            "-File", str(launcher),
        ]
    else:
        launcher = scripts_dir / "launch-plan-review.sh"
        if not launcher.exists():
            make_response("ask", "launch-plan-review.sh not found")
            return
        launcher_cmd = [str(launcher)]

    # write plan to temp file
    with tempfile.NamedTemporaryFile(
        mode="w", suffix=".md", prefix="plan-review-", delete=False
    ) as tmp:
        tmp.write(plan_content)
        tmp_path = Path(tmp.name)

    try:
        result = subprocess.run(
            [*launcher_cmd, str(tmp_path)],
            capture_output=True, text=True, timeout=345600,
            env={**os.environ},
        )
        annotations = result.stdout.strip()
        if not annotations:
            make_response("ask", "plan reviewed, no annotations")
        else:
            make_response(
                "deny",
                "user reviewed the plan in revdiff and added annotations. "
                "each annotation references a specific line and contains the user's feedback.\n\n"
                f"{annotations}\n\n"
                "adjust the plan to address each annotation, then call ExitPlanMode again.",
            )
    finally:
        tmp_path.unlink(missing_ok=True)


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\r\033[K", end="")
        sys.exit(130)
