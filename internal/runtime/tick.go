package runtime

import (
	"strings"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
	"github.com/divijg19/Nightshade/internal/dungeon"
	"github.com/divijg19/Nightshade/internal/game"
)

type Decisions map[string]agent.Action

func (r *Runtime) TickOnce() Decisions {
	// Advance non-agent world facts before agents observe.
	r.world.MoveMarker()
	if r.board != nil {
		r.board.Tick()
	}
	// v0.3.0 pressure loop: advance dungeon pressure before snapshots.
	for _, a := range r.agents {
		if d, ok := r.dungeonByAgent[a.ID()]; ok {
			d.Tick()
		}
	}

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

	// 2. Input phase: collect exactly one input per RemoteHuman.
	//    Block until each RemoteHuman provides an input to enforce strict
	//    turn-based semantics. This intentionally blocks the runtime until
	//    clients respond (or tests provide the inputs programmatically).
	inputs := make(map[string]string)
	for _, a := range r.agents {
		if rh, ok := a.(*agent.RemoteHuman); ok {
			in := <-rh.RecvInput
			// v0.3.0: server-authoritative dungeon commitment command.
			if signalID, ok2 := parseEnterSignal(in); ok2 {
				_ = r.tryEnterSignal(a.ID(), signalID)
				// Do not feed this special command into the cognition key parser.
				inputs[a.ID()] = ""
			} else {
				inputs[a.ID()] = in
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

func parseEnterSignal(in string) (string, bool) {
	in = strings.TrimSpace(in)
	if in == "" {
		return "", false
	}
	const prefix = "ENTER_SIGNAL "
	if !strings.HasPrefix(in, prefix) {
		return "", false
	}
	id := strings.TrimSpace(strings.TrimPrefix(in, prefix))
	if id == "" {
		return "", false
	}
	return id, true
}

func (r *Runtime) tryEnterSignal(agentID string, signalID string) bool {
	// One agent -> one dungeon.
	if _, exists := r.dungeonByAgent[agentID]; exists {
		return false
	}
	if r.board == nil {
		return false
	}

	s, ok := r.board.Find(signalID)
	if !ok {
		return false
	}
	// Not decayed.
	if s.DecayTicks <= 0 {
		return false
	}
	// Not locked by another agent.
	if s.LockedBy != "" && s.LockedBy != agentID {
		return false
	}
	// Lock immediately.
	if !r.board.Lock(signalID, agentID) {
		return false
	}

	inst := dungeon.NewInstance("D-"+signalID, dungeon.AnchorType(string(s.Anchor)))
	r.dungeonByAgent[agentID] = inst
	r.signalByAgent[agentID] = signalID
	return true
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

	// v0.3.0: Snapshot mode branching.
	if d, ok := r.dungeonByAgent[a.ID()]; ok {
		snap.Mode = "dungeon"
		grid := make([][]rune, d.Height)
		for y := 0; y < d.Height; y++ {
			row := make([]rune, d.Width)
			for x := 0; x < d.Width; x++ {
				row[x] = d.GlyphAt(core.Position{X: x, Y: y})
			}
			grid[y] = row
		}
		snap.Dungeon = agent.DungeonView{
			Grid:            grid,
			Pressure:        d.Pressure,
			MaxPressure:     d.MaxPressure,
			Tick:            snap.Tick,
			InstabilityBand: d.InstabilityBand(),
			// Keep legacy fields populated for forward compatibility.
			ExitStability: d.ExitStability,
			AnchorType:    string(d.AnchorType),
			AtAnchor:      false,
			AtExit:        false,
		}
		return snap
	}

	// Overworld -> Signal Board snapshot.
	if r.board != nil {
		snap.Mode = "board"
		cursor := r.boardCursor[a.ID()]
		sigs := r.board.Signals()
		views := make([]agent.SignalView, 0, len(sigs))
		for _, s := range sigs {
			views = append(views, agent.SignalView{
				ID:       s.ID,
				Type:     string(s.Type),
				Anchor:   string(s.Anchor),
				Zone:     string(s.Zone),
				Presence: string(s.Presence),
				Decay:    s.DecayTicks,
				Locked:   s.LockedBy != "" && s.LockedBy != a.ID(),
			})
		}
		snap.Board = agent.BoardView{Cursor: cursor, Signals: views}
	}
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
