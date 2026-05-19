@echo off
REM Thin cmd.exe shim around launch-revdiff.ps1.
REM
REM Why this exists:
REM   Calling `pwsh -File launch-revdiff.ps1 --compare-old=C:\path` silently
REM   splits the arg at the colon — pwsh's -File parameter parser treats
REM   `name:value` as PowerShell's switch-style notation, so the launcher
REM   receives `--compare-old=C` and `\path` as two separate args. Every
REM   Windows absolute-path flag (--compare-old, --compare-new, --only,
REM   --annotations, --description-file, --config, --keys, --history-dir,
REM   --view) hits this bug.
REM
REM `pwsh -Command "& '<script>' <args>"` does not have this bug: PowerShell
REM tokenizes -Command's value as a script block, where bareword arguments
REM keep their colons intact. cmd.exe's `%*` preserves the caller's argv as
REM a single string (quotes included), so we forward verbatim.
REM
REM Limitation: cmd.exe's `%*` is a literal byte-string, not an argv vector.
REM Args containing both spaces AND embedded double quotes round-trip with
REM mangled quoting. For free-text values (--description="hello world"),
REM use --description-file=<path> instead — paths under $env:TEMP have no
REM spaces and round-trip cleanly.
pwsh -NoProfile -ExecutionPolicy Bypass -Command "& '%~dp0launch-revdiff.ps1' %*"
