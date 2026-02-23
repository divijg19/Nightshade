package terminal

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

type ColorLevel int

const (
	ColorNone ColorLevel = iota
	ColorANSI
	Color256
	ColorTrue
)

type Options struct {
	ForceASCII   bool
	ForceNoColor bool
}

type Capabilities struct {
	TERM              string
	COLORTERM         string
	Width             int
	Height            int
	TrueColor         bool
	Has256Color       bool
	UnicodeSafe       bool
	AltScreenSupport  bool
	ColorDisabled     bool
	ASCIIMode         bool
	LimitedCapability bool
	ColorLevel        ColorLevel
}

func Detect(opts Options) Capabilities {
	term := strings.TrimSpace(os.Getenv("TERM"))
	colorTerm := strings.TrimSpace(os.Getenv("COLORTERM"))
	noUnicode := strings.TrimSpace(os.Getenv("NO_UNICODE")) == "1"
	noColor := strings.TrimSpace(os.Getenv("NO_COLOR")) != ""

	width, height := detectTerminalSize()
	limited := term == "" || term == "dumb"
	unicodeSafe := hasUTF8Locale() && !limited && !noUnicode
	asciiMode := opts.ForceASCII || !unicodeSafe || limited

	colorDisabled := opts.ForceNoColor || noColor || limited
	colorLevel := detectColorLevel(term, colorTerm, colorDisabled)

	caps := Capabilities{
		TERM:              term,
		COLORTERM:         colorTerm,
		Width:             width,
		Height:            height,
		TrueColor:         colorLevel == ColorTrue,
		Has256Color:       colorLevel == Color256 || colorLevel == ColorTrue,
		UnicodeSafe:       unicodeSafe,
		AltScreenSupport:  !limited,
		ColorDisabled:     colorDisabled,
		ASCIIMode:         asciiMode,
		LimitedCapability: limited,
		ColorLevel:        colorLevel,
	}

	return caps
}

func DiagnosticLines(c Capabilities) []string {
	size := "unknown"
	if c.Width > 0 && c.Height > 0 {
		size = fmt.Sprintf("%dx%d", c.Width, c.Height)
	}

	return []string{
		fmt.Sprintf("TERM: %s", emptyDefault(c.TERM, "(unset)")),
		fmt.Sprintf("COLORTERM: %s", emptyDefault(c.COLORTERM, "(unset)")),
		fmt.Sprintf("Terminal size: %s", size),
		fmt.Sprintf("Truecolor detected?: %t", c.TrueColor),
		fmt.Sprintf("256-color detected?: %t", c.Has256Color),
		fmt.Sprintf("Unicode safe?: %t", c.UnicodeSafe),
		fmt.Sprintf("Alt-screen support?: %t", c.AltScreenSupport),
		fmt.Sprintf("Color disabled?: %t", c.ColorDisabled),
		fmt.Sprintf("ASCII mode active?: %t", c.ASCIIMode),
	}
}

func detectColorLevel(term string, colorTerm string, colorDisabled bool) ColorLevel {
	if colorDisabled {
		return ColorNone
	}
	lowerTerm := strings.ToLower(term)
	lowerColorTerm := strings.ToLower(colorTerm)
	if strings.Contains(lowerColorTerm, "truecolor") || strings.Contains(lowerColorTerm, "24bit") || strings.Contains(lowerTerm, "direct") {
		return ColorTrue
	}
	if strings.Contains(lowerTerm, "256color") {
		return Color256
	}
	if lowerTerm == "" || lowerTerm == "dumb" {
		return ColorNone
	}
	return ColorANSI
}

func detectTerminalSize() (int, int) {
	fds := []uintptr{os.Stdout.Fd(), os.Stderr.Fd()}
	for _, fd := range fds {
		ws, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
		if err == nil && ws != nil && ws.Col > 0 && ws.Row > 0 {
			return int(ws.Col), int(ws.Row)
		}
	}
	return 0, 0
}

func hasUTF8Locale() bool {
	checks := []string{os.Getenv("LC_ALL"), os.Getenv("LC_CTYPE"), os.Getenv("LANG")}
	for _, value := range checks {
		if strings.Contains(strings.ToUpper(value), "UTF-8") || strings.Contains(strings.ToUpper(value), "UTF8") {
			return true
		}
	}
	return false
}

func emptyDefault(s string, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return s
}
