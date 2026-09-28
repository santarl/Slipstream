// interactive.go
//
// Optional interactive launch-mode picker (-i / --interactive), styled after
// the Select prompt from charmbracelet/huh.
//
// This lives in its own files (interactive*.go) on purpose: the only hooks in
// main.go are the interactiveArgs call and passing its result to launchGame,
// so merging from upstream stays painless. Raw-terminal handling is in the
// per-OS files (interactive_windows.go, interactive_linux.go,
// interactive_other.go).

package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

var errPickCancelled = errors.New("selection cancelled")

// Order matters: index 0 is online, index 1 is offline (-noeac).
var launchModes = []string{
	"Online (Easy Anti-Cheat on)",
	"Offline (-noeac, Easy Anti-Cheat off)",
}

func isInteractiveFlag(arg string) bool {
	a := strings.ToLower(arg)
	return a == "-i" || a == "--interactive"
}

func isNoEACFlag(arg string) bool {
	return strings.ToLower(arg) == "-noeac"
}

// interactiveArgs looks for -i / --interactive in args. If it is absent, args
// is returned untouched. If present, the flag is removed, the user picks
// Online or Offline in the terminal, and "-noeac" is added or removed
// accordingly. The result is meant to be handed straight to launchGame.
func interactiveArgs(cfg Config, args []string) []string {
	requested := false
	for _, a := range args {
		if isInteractiveFlag(a) {
			requested = true
			break
		}
	}
	if !requested {
		return args
	}

	rest := make([]string, 0, len(args))
	hasNoEAC := false
	for _, a := range args {
		if isInteractiveFlag(a) {
			continue
		}
		if isNoEACFlag(a) {
			hasNoEAC = true
		}
		rest = append(rest, a)
	}

	if cfg.BakkesModEnabled {
		log.Println("Interactive mode: BakkesMod is enabled, so the game will launch offline (no EAC). Skipping picker.")
		return rest
	}
	if !isTerminal() {
		log.Println("Interactive mode requested but no terminal is attached. Skipping picker.")
		return rest
	}

	def := 0
	if hasNoEAC {
		def = 1 // the user already asked for -noeac, so start on Offline
	}

	choice, err := runPicker("Launch mode", launchModes, def)
	if err != nil {
		fmt.Println("Cancelled.")
		os.Exit(130)
	}
	log.Printf("Interactive mode: selected %q", launchModes[choice])

	// Normalise: drop any existing -noeac, then re-add it if Offline was chosen.
	out := make([]string, 0, len(rest)+1)
	for _, a := range rest {
		if !isNoEACFlag(a) {
			out = append(out, a)
		}
	}
	if choice == 1 {
		out = append(out, "-noeac")
	}
	return out
}

// --- Rendering (mirrors huh's Select: indigo title, fuchsia "> " cursor,
// grey left bar, grey help line) ---

type pickPalette struct{ color bool }

func newPickPalette() pickPalette {
	_, noColor := os.LookupEnv("NO_COLOR")
	return pickPalette{color: !noColor}
}

func (p pickPalette) paint(code, s string) string {
	if !p.color {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (p pickPalette) title(s string) string  { return p.paint("1;38;5;105", s) }
func (p pickPalette) accent(s string) string { return p.paint("38;5;212", s) }
func (p pickPalette) bar(s string) string    { return p.paint("38;5;238", s) }
func (p pickPalette) muted(s string) string  { return p.paint("38;5;241", s) }

func renderPicker(p pickPalette, title string, opts []string, cur int) []string {
	bar := p.bar("┃")
	lines := []string{bar + " " + p.title(title)}
	for i, o := range opts {
		if i == cur {
			lines = append(lines, bar+" "+p.accent("> "+o))
		} else {
			lines = append(lines, bar+"   "+o)
		}
	}
	lines = append(lines, "", "  "+p.muted("↑ up • ↓ down • enter submit • esc cancel"))
	return lines
}

// --- Input handling ---

type pickKey int

const (
	pickNone pickKey = iota
	pickUp
	pickDown
	pickEnter
	pickCancel
)

// parsePickKey decodes one read() worth of terminal input.
func parsePickKey(b []byte) pickKey {
	if len(b) == 0 {
		return pickNone
	}
	if b[0] == 0x1b {
		if len(b) == 1 {
			return pickCancel // bare Esc
		}
		if len(b) >= 3 && (b[1] == '[' || b[1] == 'O') {
			switch b[len(b)-1] {
			case 'A':
				return pickUp
			case 'B':
				return pickDown
			}
		}
		return pickNone
	}
	switch b[0] {
	case '\r', '\n':
		return pickEnter
	case 0x03, 'q': // Ctrl+C, q
		return pickCancel
	case 'k', 0x10: // k, Ctrl+P
		return pickUp
	case 'j', 0x0e: // j, Ctrl+N
		return pickDown
	}
	return pickNone
}

// runPicker shows the menu and returns the chosen index. If raw terminal mode
// is unavailable (e.g. some Wine/Proton consoles), it falls back to a plain
// numbered prompt.
func runPicker(title string, opts []string, cur int) (int, error) {
	restore, err := enableRawMode()
	if err != nil {
		log.Printf("Raw terminal mode unavailable (%v); using plain prompt.", err)
		return linePrompt(title, opts, cur)
	}
	defer restore()

	out := os.Stdout
	p := newPickPalette()
	height := len(opts) + 3 // title + options + blank + help

	fmt.Fprint(out, "\x1b[?25l") // hide cursor
	defer fmt.Fprint(out, "\x1b[?25h")

	draw := func(first bool) {
		if !first {
			fmt.Fprintf(out, "\r\x1b[%dA\x1b[J", height-1)
		}
		fmt.Fprint(out, strings.Join(renderPicker(p, title, opts, cur), "\r\n"))
	}
	erase := func() {
		fmt.Fprintf(out, "\r\x1b[%dA\x1b[J", height-1)
	}

	draw(true)
	buf := make([]byte, 16)
	for {
		n, readErr := os.Stdin.Read(buf)
		if readErr != nil {
			erase()
			return 0, readErr
		}
		switch parsePickKey(buf[:n]) {
		case pickUp:
			cur = (cur - 1 + len(opts)) % len(opts)
			draw(false)
		case pickDown:
			cur = (cur + 1) % len(opts)
			draw(false)
		case pickEnter:
			erase()
			fmt.Fprintf(out, "%s %s\r\n", p.title(title+":"), p.accent(opts[cur]))
			return cur, nil
		case pickCancel:
			erase()
			return 0, errPickCancelled
		}
	}
}

// linePrompt is the no-frills fallback: "1) ... 2) ..." and type a number.
func linePrompt(title string, opts []string, def int) (int, error) {
	fmt.Println(title)
	for i, o := range opts {
		fmt.Printf("  %d) %s\n", i+1, o)
	}
	r := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Choose 1-%d [%d]: ", len(opts), def+1)
		line, err := r.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" && err == nil {
			return def, nil
		}
		if n, convErr := strconv.Atoi(line); convErr == nil && n >= 1 && n <= len(opts) {
			return n - 1, nil
		}
		if err != nil {
			return 0, errPickCancelled
		}
	}
}
