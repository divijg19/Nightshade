package ui

import (
	"strings"

	"github.com/divijg19/Nightshade/internal/render"
)

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func renderDungeon(m Model) string {
	frame := render.BuildFrameWithOptions(m.obs, m.status, m.energy, 3, 0, render.Options{})
	lines := make([]string, 0, 1+1+len(frame.Status)+1+len(frame.Grid)+1+1)
	lines = append(lines, frame.Header)
	lines = append(lines, strings.Repeat("-", 34))
	for i, s := range frame.Status {
		if i >= 3 {
			break
		}
		lines = append(lines, s)
	}
	lines = append(lines, strings.Repeat("-", 34))
	lines = append(lines, frame.Grid...)
	lines = append(lines, strings.Repeat("-", 34))
	if len(frame.Narration) > 0 {
		lines = append(lines, frame.Narration[0])
	}
	return joinLines(lines)
}
