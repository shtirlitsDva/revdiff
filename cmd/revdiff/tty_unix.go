//go:build !windows

package main

import "os"

// openInteractiveTTY opens the controlling terminal for interactive input.
// on Unix-like systems this is /dev/tty, which remains addressable even when
// stdin has been redirected to a pipe (as is the case in --stdin mode).
func openInteractiveTTY() (*os.File, error) {
	return os.Open("/dev/tty")
}
