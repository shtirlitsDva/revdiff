package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/umputun/revdiff/app/diff"
	"github.com/umputun/revdiff/app/ui"
)

const defaultScratchBufferName = "scratch-buffer"

type stdinStat interface {
	Stat() (os.FileInfo, error)
}

func validateStdinFlags(opts options) error {
	if opts.StdinName != "" && !opts.Stdin {
		return errors.New("--stdin-name requires --stdin")
	}
	if !opts.Stdin {
		return nil
	}
	if opts.Refs.Base != "" || opts.Refs.Against != "" {
		return errors.New("--stdin cannot be used with refs")
	}
	if opts.Staged {
		return errors.New("--stdin cannot be used with --staged")
	}
	if len(opts.Only) > 0 {
		return errors.New("--stdin cannot be used with --only")
	}
	if opts.AllFiles {
		return errors.New("--stdin cannot be used with --all-files")
	}
	if len(opts.Exclude) > 0 {
		return errors.New("--stdin cannot be used with --exclude")
	}
	if len(opts.Include) > 0 {
		return errors.New("--stdin cannot be used with --include")
	}
	if opts.Annotations != "" {
		return errors.New("--stdin cannot be used with --annotations")
	}
	return nil
}

func validateStdinInput(opts options, stdin stdinStat) error {
	if !opts.Stdin {
		return nil
	}
	info, err := stdin.Stat()
	if err != nil {
		return fmt.Errorf("stat stdin: %w", err)
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return errors.New("--stdin requires piped or redirected input")
	}
	return nil
}

func stdinName(name string) string {
	if name == "" {
		return defaultScratchBufferName
	}
	return name
}

// openTTY opens the controlling terminal for interactive input.
// This fork is Windows-only — `CONIN$` is the Windows console device, which
// stays openable even when stdin has been redirected to a pipe (as in
// `--stdin` mode).
//
// O_RDWR is required, not O_RDONLY. Bubble Tea calls term.MakeRaw on the
// returned handle, which on Windows invokes SetConsoleMode; that API returns
// ERROR_ACCESS_DENIED when the handle was opened read-only. Read-write is
// safe here: every process can already set its own console's mode.
func openTTY() (*os.File, error) {
	tty, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open CONIN$: %w", err)
	}
	return tty, nil
}

func prepareStdinMode(opts options, stdin *os.File) (ui.Renderer, *os.File, error) {
	if err := validateStdinInput(opts, stdin); err != nil {
		return nil, nil, err
	}

	renderer, err := diff.NewStdinReaderFromReader(stdinName(opts.StdinName), stdin)
	if err != nil {
		return nil, nil, fmt.Errorf("read stdin: %w", err)
	}

	tty, err := openTTY()
	if err != nil {
		return nil, nil, err
	}
	return renderer, tty, nil
}
