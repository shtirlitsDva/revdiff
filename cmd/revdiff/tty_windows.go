//go:build windows

package main

import "os"

// openInteractiveTTY opens the controlling terminal for interactive input.
// on Windows the equivalent of /dev/tty is the CONIN$ console device, which
// can be opened even when stdin has been redirected to a pipe (as is the case
// in --stdin mode).
//
// O_RDWR is required, not O_RDONLY. Bubble Tea calls term.MakeRaw on the
// returned handle, which on Windows invokes SetConsoleMode; that API returns
// ERROR_ACCESS_DENIED when the handle was opened read-only. Read-write is
// safe here: every process can already set its own console's mode.
func openInteractiveTTY() (*os.File, error) {
	return os.OpenFile("CONIN$", os.O_RDWR, 0)
}
