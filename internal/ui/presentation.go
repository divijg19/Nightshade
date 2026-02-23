package ui

import (
	"strings"
	"sync/atomic"
)

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

type SemanticColor int

const (
	ColorBackground SemanticColor = iota
	ColorSurface
	ColorBorder
	ColorPrimary
	ColorAccent
	ColorMuted
	ColorSuccess
	ColorWarning
	ColorDanger
	ColorCritical
	ColorHighlight
	ColorDim
)

type colorSpec struct {
	basic string
	c256  string
	true  string
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

func semanticColorSpec(token SemanticColor) colorSpec {
	switch token {
	case ColorBackground:
		return colorSpec{basic: "30", c256: "38;5;235", true: "38;2;12;12;16"}
	case ColorSurface:
		return colorSpec{basic: "37", c256: "38;5;250", true: "38;2;220;220;224"}
	case ColorBorder:
		return colorSpec{basic: "37", c256: "38;5;246", true: "38;2;124;138;164"}
	case ColorPrimary:
		return colorSpec{basic: "36", c256: "38;5;45", true: "38;2;88;224;255"}
	case ColorAccent:
		return colorSpec{basic: "35", c256: "38;5;177", true: "38;2;182;138;255"}
	case ColorMuted:
		return colorSpec{basic: "37", c256: "38;5;245", true: "38;2;150;158;170"}
	case ColorSuccess:
		return colorSpec{basic: "32", c256: "38;5;82", true: "38;2;86;232;143"}
	case ColorWarning:
		return colorSpec{basic: "33", c256: "38;5;220", true: "38;2;255;200;96"}
	case ColorDanger:
		return colorSpec{basic: "31", c256: "38;5;203", true: "38;2;255;118;118"}
	case ColorCritical:
		return colorSpec{basic: "1;31", c256: "1;38;5;196", true: "1;38;2;255;82;82"}
	case ColorHighlight:
		return colorSpec{basic: "1;36", c256: "1;38;5;51", true: "1;38;2;112;246;255"}
	case ColorDim:
		return colorSpec{basic: "2", c256: "38;5;244", true: "38;2;130;136;146"}
	default:
		return colorSpec{basic: "37", c256: "38;5;250", true: "38;2;220;220;224"}
	}
}

func styleColor(token SemanticColor, s string) string {
	if s == "" {
		return s
	}
	spec := semanticColorSpec(token)
	return styleWithColor(spec.basic, spec.c256, spec.true, s)
}

func styleBoldColor(token SemanticColor, s string) string {
	if s == "" {
		return s
	}
	spec := semanticColorSpec(token)
	return styleWithColor(withBold(spec.basic), withBold(spec.c256), withBold(spec.true), s)
}

func withBold(code string) string {
	if code == "" {
		return "1"
	}
	if strings.HasPrefix(code, "1;") || code == "1" {
		return code
	}
	return "1;" + code
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

func styleDim(s string) string {
	return styleColor(ColorDim, s)
}
