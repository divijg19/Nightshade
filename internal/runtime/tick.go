package runtime

import (
	"time"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
	"github.com/divijg19/Nightshade/internal/game"
)

type Decisions map[string]agent.Action

func (r *Runtime) TickOnce() Decisions {
	// Advance non-agent world facts before agents observe.
	r.world.MoveMarker()

	decisions := make(Decisions)

	// 1. Observation phase: build snapshot for each agent and deliver to
	//    RemoteHuman agents via their Observe/SendObservation channels.
	snaps := make(map[string]Snapshot)
	for _, a := range r.agents {
		preSnap := r.snapshotFor(a, agent.Action(-1))
		snaps[a.ID()] = preSnap
		if rh, ok := a.(*agent.RemoteHuman); ok {
			// Non-blocking notify the agent of the snapshot (agent will build
			// its own Observation from Memory and Snapshot).
			rh.Observe(preSnap)
		}
	}

	// 2. Input phase: collect exactly one input per connected RemoteHuman.
	//    We use a bounded timeout to avoid indefinite blocking.
	inputs := make(map[string]string)
	inputTimeout := 200 * time.Millisecond
	for _, a := range r.agents {
		if rh, ok := a.(*agent.RemoteHuman); ok {
			// Attempt to read one input for this agent with timeout.
			select {
			case in := <-rh.RecvInput:
				inputs[a.ID()] = in
			case <-time.After(inputTimeout):
				inputs[a.ID()] = ""
			}
		} else {
			inputs[a.ID()] = ""
		}
	}

	// 3. Decision phase: call Decide (or DecideWithInput for RemoteHuman)

	// Emission pass: ask each agent to publish its BeliefSignal before any
	// contagion is applied. Agents that implement EmitBeliefs will be called.
	for _, a := range r.agents {
		preSnap := snaps[a.ID()]
		if emitter, ok := a.(interface{ EmitBeliefs(agent.Snapshot) }); ok {
			emitter.EmitBeliefs(preSnap)
		}
	}

	for _, a := range r.agents {
		preSnap := snaps[a.ID()]
		var action agent.Action
		if rh, ok := a.(*agent.RemoteHuman); ok {
			action = rh.DecideWithInput(preSnap, inputs[a.ID()])
		} else {
			action = a.Decide(preSnap)
		}
		decisions[a.ID()] = action

		// 4. Resolution: apply movement results to world
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
	}

	// 5. Advance the runtime tick counter
	r.advanceTick()
	return decisions
}

func (r *Runtime) snapshotFor(a agent.Agent, action agent.Action) Snapshot {
	snap := Snapshot{
		Tick:   r.tick,
		SelfID: a.ID(),
	}

	// Use the `action` parameter to avoid unused parameter linter warnings.
	// The runtime does not change visibility based on actions (OBSERVE
	// semantics are agent-layer only), so we only reference the value
	// harmlessly here to preserve the current API.
	_ = action

	pos, ok := r.world.PositionOf(a.ID())
	if !ok {
		return snap
	}
	snap.Position = core.Position{
		X: pos.X,
		Y: pos.Y,
	}
	radius := defaultVisibilityRadius

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
