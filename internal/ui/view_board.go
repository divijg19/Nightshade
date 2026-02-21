package ui

import (
	"strings"

	"github.com/divijg19/Nightshade/internal/render"
)

func renderBoard(m Model) string {
	frame := render.BuildFrameWithOptions(m.obs, m.status, m.energy, 3, 0, render.Options{})
	lines := make([]string, 0, 2+len(frame.Grid)+2)
	lines = append(lines, frame.Header)
	lines = append(lines, strings.Repeat("-", 34))
	lines = append(lines, frame.Grid...)
	if len(frame.Narration) > 0 {
		lines = append(lines, frame.Narration[0])
	}
	return joinLines(lines)
}
