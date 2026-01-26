package runtime

import (
	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
	"github.com/divijg19/Nightshade/internal/game"
)

type Decisions map[string]agent.Action

func (r *Runtime) TickOnce() Decisions {
	// Advance non-agent world facts before agents observe.
	r.world.MoveMarker()

	decisions := make(Decisions)

	for _, a := range r.agents {
		// 1. Pre-action observation
		preSnap := r.snapshotFor(a, agent.Action(-1))

		// 2. Decide
		action := a.Decide(preSnap)
		decisions[a.ID()] = action

		// 3. Apply movement
		pos, ok := r.world.PositionOf(a.ID())
		if !ok {
			continue
		}
		newPos := game.ResolveMovement(
			pos,
			action,
			r.world.Width(),
			r.world.Height(),
		)
		r.world.SetPosition(a.ID(), newPos)

		// 4. Post-action observation (THIS WAS MISSING)
		_ = r.snapshotFor(a, action)
	}

	r.advanceTick()
	return decisions
}

func (r *Runtime) snapshotFor(a agent.Agent, action agent.Action) Snapshot {
	snap := Snapshot{
		Tick:   r.tick,
		SelfID: a.ID(),
	}

	pos, ok := r.world.PositionOf(a.ID())
	if !ok {
		return snap
	}
	snap.Position = core.Position{
		X: pos.X,
		Y: pos.Y,
	}
	radius := defaultVisibilityRadius
	if action == agent.Action(agent.OBSERVE) {
		radius = defaultVisibilityRadius * 2
	}

	markerPos := r.world.MarkerPosition()
	snap.Visible = computeVisibleTiles(
		pos.X,
		pos.Y,
		r.world.Width(),
		r.world.Height(),
		radius,
		markerPos.X,
		markerPos.Y,
	)
	// Do NOT populate snap.Known here. Known is the agent's interpretation
	// (belief) and must be maintained by the agent's Memory. Runtime reports
	// only current visibility in Snapshot.Visible.
	return snap
}

func computeVisibleTiles(
	ax, ay int,
	worldWidth, worldHeight int,
	radius int,
	markerX, markerY int,
) []core.TileView {
	tiles := []core.TileView{}

	for dx := -radius; dx <= radius; dx++ {
		for dy := -radius; dy <= radius; dy++ {
			x := ax + dx
			y := ay + dy

			if x < 0 || y < 0 || x >= worldWidth || y >= worldHeight {
				continue
			}

			glyph := rune(0)
			// Reveal marker if within visibility by comparing to
			// the passed-in marker coordinates.
			if markerX == x && markerY == y {
				glyph = 'M'
			}
			tiles = append(tiles, core.TileView{
				Position: core.Position{X: x, Y: y},
				Glyph:    glyph,
				Visible:  true,
			})

		}
	}

	return tiles
}
