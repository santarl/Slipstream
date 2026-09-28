//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// isTerminal reports whether both stdin and stdout are real consoles.
func isTerminal() bool {
	var mode uint32
	if windows.GetConsoleMode(windows.Handle(os.Stdin.Fd()), &mode) != nil {
		return false
	}
	return windows.GetConsoleMode(windows.Handle(os.Stdout.Fd()), &mode) == nil
}

// enableRawMode switches the console to unbuffered, no-echo input that
// delivers arrow keys as VT escape sequences, and enables ANSI output.
// The returned func restores the original modes.
func enableRawMode() (func(), error) {
	in := windows.Handle(os.Stdin.Fd())
	out := windows.Handle(os.Stdout.Fd())

	var inMode, outMode uint32
	if err := windows.GetConsoleMode(in, &inMode); err != nil {
		return nil, err
	}
	if err := windows.GetConsoleMode(out, &outMode); err != nil {
		return nil, err
	}

	newIn := inMode&^(windows.ENABLE_LINE_INPUT|windows.ENABLE_ECHO_INPUT|windows.ENABLE_PROCESSED_INPUT) | windows.ENABLE_VIRTUAL_TERMINAL_INPUT
	if err := windows.SetConsoleMode(in, newIn); err != nil {
		return nil, err
	}
	if err := windows.SetConsoleMode(out, outMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		windows.SetConsoleMode(in, inMode)
		return nil, err
	}

	return func() {
		windows.SetConsoleMode(in, inMode)
		windows.SetConsoleMode(out, outMode)
	}, nil
}
