package main

import (
	"os"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
	"github.com/divijg19/Nightshade/internal/render"
)

func main() {
	obs := agent.Observation{
		Visible: []core.TileView{
			{Position: core.Position{X: -2, Y: -2}, Glyph: 'A', Visible: true},
			{Position: core.Position{X: -1, Y: -1}, Glyph: 'B', Visible: true},
			{Position: core.Position{X: 0, Y: 0}, Glyph: '.', Visible: true},
		},
		Known:    nil,
		Tick:     123,
		Position: core.Position{X: 0, Y: 0},
		Presence: []agent.PresenceCue{{Type: agent.PresenceHumanOther, Position: core.Position{X: 1, Y: 0}}},
	}
	render.RenderTo(os.Stdout, obs, 100, 3, 0, "", "An ephemeral line")
}
