package game

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/world"
)

func TestResolveMovement_Basic(t *testing.T) {
	cases := []struct {
		name   string
		pos    world.Position
		action agent.Action
		want   world.Position
	}{
		{"north within bounds", world.Position{X: 1, Y: 1}, agent.MOVE_N, world.Position{X: 1, Y: 0}},
		{"north out of bounds", world.Position{X: 0, Y: 0}, agent.MOVE_N, world.Position{X: 0, Y: 0}},
		{"east within bounds", world.Position{X: 1, Y: 1}, agent.MOVE_E, world.Position{X: 2, Y: 1}},
		{"east out of bounds", world.Position{X: world.Width - 1, Y: 0}, agent.MOVE_E, world.Position{X: world.Width - 1, Y: 0}},
		{"west within bounds", world.Position{X: 5, Y: 5}, agent.MOVE_W, world.Position{X: 4, Y: 5}},
		{"south within bounds", world.Position{X: 5, Y: 5}, agent.MOVE_S, world.Position{X: 5, Y: 6}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ResolveMovement(c.pos, c.action, world.Width, world.Height)
			if got != c.want {
				t.Fatalf("ResolveMovement(%+v,%v) = %+v, want %+v", c.pos, c.action, got, c.want)
			}
		})
	}
}
