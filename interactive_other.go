//go:build !linux && !windows

package main

import (
	"errors"
	"os"
)

// isTerminal is a portable approximation: stdin and stdout are character devices.
func isTerminal() bool {
	in, err := os.Stdin.Stat()
	if err != nil || in.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	out, err := os.Stdout.Stat()
	return err == nil && out.Mode()&os.ModeCharDevice != 0
}

// enableRawMode is not implemented here, so runPicker uses the numbered prompt.
func enableRawMode() (func(), error) {
	return nil, errors.New("raw mode not supported on this platform")
}
