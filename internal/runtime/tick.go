package runtime

import (
	"strings"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
	"github.com/divijg19/Nightshade/internal/dungeon"
	"github.com/divijg19/Nightshade/internal/game"
	"github.com/divijg19/Nightshade/internal/util"
)

type Decisions map[string]agent.Action

func (r *Runtime) TickOnce() Decisions {
	// Advance non-agent world facts before agents observe.
	r.world.MoveMarker()
	if r.board != nil {
		r.board.Tick()
	}
	// v0.3.0 pressure loop: advance dungeon pressure before snapshots.
	// Advance each unique dungeon instance once. Collect mapping of instance->agentIDs
	instAgents := map[*dungeon.Instance][]string{}
	instAgentPositions := map[*dungeon.Instance]map[string]core.Position{}
	for _, a := range r.agents {
		if d, ok := r.dungeonByAgent[a.ID()]; ok {
			instAgents[d] = append(instAgents[d], a.ID())
			if instAgentPositions[d] == nil {
				instAgentPositions[d] = map[string]core.Position{}
			}
			wpos, _ := r.world.PositionOf(a.ID())
			instAgentPositions[d][a.ID()] = core.Position{X: wpos.X, Y: wpos.Y}
		}
	}
	for inst, agentIDs := range instAgents {
		// Determine if any occupying agent stands on the anchor
		atAnchor := false
		for _, aid := range agentIDs {
			if p, ok := instAgentPositions[inst][aid]; ok {
				if p == inst.Anchor {
					atAnchor = true
					break
				}
			}
		}
		inst.TickWithAnchor(atAnchor, r.tick)

		// Entity pipeline: perception -> movement -> effects (one action per tick)
		// Only run entities if instance still has agents bound.
		if len(agentIDs) == 0 {
			continue
		}
		// Perception: increase aggro for entities near any player
		for i := range inst.Entities {
			e := &inst.Entities[i]
			nearest := 999
			for _, aid := range agentIDs {
				if pos, ok := instAgentPositions[inst][aid]; ok {
					d := e.Pos
					dist := util.IntAbs(d.X-pos.X) + util.IntAbs(d.Y-pos.Y)
					if dist < nearest {
						nearest = dist
					}
				}
			}
			if nearest <= 3 {
				e.Aggro += 1
			}
		}
		// Movement: wraiths move every 2 ticks toward nearest player
		if r.tick%2 == 0 {
			for i := range inst.Entities {
				e := &inst.Entities[i]
				// find nearest player
				nearest := 999
				var target core.Position
				for _, aid := range agentIDs {
					if pos, ok := instAgentPositions[inst][aid]; ok {
						dist := util.IntAbs(e.Pos.X-pos.X) + util.IntAbs(e.Pos.Y-pos.Y)
						if dist < nearest {
							nearest = dist
							target = pos
						}
					}
				}
				// Move one Manhattan step toward target (prefer X then Y)
				if nearest > 0 && nearest < 999 {
					dx := target.X - e.Pos.X
					if dx != 0 {
						if dx > 0 {
							e.Pos.X++
						} else {
							e.Pos.X--
						}
					} else {
						dy := target.Y - e.Pos.Y
						if dy > 0 {
							e.Pos.Y++
						} else if dy < 0 {
							e.Pos.Y--
						}
					}
					// clamp inside inner bounds
					if e.Pos.X < 1 {
						e.Pos.X = 1
					}
					if e.Pos.Y < 1 {
						e.Pos.Y = 1
					}
					if e.Pos.X > inst.Width-2 {
						e.Pos.X = inst.Width - 2
					}
					if e.Pos.Y > inst.Height-2 {
						e.Pos.Y = inst.Height - 2
					}
				}
			}
		}
		// Effects: if any entity adjacent to a player, apply energy -=2
		for i := range inst.Entities {
			e := &inst.Entities[i]
			for _, aid := range agentIDs {
				if pos, ok := instAgentPositions[inst][aid]; ok {
					dist := util.IntAbs(e.Pos.X-pos.X) + util.IntAbs(e.Pos.Y-pos.Y)
					if dist == 1 {
						// apply energy penalty to agent
						for _, a := range r.agents {
							if a.ID() != aid {
								continue
							}
							switch at := a.(type) {
							case *agent.RemoteHuman:
								at.AdjustEnergy(-2)
							case *agent.Human:
								at.AdjustEnergy(-2)
							case *agent.Scripted:
								at.AdjustEnergy(-2)
							}
							// If agent energy <= 0, force eject collapse
							var energy int
							switch at := a.(type) {
							case *agent.RemoteHuman:
								energy = at.Energy()
							case *agent.Human:
								energy = at.Energy()
							case *agent.Scripted:
								energy = at.Energy()
							}
							if energy <= 0 {
								// schedule forced eject collapse
								if r.board != nil {
									// remove bindings
									delete(r.signalByAgent, aid)
								}
								delete(r.dungeonByAgent, aid)
								r.pendingEvents[aid] = "You collapse and are expelled."
								r.pendingEjects[aid] = "collapse"
							}
						}
					}
				}
			}
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
				// schedule one-shot event and eject flag for next snapshot
				r.pendingEvents[aid] = "The dungeon collapses and expels you."
				r.pendingEjects[aid] = "pressure"
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
		// Apply navigation distortion if agent is inside a dungeon in Dangerous band
		// and the authoritative tick meets distortion cadence (tick % 3 == 0).
		if d, ok := r.dungeonByAgent[a.ID()]; ok {
			if d.Pressure >= 11 && r.tick%3 == 0 {
				// Only rotate real MOVE actions; OBSERVE/WAIT unaffected.
				switch action {
				case agent.MOVE_N:
					action = agent.MOVE_E
				case agent.MOVE_E:
					action = agent.MOVE_S
				case agent.MOVE_S:
					action = agent.MOVE_W
				case agent.MOVE_W:
					action = agent.MOVE_N
				}
			}
		}

		// HIDE handling: server-authoritative enforcement and effects.
		if action == agent.HIDE {
			if d, ok := r.dungeonByAgent[a.ID()]; ok {
				// cannot hide in CRITICAL band
				if d.InstabilityBand() >= 3 {
					action = agent.WAIT
				} else {
					// consume 1 energy and reduce aggro deterministically
					switch at := a.(type) {
					case *agent.RemoteHuman:
						at.AdjustEnergy(-1)
					case *agent.Human:
						at.AdjustEnergy(-1)
					case *agent.Scripted:
						at.AdjustEnergy(-1)
					}
					for i := range d.Entities {
						d.Entities[i].Aggro -= 1
						if d.Entities[i].Aggro < 0 {
							d.Entities[i].Aggro = 0
						}
					}
				}
			} else {
				// HIDE outside dungeon is a no-op (treat as WAIT)
				action = agent.WAIT
			}
		}

		newPos := game.ResolveMovement(
			pos,
			action,
			r.world.Width(),
			r.world.Height(),
		)

		// If agent is inside a dungeon and attempts to move onto the exit,
		// treat that as an EXIT attempt (commitment). Enforce band/energy
		// constraints server-side.
				if d, ok := r.dungeonByAgent[a.ID()]; ok {
					np := core.Position{X: newPos.X, Y: newPos.Y}
					if np == d.Exit {
				// check agent energy via concrete types
				energy := agent.MaxEnergy
				switch at := a.(type) {
				case *agent.RemoteHuman:
					energy = at.Energy()
				case *agent.Human:
					energy = at.Energy()
				case *agent.Scripted:
					energy = at.Energy()
				}
				band := d.InstabilityBand()
				// If in CRITICAL band and exhausted (energy < 1) then exit fails.
				if band >= 3 && energy < 1 {
					// silent failure; renderer will be hinted via snapshot.BlockedActions
				} else {
					// Successful exit: reset pressure, mark instance done, remove bindings.
					d.Pressure = 0
					d.Done = true
					d.DoneReason = "exit"
					delete(r.dungeonByAgent, a.ID())
					delete(r.signalByAgent, a.ID())
					// deliver a short confirmation next snapshot (presentation-only)
					r.pendingEvents[a.ID()] = "You exit the dungeon."
				}
			}
		}
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
	// seed deterministic default entities
	inst.AddDefaultEntities()
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

    // Deliver any pending one-shot event (e.g. forced-eject) to the next snapshot.
    // Do this before mode branching so events set during TickOnce() are seen
    // even if the agent's dungeon binding was removed earlier in the tick.
	if ev, ok := r.pendingEvents[a.ID()]; ok {
		snap.Event = ev
		delete(r.pendingEvents, a.ID())
	}
	if er, ok := r.pendingEjects[a.ID()]; ok {
		snap.Ejected = true
		snap.EjectReason = er
		delete(r.pendingEjects, a.ID())
	}
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
				AtAnchor:      snap.Position == d.Anchor,
				AtExit:        snap.Position == d.Exit,
		}

			// Populate decayed tiles slice for presentation and report distortion state.
			decayed := make([]core.Position, 0, len(d.Decayed))
			for p := range d.Decayed {
				decayed = append(decayed, p)
			}
			snap.Dungeon.DecayedTiles = decayed
			snap.Dungeon.DistortionActive = d.Pressure >= 11 && r.tick%3 == 0
			labels := []string{"Stable", "Unstable", "Dangerous", "Critical"}
			if band >= 0 && band < len(labels) {
				snap.Dungeon.InstabilityLabel = labels[band]
			}

			// Provide presentation-only action cost hints and blocked-action reasons
			snap.Dungeon.ActionCosts = map[string]int{}
			if band == 1 {
				snap.Dungeon.ActionCosts["observe"] = 1
			}

			// Populate visible enemies and compute threat level.
			visMap := map[core.Position]struct{}{}
			for _, tv := range snap.Visible {
				visMap[tv.Position] = struct{}{}
			}
			maxAgg := 0
			nearest := 999
			for _, e := range d.Entities {
				if e.Aggro > maxAgg {
					maxAgg = e.Aggro
				}
				dist := util.IntAbs(e.Pos.X-snap.Position.X) + util.IntAbs(e.Pos.Y-snap.Position.Y)
				if dist < nearest {
					nearest = dist
				}
				// Only include enemy in view if visible to agent
				if _, ok := visMap[e.Pos]; ok {
					snap.Dungeon.Enemies = append(snap.Dungeon.Enemies, agent.EnemyView{Pos: e.Pos, Threat: "", Glyph: 'w'})
				}
			}
			// threatScore: combine nearest distance (inverse), maxAggro, instability band
			distScore := 0
			if nearest < 999 {
				if nearest <= 0 {
					distScore = 3
				} else {
					distScore = 4 - nearest
				}
			}
			score := distScore
			if maxAgg > score {
				score = maxAgg
			}
			if band > score {
				score = band
			}
			if score <= 1 {
				snap.Dungeon.Threat = "LOW"
			} else if score == 2 {
				snap.Dungeon.Threat = "MEDIUM"
			} else {
				snap.Dungeon.Threat = "HIGH"
			}
			if band == 3 {
				snap.Dungeon.ActionCosts["move"] = 1
			}
			// WAIT behavior in Dangerous band: no restore
			if band == 2 {
				snap.Dungeon.BlockedActions = append(snap.Dungeon.BlockedActions, "wait:norest")
			}
			// EXIT: only permitted at exit tile; also in CRITICAL requires at least 1 energy
			if !snap.Dungeon.AtExit {
				snap.Dungeon.BlockedActions = append(snap.Dungeon.BlockedActions, "exit:not_at_exit")
			} else {
				// check agent energy
				energy := agent.MaxEnergy
				switch at := a.(type) {
				case *agent.RemoteHuman:
					energy = at.Energy()
				case *agent.Human:
					energy = at.Energy()
				case *agent.Scripted:
					energy = at.Energy()
				}
				if band >= 3 && energy < 1 {
					snap.Dungeon.BlockedActions = append(snap.Dungeon.BlockedActions, "exit:exhausted")
				}
			}
			// HIDE is disabled in CRITICAL band
			if band >= 3 {
				snap.Dungeon.BlockedActions = append(snap.Dungeon.BlockedActions, "hide:disabled")
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
