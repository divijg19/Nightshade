package ui

import "sync/atomic"

type ColorLevel int

const (
	ColorNone ColorLevel = iota
	ColorANSI
	Color256
	ColorTrue
)

type PresentationOptions struct {
	ASCIIMode  bool
	ColorLevel ColorLevel
}

var presentationState atomic.Value

func init() {
	presentationState.Store(PresentationOptions{ASCIIMode: false, ColorLevel: ColorTrue})
}

func SetPresentationOptions(opts PresentationOptions) {
	if opts.ColorLevel < ColorNone || opts.ColorLevel > ColorTrue {
		opts.ColorLevel = ColorANSI
	}
	presentationState.Store(opts)
}

func currentPresentationOptions() PresentationOptions {
	v := presentationState.Load()
	if v == nil {
		return PresentationOptions{ASCIIMode: false, ColorLevel: ColorANSI}
	}
	return v.(PresentationOptions)
}

func styleDim(s string) string {
	return styleWithColor("2", "38;5;244", "38;2;160;160;160", s)
}

func styleWarn(s string) string {
	return styleWithColor("33", "38;5;220", "38;2;255;208;88", s)
}

func styleDanger(s string, bold bool) string {
	basic := "31"
	ansi256 := "38;5;196"
	trueColor := "38;2;255;96;96"
	if bold {
		basic = "1;31"
		ansi256 = "1;38;5;196"
		trueColor = "1;38;2;255;96;96"
	}
	return styleWithColor(basic, ansi256, trueColor, s)
}

func styleWithColor(basic string, ansi256 string, trueColor string, s string) string {
	if s == "" {
		return s
	}
	mode := currentPresentationOptions().ColorLevel
	if mode == ColorNone {
		return s
	}
	code := basic
	if mode == Color256 {
		code = ansi256
	}
	if mode == ColorTrue {
		code = trueColor
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}
