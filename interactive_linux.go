//go:build linux

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

// isTerminal reports whether both stdin and stdout are TTYs.
func isTerminal() bool {
	if _, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS); err != nil {
		return false
	}
	_, err := unix.IoctlGetTermios(int(os.Stdout.Fd()), unix.TCGETS)
	return err == nil
}

// enableRawMode puts the TTY into non-canonical, no-echo mode so single key
// presses (including Ctrl+C, delivered as byte 0x03) can be read directly.
// The returned func restores the original settings.
func enableRawMode() (func(), error) {
	fd := int(os.Stdin.Fd())
	old, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, err
	}

	raw := *old
	raw.Lflag &^= unix.ECHO | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Iflag &^= unix.ICRNL | unix.IXON
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &raw); err != nil {
		return nil, err
	}

	return func() {
		unix.IoctlSetTermios(fd, unix.TCSETS, old)
	}, nil
}
