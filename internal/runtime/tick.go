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

	// v0.3.1: check for forced eject (Pressure > MaxPressure) and apply effects
	for _, a := range r.agents {
		aid := a.ID()
		if d, ok := r.dungeonByAgent[aid]; ok {
			if d.Pressure >= d.MaxPressure {
				// Forced eject: burn signal, destroy dungeon, apply scar, set event
				sigID := r.signalByAgent[aid]
				if r.board != nil && sigID != "" {
					r.board.Burn(sigID)
				}
				// Apply +1 Scar to agent memory if present
				if rh, ok2 := a.(*agent.RemoteHuman); ok2 {
					mem := rh.Memory()
					wpos, _ := r.world.PositionOf(aid)
					pos := core.Position{X: wpos.X, Y: wpos.Y}
					if mt, found := mem.GetMemoryTile(pos); found {
						mt.ScarLevel++
						mem.SetMemoryTile(pos, mt)
					} else {
						// create a minimal tile with scar
						tv := core.TileView{Position: pos, Glyph: 'X', Visible: true}
						mem.SetMemoryTile(pos, agent.MemoryTile{Tile: tv, LastSeen: r.tick, ScarLevel: 1})
					}
				}
				// remove bindings
				delete(r.dungeonByAgent, aid)
				delete(r.signalByAgent, aid)
				// schedule one-shot event for next snapshot
				r.pendingEvents[aid] = "You are ejected from the dungeon!"
			}
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
		// Determine exit visibility state for this agent based on band/tick/position.
		band := d.InstabilityBand()
		ex := d.Exit
		exitVisible := true
		exitState := "visible"
		switch {
		case band >= 3:
			exitVisible = false
			exitState = "collapsed"
		case band >= 11: // unreachable, kept for clarity
			fallthrough
		default:
			// handle below
		}
		// band-specific handling
		switch band {
		case 1:
			// flicker deterministic: visible when (x+y+tick)%2==0
			if (ex.X+ex.Y+snap.Tick)%2 != 0 {
				exitVisible = false
				exitState = "flicker"
			}
		case 2:
			// visible only when adjacent to agent
			dx := snap.Position.X - ex.X
			dy := snap.Position.Y - ex.Y
			if dx < 0 {
				dx = -dx
			}
			if dy < 0 {
				dy = -dy
			}
			if dx+dy <= 1 {
				exitVisible = true
				exitState = "adjacent"
			} else {
				exitVisible = false
				exitState = "hidden"
			}
		}

		// If collapsed, ensure exit is not shown in grid.
		if !exitVisible {
			// replace exit glyph with wall for presentation
			if ex.Y >= 0 && ex.Y < len(grid) && ex.X >= 0 && ex.X < len(grid[ex.Y]) {
				grid[ex.Y][ex.X] = '#'
			}
		}

		snap.Dungeon = agent.DungeonView{
			Grid:            grid,
			Pressure:        d.Pressure,
			MaxPressure:     d.MaxPressure,
			Tick:            snap.Tick,
			InstabilityBand: band,
			ExitState:       exitState,
			// Keep legacy fields populated for forward compatibility.
			ExitStability: d.ExitStability,
			AnchorType:    string(d.AnchorType),
			AtAnchor:      false,
			AtExit:        false,
		}
		// One-shot narration events per band
		if _, ok := r.dungeonNarration[a.ID()]; !ok {
			r.dungeonNarration[a.ID()] = map[string]bool{}
		}
		if band == 1 && !r.dungeonNarration[a.ID()]["unstable"] {
			r.dungeonNarration[a.ID()]["unstable"] = true
			snap.Dungeon.Event = "The air feels unstable."
		}
		if band == 2 && !r.dungeonNarration[a.ID()]["dangerous"] {
			r.dungeonNarration[a.ID()]["dangerous"] = true
			snap.Dungeon.Event = "The dungeon resists your presence."
		}
		if band == 3 && !r.dungeonNarration[a.ID()]["collapse_warning"] {
			r.dungeonNarration[a.ID()]["collapse_warning"] = true
			snap.Dungeon.Event = "The exit shows signs of collapse."
		}
		// propagate any pending global event (forced eject) into snapshot as event on board mode later
		if ev, ok := r.pendingEvents[a.ID()]; ok {
			snap.Event = ev
			delete(r.pendingEvents, a.ID())
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
				Burned:   s.Burned,
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
