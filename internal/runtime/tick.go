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
	// v0.3.5: decrement dungeon-action cooldowns (server-authoritative).
	for aid, v := range r.hideCooldown {
		if v > 0 {
			r.hideCooldown[aid] = v - 1
		}
	}
	for aid, v := range r.distractCooldown {
		if v > 0 {
			r.distractCooldown[aid] = v - 1
		}
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
		// v0.3.5: Enemies act once per tick with deterministic targeting.
		occupied := map[core.Position]struct{}{}
		for _, aid := range agentIDs {
			if p, ok := instAgentPositions[inst][aid]; ok {
				occupied[p] = struct{}{}
			}
		}
		for i := range inst.Entities {
			e := &inst.Entities[i]

			// Override timing: decrement at tick start so overrides last exactly N ticks.
			if e.OverrideTicks > 0 {
				e.OverrideTicks--
				if e.OverrideTicks == 0 {
					e.TargetOverride = ""
				}
			}

			// Determine nearest visible player (distance-based visibility).
			nearestVisible := 999
			var nearestVisiblePos core.Position
			nearestAny := 999
			for _, aid := range agentIDs {
				pos, ok := instAgentPositions[inst][aid]
				if !ok {
					continue
				}
				dist := util.IntAbs(e.Pos.X-pos.X) + util.IntAbs(e.Pos.Y-pos.Y)
				if dist < nearestAny {
					nearestAny = dist
				}
				if dist <= defaultVisibilityRadius {
					if dist < nearestVisible {
						nearestVisible = dist
						nearestVisiblePos = pos
					}
				}
			}
			// Preserve prior deterministic aggro mechanic.
			if nearestAny <= 3 {
				e.Aggro += 1
			}
			// Update last-known when a player is visible.
			if nearestVisible != 999 {
				e.LastKnown = nearestVisiblePos
				e.HasLastKnown = true
			}

			// Choose target by override or default priority.
			halt := false
			target := core.Position{}
			if e.OverrideTicks > 0 && e.TargetOverride != "" {
				switch e.TargetOverride {
				case "unknown":
					halt = true
				case "anchor":
					target = inst.Anchor
				case "exit":
					target = inst.Exit
				}
			} else {
				if nearestVisible != 999 {
					target = nearestVisiblePos
				} else if e.HasLastKnown {
					target = e.LastKnown
				} else {
					target = inst.Anchor
				}
			}

			// Move one tile per tick, cardinal, cannot pass walls.
			if !halt {
				dx := target.X - e.Pos.X
				dy := target.Y - e.Pos.Y
				moved := false
				if dx != 0 {
					step := 1
					if dx < 0 {
						step = -1
					}
					cand := core.Position{X: e.Pos.X + step, Y: e.Pos.Y}
					if inst.InBounds(cand) && inst.GlyphAt(cand) != '#' {
						if _, occ := occupied[cand]; !occ {
							e.Pos = cand
							moved = true
						}
					}
				}
				if !moved && dy != 0 {
					step := 1
					if dy < 0 {
						step = -1
					}
					cand := core.Position{X: e.Pos.X, Y: e.Pos.Y + step}
					if inst.InBounds(cand) && inst.GlyphAt(cand) != '#' {
						if _, occ := occupied[cand]; !occ {
							e.Pos = cand
						}
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
		authoritative := action

		// 4. Resolution: apply movement results to world
		pos, ok := r.world.PositionOf(a.ID())
		if !ok {
			continue
		}

		// v0.3.5: band-aware legality enforcement (server-authoritative).
		inDungeon := false
		band := 0
		var inst *dungeon.Instance
		if d, ok := r.dungeonByAgent[a.ID()]; ok {
			inDungeon = true
			band = d.InstabilityBand()
			inst = d
		}
		if inDungeon {
			// Action availability by instability band.
			if band >= 3 {
				if authoritative == agent.OBSERVE {
					authoritative = agent.WAIT
				}
				if authoritative == agent.HIDE {
					authoritative = agent.WAIT
				}
				if authoritative == agent.DISTRACT {
					authoritative = agent.WAIT
				}
			}
			if band >= 2 {
				if authoritative == agent.DISTRACT {
					authoritative = agent.WAIT
				}
			}
		} else {
			// Dungeon-only actions are invalid outside dungeons.
			if authoritative == agent.HIDE || authoritative == agent.DISTRACT {
				authoritative = agent.WAIT
			}
		}
		// Apply navigation distortion if agent is inside a dungeon in Dangerous band
		// and the authoritative tick meets distortion cadence (tick % 3 == 0).
		if d, ok := r.dungeonByAgent[a.ID()]; ok {
			if d.Pressure >= 11 && r.tick%3 == 0 {
				// Only rotate real MOVE actions; OBSERVE/WAIT unaffected.
				switch authoritative {
				case agent.MOVE_N:
					authoritative = agent.MOVE_E
				case agent.MOVE_E:
					authoritative = agent.MOVE_S
				case agent.MOVE_S:
					authoritative = agent.MOVE_W
				case agent.MOVE_W:
					authoritative = agent.MOVE_N
				}
			}
		}
		// v0.3.5: HIDE (dungeon-only)
		if authoritative == agent.HIDE {
			if !inDungeon || inst == nil || band >= 3 {
				authoritative = agent.WAIT
			} else {
				ap := core.Position{X: pos.X, Y: pos.Y}
				if inst.GlyphAt(ap) == '#' {
					authoritative = agent.WAIT
				} else if r.hideCooldown[a.ID()] > 0 {
					authoritative = agent.WAIT
				} else {
					// set to 2 because cooldowns decrement at tick start
					r.hideCooldown[a.ID()] = 2
					for i := range inst.Entities {
						inst.Entities[i].TargetOverride = "unknown"
						// +1 because overrides decrement at tick start.
						inst.Entities[i].OverrideTicks = 2
					}
					r.pendingEvents[a.ID()] = "You fade from immediate pursuit."
				}
			}
		}
		// v0.3.5: DISTRACT (dungeon-only)
		if authoritative == agent.DISTRACT {
			if !inDungeon || inst == nil || band >= 2 {
				authoritative = agent.WAIT
			} else if len(inst.Entities) == 0 {
				// fails silently
				authoritative = agent.WAIT
			} else {
				anchorVisible := false
				for _, tv := range preSnap.Visible {
					if tv.Position == inst.Anchor {
						anchorVisible = true
						break
					}
				}
				ap := core.Position{X: pos.X, Y: pos.Y}
				nearest := 999
				idx := -1
				for i := range inst.Entities {
					e := &inst.Entities[i]
					dist := util.IntAbs(e.Pos.X-ap.X) + util.IntAbs(e.Pos.Y-ap.Y)
					if dist < nearest {
						nearest = dist
						idx = i
					}
				}
				if idx >= 0 {
					if anchorVisible {
						inst.Entities[idx].TargetOverride = "anchor"
					} else {
						inst.Entities[idx].TargetOverride = "exit"
					}
					// +1 because overrides decrement at tick start.
					inst.Entities[idx].OverrideTicks = 3
					r.pendingEvents[a.ID()] = "The presence shifts its attention."
				}
			}
		}

		// v0.3.5: server-authoritative energy costs (adjust difference vs agent mirroring).
		if inDungeon {
			agentDelta := dungeonEnergyDelta(action, band)
			authDelta := dungeonEnergyDelta(authoritative, band)
			applyEnergyDelta(a, authDelta-agentDelta)
		}

		decisions[a.ID()] = authoritative

		newPos := game.ResolveMovement(
			pos,
			authoritative,
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
		// MOVE costs more in UNSTABLE/DANGEROUS/CRITICAL.
		if band >= 1 {
			snap.Dungeon.ActionCosts["move"] = 1
		}
		// OBSERVE costs more in UNSTABLE/DANGEROUS; blocked in CRITICAL.
		if band == 1 || band == 2 {
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
			// Determine the current target tag for presentation.
			targetTag := "unknown"
			if e.OverrideTicks > 0 && e.TargetOverride != "" {
				switch e.TargetOverride {
				case "anchor":
					targetTag = "anchor"
				case "exit":
					targetTag = "exit"
				case "unknown":
					targetTag = "unknown"
				}
			} else {
				// Default targeting priority: player (if visible), player last-known, anchor, exit.
				if dist <= defaultVisibilityRadius {
					targetTag = "player"
				} else if e.HasLastKnown {
					targetTag = "player"
				} else {
					targetTag = "anchor"
				}
			}
			// Only include enemy in view if visible to agent
			if _, ok := visMap[e.Pos]; ok {
				snap.Dungeon.Enemies = append(snap.Dungeon.Enemies, agent.EnemyView{Kind: string(e.Kind), X: e.Pos.X, Y: e.Pos.Y, Target: targetTag})
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
		// Blocked actions by instability band (v0.3.5).
		if band >= 2 {
			snap.Dungeon.BlockedActions = append(snap.Dungeon.BlockedActions, "wait:blocked")
			snap.Dungeon.BlockedActions = append(snap.Dungeon.BlockedActions, "distract:blocked")
		}
		if band >= 3 {
			snap.Dungeon.BlockedActions = append(snap.Dungeon.BlockedActions, "observe:blocked")
			snap.Dungeon.BlockedActions = append(snap.Dungeon.BlockedActions, "hide:blocked")
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
		// HIDE is disabled in CRITICAL band.
		// (Preserved as blocked-action hint; enforcement is server-side.)
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

// dungeonEnergyDelta returns the energy delta (positive restores, negative costs)
// for an action in dungeon mode for the given instability band.
// This is used to keep the runtime authoritative while allowing agents to
// mirror costs for UX.
func dungeonEnergyDelta(action agent.Action, band int) int {
	extraMove := 0
	extraObserve := 0
	waitRestore := agent.WaitEnergyRestore
	if band >= 1 {
		extraMove = 1
	}
	if band == 1 || band == 2 {
		extraObserve = 1
	}
	if band >= 2 {
		waitRestore = 0
	}

	switch action {
	case agent.MOVE_N, agent.MOVE_S, agent.MOVE_E, agent.MOVE_W:
		return -(agent.MoveEnergyCost + extraMove)
	case agent.OBSERVE:
		return -(agent.ObserveEnergyCost + extraObserve)
	case agent.WAIT:
		return waitRestore
	case agent.HIDE:
		return -2
	case agent.DISTRACT:
		return -3
	default:
		return 0
	}
}

func applyEnergyDelta(a agent.Agent, delta int) {
	if delta == 0 {
		return
	}
	if adj, ok := a.(interface{ AdjustEnergy(int) }); ok {
		adj.AdjustEnergy(delta)
	}
}
