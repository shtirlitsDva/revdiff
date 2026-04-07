//go:build windows

package main

import "os"

// openInteractiveTTY opens the controlling terminal for interactive input.
// on Windows the equivalent of /dev/tty is the CONIN$ console device, which
// can be opened even when stdin has been redirected to a pipe (as is the case
// in --stdin mode).
func openInteractiveTTY() (*os.File, error) {
	return os.Open("CONIN$")
}
