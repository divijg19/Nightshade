package ui

import "github.com/divijg19/Nightshade/internal/render"

func renderSummary(m Model) string {
	frame := render.BuildFrameWithOptions(m.obs, m.status, m.energy, 3, 0, render.Options{})
	lines := make([]string, 0, 1+len(frame.Grid)+1)
	lines = append(lines, frame.Header)
	lines = append(lines, frame.Grid...)
	if len(frame.Narration) > 0 {
		lines = append(lines, frame.Narration[0])
	}
	return joinLines(lines)
}
