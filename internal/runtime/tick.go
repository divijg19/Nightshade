package runtime

import (
	"fmt"
	"strings"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
	"github.com/divijg19/Nightshade/internal/dungeon"
	"github.com/divijg19/Nightshade/internal/game"
	"github.com/divijg19/Nightshade/internal/persist"
	"github.com/divijg19/Nightshade/internal/util"
)

type Decisions map[string]agent.Action

func narrationPriority(msg string) int {
	m := strings.ToLower(strings.TrimSpace(msg))
	switch {
	case strings.Contains(m, "shatters") || strings.Contains(m, "collapse") || strings.Contains(m, "expel"):
		return 1
	case strings.Contains(m, "signal stabilized"):
		return 2
	case strings.Contains(m, "channel broken"):
		return 3
	case strings.Contains(m, "exit channel"):
		return 4
	case strings.Contains(m, "exit blocked"):
		return 5
	case strings.Contains(m, "lock"):
		return 6
	case strings.Contains(m, "instability surges"):
		return 7
	default:
		return 8
	}
}

func (r *Runtime) setPendingEvent(agentID string, msg string) {
	if strings.TrimSpace(msg) == "" {
		return
	}
	if cur, ok := r.pendingEvents[agentID]; ok {
		if narrationPriority(msg) >= narrationPriority(cur) {
			return
		}
	}
	r.pendingEvents[agentID] = msg
}

func (r *Runtime) TickOnce() Decisions {
	// Advance non-agent world facts before agents observe.
	r.world.MoveMarker()
	if r.board != nil {
		r.board.Tick()
	}

	// World event cycles (v0.3.9): deterministic every 100 ticks
	// Note: label computed in snapshots when needed to avoid unused state here.
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
		// determine anchor cadence (default 2); if the occupying agent at the anchor
		// has anchor_mastery, slow to every 3 ticks.
		cadence := 2
		if atAnchor {
			for _, aid := range agentIDs {
				if p, ok := instAgentPositions[inst][aid]; ok {
					if p == inst.Anchor {
						if pr, ok2 := r.progressByAgent[aid]; ok2 && pr != nil {
							if pr.UnlockedSkills["anchor_mastery"] {
								cadence = 3
							}
						}
						break
					}
				}
			}
		}
		inst.TickWithAnchor(atAnchor, r.tick, cadence)

		// World event effects (v0.3.9)
		worldCycle := r.tick / 100
		worldEventIdx := worldCycle % 3
		if worldEventIdx == 1 {
			// Stability Drain: extra +1 pressure deterministically
			inst.Pressure++
		}
		if worldEventIdx == 2 {
			// Hunter Migration: spawn a cycle-identified hunter if not present
			migID := fmt.Sprintf("migr-%d", worldCycle)
			found := false
			for _, e := range inst.Entities {
				if e.ID == migID {
					found = true
					break
				}
			}
			if !found {
				pos := inst.Anchor
				if pos.Y-1 >= 0 {
					pos = core.Position{X: pos.X + 1, Y: pos.Y - 1}
				}
				inst.Entities = append(inst.Entities, dungeon.Entity{ID: migID, Pos: pos, Kind: dungeon.EnemyHunter})
			}
		}

		// Objective progression (v0.3.8)
		switch inst.ObjectiveType {
		case "STABILIZE":
			if atAnchor && inst.Pressure < inst.MaxPressure {
				inst.ObjectiveProgress++
			} else if !atAnchor {
				inst.ObjectiveProgress = 0
			}
			if inst.ObjectiveProgress >= inst.ObjectiveTarget {
				inst.ObjectiveCompleted = true
			}
		case "HUNT":
			foundElite := false
			for i := range inst.Entities {
				if inst.Entities[i].ID == "elite-0" {
					foundElite = true
					break
				}
			}
			if !foundElite {
				inst.ObjectiveProgress = inst.ObjectiveTarget
				inst.ObjectiveCompleted = true
			}
		}

		pressureThreshold := (inst.MaxPressure*80 + 99) / 100
		progressThreshold := 0
		if inst.ObjectiveTarget > 0 {
			progressThreshold = (inst.ObjectiveTarget*80 + 99) / 100
		}
		if inst.Pressure >= pressureThreshold || (progressThreshold > 0 && inst.ObjectiveProgress >= progressThreshold) {
			inst.Phase = "ENRAGED"
		} else {
			inst.Phase = "NORMAL"
		}

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

			// Lock timing: decrement TargetLocked lock ticks at tick start
			if e.LockTicks > 0 {
				e.LockTicks--
				if e.LockTicks == 0 {
					e.TargetLocked = false
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

			// Archetype-specific behavior: Sentinels lock, Wardens move on cadence,
			// Shades only move in Dangerous+ and phase through walls.
			sentinelRange := 2
			if inst.Enraged() {
				sentinelRange = 3
			}
			switch e.Kind {
			case dungeon.EntityKind("SENTINEL"):
				// if player within range, lock onto player for N ticks (halved when enraged)
				if nearestAny <= sentinelRange && !e.TargetLocked {
					e.TargetLocked = true
					if inst.Enraged() {
						e.LockTicks = 2
					} else {
						e.LockTicks = 3
					}
				}
			case dungeon.EntityKind("HUNTER"):
				// Hunters immediately lock when player is visible
				if nearestVisible != 999 && !e.TargetLocked {
					e.TargetLocked = true
					e.LockTicks = 3
				}
			case dungeon.EntityKind("WARDEN"):
				// Wardens move only on even ticks; last-move tracking prevents extra moves
				// handled below in movement cadence check.
			case dungeon.EntityKind("SHADE"):
				// Shades will only move when dungeon is Dangerous+; movement check below
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
				// If TargetLocked, prioritize player target
				if e.TargetLocked {
					if nearestVisible != 999 {
						target = nearestVisiblePos
					} else if e.HasLastKnown {
						target = e.LastKnown
					} else {
						target = inst.Anchor
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
			}

			// Move one tile per step, cardinal, cannot pass walls.
			if !halt {
				// movement cadence/permission checks
				allowMove := true
				if e.Kind == dungeon.EntityKind("WARDEN") {
					if r.tick%2 != 0 {
						allowMove = false
					}
				}
				if e.Kind == dungeon.EntityKind("SHADE") {
					// Shades only move when dungeon Dangerous+ (band >= 2)
					if inst.InstabilityBand() < 2 && !inst.Enraged() {
						allowMove = false
					}
				}
				if inst.Enraged() {
					allowMove = true
				}
				if !allowMove {
					// skip movement
					continue
				}
				moveSteps := 1
				if inst.Enraged() {
					moveSteps = 2
				}
				for s := 0; s < moveSteps; s++ {
					dx := target.X - e.Pos.X
					dy := target.Y - e.Pos.Y
					moved := false
					if dx != 0 {
						step := 1
						if dx < 0 {
							step = -1
						}
						cand := core.Position{X: e.Pos.X + step, Y: e.Pos.Y}
						// Shades phase through walls
						if inst.InBounds(cand) && (e.Kind == dungeon.EntityKind("SHADE") || inst.GlyphAt(cand) != '#') {
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
						if inst.InBounds(cand) && (e.Kind == dungeon.EntityKind("SHADE") || inst.GlyphAt(cand) != '#') {
							if _, occ := occupied[cand]; !occ {
								e.Pos = cand
							}
						}
					}
				}
			}
		}

		if inst.ObjectiveType == "HUNT" {
			foundElite := false
			for i := range inst.Entities {
				if inst.Entities[i].ID == "elite-0" {
					foundElite = true
					break
				}
			}
			if !foundElite {
				inst.ObjectiveProgress = inst.ObjectiveTarget
				inst.ObjectiveCompleted = true
			}
		}

		threshold := inst.MaxPressure * 75 / 100
		if inst.Pressure >= threshold {
			inst.CoreIntegrity -= 1
		}
		for i := range inst.Entities {
			if inst.Entities[i].Pos == inst.Anchor {
				inst.CoreIntegrity -= 2
				break
			}
		}
		if inst.CoreIntegrity <= 0 {
			for _, aid := range agentIDs {
				sigID := r.signalByAgent[aid]
				if r.board != nil && sigID != "" {
					r.board.Burn(sigID)
				}
				if _, ok := r.dungeonByAgent[aid]; ok {
					r.applyDungeonRewards(aid, inst, false)
				}
				delete(r.dungeonByAgent, aid)
				delete(r.signalByAgent, aid)
				r.setPendingEvent(aid, "The signal shatters.")
				r.pendingEjects[aid] = "integrity"
			}
			inst.Done = true
			inst.DoneReason = "integrity"
			continue
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
								// apply deterministic rewards for forced-eject
								if inst, ok := r.dungeonByAgent[aid]; ok {
									r.applyDungeonRewards(aid, inst, false)
								}
								delete(r.dungeonByAgent, aid)
								r.setPendingEvent(aid, "The signal shatters.")
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
				d.CoreIntegrity -= 10
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
				// apply deterministic rewards for collapse
				if inst, ok := r.dungeonByAgent[aid]; ok {
					r.applyDungeonRewards(aid, inst, false)
				}
				// remove bindings
				delete(r.dungeonByAgent, aid)
				delete(r.signalByAgent, aid)
				// schedule one-shot event and eject flag for next snapshot
				r.setPendingEvent(aid, "The signal shatters.")
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
			} else if strings.TrimSpace(in) == "UPGRADE" {
				// send a fresh snapshot containing AvailableSkills/unlocked info
				rsh := r.snapshotFor(a, agent.Action(-1))
				rh.Observe(rsh)
				inputs[a.ID()] = ""
			} else if strings.HasPrefix(strings.TrimSpace(in), "UNLOCK ") {
				skillID := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(in), "UNLOCK "))
				p, ok := r.progressByAgent[a.ID()]
				if !ok || p == nil {
					p = persist.DefaultProgress()
					r.progressByAgent[a.ID()] = p
				}
				if err := agent.UnlockSkill(p, skillID); err != nil {
					r.setPendingEvent(a.ID(), "Unlock failed: "+err.Error())
				} else {
					_ = persist.SaveProgress(a.ID(), p)
					r.setPendingEvent(a.ID(), "Skill unlocked.")
					// If endurance unlocked, update agent instance cap immediately
					bonus := 0
					if p.UnlockedSkills["endurance_2"] {
						bonus = 10
					} else if p.UnlockedSkills["endurance_1"] {
						bonus = 5
					}
					for _, ag := range r.agents {
						if ag.ID() == a.ID() {
							if setter, ok := ag.(interface{ SetMaxEnergyBonus(int) }); ok {
								setter.SetMaxEnergyBonus(bonus)
							}
							break
						}
					}
				}
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
					// determine hide duration (1 by default; deep_concealment -> 2)
					hideDur := 1
					if p, ok := r.progressByAgent[a.ID()]; ok && p.UnlockedSkills["deep_concealment"] {
						hideDur = 2
					}
					// set cooldown (ticks) and apply overrides (+1 because decremented at tick start)
					r.hideCooldown[a.ID()] = hideDur + 1
					for i := range inst.Entities {
						e := &inst.Entities[i]
						// Sentinels are immune to hide/distract.
						if e.Kind == dungeon.EntityKind("SENTINEL") || (inst.Enraged() && e.Kind == dungeon.EntityKind("HUNTER")) {
							continue
						}
						// Hide breaks locks for Wardens/Shades
						if e.Kind == dungeon.EntityKind("WARDEN") || e.Kind == dungeon.EntityKind("SHADE") {
							e.TargetLocked = false
							e.LockTicks = 0
						}
						e.TargetOverride = "unknown"
						// set override to desired duration +1
						e.OverrideTicks = hideDur + 1
					}
					r.setPendingEvent(a.ID(), "Lock disrupted.")
				}
			}
		}
		if inDungeon && inst != nil && authoritative == agent.OBSERVE && inst.ObjectiveType == "PURGE" && !inst.ObjectiveCompleted {
			ap := core.Position{X: pos.X, Y: pos.Y}
			for i := 0; i < len(inst.PurgeNodes); i++ {
				if inst.PurgeNodes[i] == ap {
					inst.PurgeNodes = append(inst.PurgeNodes[:i], inst.PurgeNodes[i+1:]...)
					inst.ObjectiveProgress++
					if inst.ObjectiveProgress >= inst.ObjectiveTarget {
						inst.ObjectiveCompleted = true
						r.setPendingEvent(a.ID(), "Objective PURGE complete.")
					} else {
						r.setPendingEvent(a.ID(), "Instability surges (+1).")
					}
					break
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
					e := &inst.Entities[idx]
					// Sentinels/Wardens/Shades are immune to distract.
					if e.Kind == dungeon.EntityKind("SENTINEL") || e.Kind == dungeon.EntityKind("WARDEN") || e.Kind == dungeon.EntityKind("SHADE") {
						// no effect
					} else {
						if anchorVisible {
							e.TargetOverride = "anchor"
						} else {
							e.TargetOverride = "exit"
						}
						// determine distract duration (default 2; extended_distract -> 3)
						distDur := 2
						if p, ok := r.progressByAgent[a.ID()]; ok && p.UnlockedSkills["extended_distract"] {
							distDur = 3
						}
						// overrides set to duration+1 because they decrement at tick start
						e.OverrideTicks = distDur + 1
						r.setPendingEvent(a.ID(), "Lock disrupted.")
					}
				}
			}
		}

		// v0.3.5: server-authoritative energy costs (adjust difference vs agent mirroring).
		if inDungeon {
			agentDelta := dungeonEnergyDelta(action, band)
			authDelta := dungeonEnergyDelta(authoritative, band)
			// v0.3.9: Energy loop tightening
			// OBSERVE always costs -1 energy; WAIT restores +1 only if not enraged and no TargetLocked enemies
			if inst != nil {
				if authoritative == agent.OBSERVE {
					authDelta = -1
				}
				if authoritative == agent.WAIT {
					// default no restore
					authDelta = 0
					if !inst.Enraged() {
						// check for any target-locked enemies in dungeon
						locked := false
						for i := range inst.Entities {
							if inst.Entities[i].TargetLocked {
								locked = true
								break
							}
						}
						if !locked {
							authDelta = 1
						}
					}
				}
			}
			// Skill: Stability Training reduces CRITICAL move penalty by 1
			if band >= 3 {
				if p, ok := r.progressByAgent[a.ID()]; ok && p != nil {
					if p.UnlockedSkills["stability_training"] {
						// if authoritative action is a move, reduce cost by 1
						switch authoritative {
						case agent.MOVE_N, agent.MOVE_S, agent.MOVE_E, agent.MOVE_W:
							authDelta += 1
						}
					}
					// Efficient Observe reduces OBSERVE cost by 1 (if positive)
					if p.UnlockedSkills["efficient_observe"] {
						if authoritative == agent.OBSERVE {
							authDelta += 1
						}
					}
				}
			}
			applyEnergyDelta(a, authDelta-agentDelta)
		}

		decisions[a.ID()] = authoritative

		newPos := game.ResolveMovement(
			pos,
			authoritative,
			r.world.Width(),
			r.world.Height(),
		)

		// If agent is inside a dungeon and steps onto a risk node, apply deterministic effects
		if d, ok := r.dungeonByAgent[a.ID()]; ok && d != nil {
			np := core.Position{X: newPos.X, Y: newPos.Y}
			// Fragment Node: award fragments (+2 effective including tick increment)
			if np == d.FragmentNode {
				// award deterministic fragments to progress record
				if p, ok := r.progressByAgent[a.ID()]; ok && p != nil {
					bonus := CalculateFragments(d.Pressure, d.InstabilityBand(), false, d.ObjectiveCompleted, d.CoreIntegrity)
					p.Fragments += bonus
					_ = persist.SaveProgress(a.ID(), p)
				}
				// Pressure already advanced at start of tick; add +1 here so the
				// net effect of stepping on this node during a tick is +2.
				d.Pressure += 1
				r.setPendingEvent(a.ID(), "Instability surges (+2).")
				// remove node so it cannot be reused
				d.FragmentNode = core.Position{}
			}
			// Corruption Well: reduces pressure (net -2 including tick increment) and increase aggression
			if np == d.CorruptionWell {
				// Because pressure was already incremented this tick, reduce by 3
				// here so the net effect compared to the pre-tick value is -2.
				d.Pressure -= 3
				if d.Pressure < 0 {
					d.Pressure = 0
				}
				for i := range d.Entities {
					d.Entities[i].Aggro += 3
				}
				r.setPendingEvent(a.ID(), "Instability surges (-2).")
				d.CorruptionWell = core.Position{}
			}
			// Overcharge: +2 energy and +1 pressure
			if np == d.OverchargeNode {
				// award energy to agent via concrete types
				for _, ag := range r.agents {
					if ag.ID() != a.ID() {
						continue
					}
					switch at := ag.(type) {
					case *agent.RemoteHuman:
						at.AdjustEnergy(2)
					case *agent.Human:
						at.AdjustEnergy(2)
					case *agent.Scripted:
						at.AdjustEnergy(2)
					}
				}
				d.Pressure += 1
				r.setPendingEvent(a.ID(), "Instability surges (+1).")
				d.OverchargeNode = core.Position{}
			}
		}

		// If agent is inside a dungeon and attempts to move onto the exit,
		// treat that as an EXIT attempt (commitment). Enforce band/energy
		// constraints server-side.
		if d, ok := r.dungeonByAgent[a.ID()]; ok {
			np := core.Position{X: newPos.X, Y: newPos.Y}
			if np == d.Exit {
				// Enraged exit behavior takes precedence: start channeling when ENRAGED
				if d.Enraged() {
					// Start channeling if not already
					if !d.ExitChanneling {
						d.ExitChanneling = true
						d.ExitChannelTick = r.tick
						r.setPendingEvent(a.ID(), "EXIT CHANNEL: █░░░░")
					} else {
						// if channel started on prior tick and player remains, check locks
						if r.tick > d.ExitChannelTick {
							// if any enemy currently TargetLocked, break channel
							locked := false
							for i := range d.Entities {
								if d.Entities[i].TargetLocked {
									locked = true
									break
								}
							}
							if locked {
								d.ExitChanneling = false
								r.setPendingEvent(a.ID(), "Channel broken by threat.")
							} else if !d.ObjectiveCompleted {
								r.setPendingEvent(a.ID(), "Exit blocked — objective incomplete.")
							} else {
								// complete exit
								if r != nil {
									r.applyDungeonRewards(a.ID(), d, true)
								}
								d.Pressure = 0
								d.Done = true
								d.DoneReason = "exit"
								delete(r.dungeonByAgent, a.ID())
								delete(r.signalByAgent, a.ID())
								r.setPendingEvent(a.ID(), "Signal stabilized.")
							}
						}
					}
				} else {
					// Non-enraged immediate exit behavior (unchanged)
					// Enforce objective completion before allowing exit.
					if !d.ObjectiveCompleted {
						r.setPendingEvent(a.ID(), "Exit blocked — objective incomplete.")
						// do not remove binding
					} else {
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
						if band >= 3 && energy < 1 {
							allow := false
							if p, ok := r.progressByAgent[a.ID()]; ok && p != nil {
								if p.UnlockedSkills["exit_instinct"] {
									allow = true
								}
							}
							if !allow {
								// silent failure; renderer will be hinted via snapshot.BlockedActions
							} else {
								if r != nil {
									r.applyDungeonRewards(a.ID(), d, true)
								}
								d.Pressure = 0
								d.Done = true
								delete(r.dungeonByAgent, a.ID())
								delete(r.signalByAgent, a.ID())
								r.setPendingEvent(a.ID(), "Signal stabilized.")
							}
						} else {
							if r != nil {
								r.applyDungeonRewards(a.ID(), d, true)
							}
							d.Pressure = 0
							d.Done = true
							d.DoneReason = "exit"
							delete(r.dungeonByAgent, a.ID())
							delete(r.signalByAgent, a.ID())
							r.setPendingEvent(a.ID(), "Signal stabilized.")
						}
					}
				}
			}
		}

		if d, ok := r.dungeonByAgent[a.ID()]; ok && d != nil && d.ObjectiveType == "RETRIEVE" {
			np := core.Position{X: newPos.X, Y: newPos.Y}
			if np == d.Anchor {
				d.RetrieveHold++
			} else {
				d.RetrieveHold = 0
			}
			d.ObjectiveProgress = d.RetrieveHold
			if d.RetrieveHold >= d.ObjectiveTarget {
				d.ObjectiveCompleted = true
				r.setPendingEvent(a.ID(), "Objective RETRIEVE complete.")
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
	// seed deterministic entities based on signal type and current tick
	if inst.ObjectiveType == "HUNT" {
		pos := inst.Anchor
		if pos.Y-1 >= 0 {
			pos = core.Position{X: pos.X, Y: pos.Y - 1}
		}
		inst.Entities = []dungeon.Entity{{ID: "elite-0", Pos: pos, Kind: dungeon.EnemyHunter}}
	} else {
		inst.SeedEntitiesForSignal(string(s.Type), r.tick)
	}
	r.dungeonByAgent[agentID] = inst
	r.signalByAgent[agentID] = signalID
	// ensure progress record exists
	if _, ok := r.progressByAgent[agentID]; !ok {
		if p, err := persist.LoadProgress(agentID); err == nil {
			r.progressByAgent[agentID] = p
		} else {
			r.progressByAgent[agentID] = persist.DefaultProgress()
		}
	}
	if p, ok := r.progressByAgent[agentID]; ok && p != nil {
		p.LastSignalID = signalID
		if !p.DungeonIntroShown {
			r.setPendingEvent(agentID, "You are inside a signal.\nPressure rises each tick.\nReach the exit before collapse.")
			p.DungeonIntroShown = true
		}
		_ = persist.SaveProgress(agentID, p)
	}
	// initialize run stats
	r.runStats[agentID] = struct {
		FragmentsEarnedThisRun int
		HighestPressureThisRun int
	}{FragmentsEarnedThisRun: 0, HighestPressureThisRun: inst.Pressure}
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
			Grid:               grid,
			Pressure:           d.Pressure,
			MaxPressure:        d.MaxPressure,
			Tick:               snap.Tick,
			InstabilityBand:    band,
			ObjectiveType:      d.ObjectiveType,
			ObjectiveProgress:  d.ObjectiveProgress,
			ObjectiveTarget:    d.ObjectiveTarget,
			ObjectiveCompleted: d.ObjectiveCompleted,
			CoreIntegrity:      d.CoreIntegrity,
			Phase:              d.Phase,
			Enraged:            d.Enraged(),
			ExitChanneling:     d.ExitChanneling,
			ExitChannelTick:    d.ExitChannelTick,
			WorldEventLabel: func() string {
				wc := r.tick / 100
				e := wc % 3
				switch e {
				case 0:
					return "Signal Surge"
				case 1:
					return "Stability Drain"
				default:
					return "Hunter Migration"
				}
			}(),
			ExitState: exitState,
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
				// snapshot-level telegraphing for Threat Awareness
				displayLocked := e.TargetLocked
				if p, ok := r.progressByAgent[a.ID()]; ok {
					if p.UnlockedSkills["threat_awareness"] {
						// telegraph one tick earlier: if LockTicks == 1, show as locked
						if e.LockTicks == 1 {
							displayLocked = true
						}
					}
				}
				snap.Dungeon.Enemies = append(snap.Dungeon.Enemies, agent.EnemyView{Kind: string(e.Kind), X: e.Pos.X, Y: e.Pos.Y, Target: targetTag, TargetLocked: displayLocked})
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
			if !d.ObjectiveCompleted {
				snap.Dungeon.BlockedActions = append(snap.Dungeon.BlockedActions, "exit:objective_incomplete")
			}
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

		// Attach progression/run stats for presentation
		if p, ok := r.progressByAgent[a.ID()]; ok && p != nil {
			snap.Fragments = p.Fragments
			snap.SkillPoints = p.SkillPoints
			// list unlocked skills
			ul := make([]string, 0, len(p.UnlockedSkills))
			for k := range p.UnlockedSkills {
				ul = append(ul, k)
			}
			snap.UnlockedSkills = ul
			// populate active skills in dungeon view
			active := make([]string, 0, len(p.UnlockedSkills))
			for k := range p.UnlockedSkills {
				active = append(active, k)
			}
			snap.Dungeon.ActiveSkillShortNames = active

			// BuildLabel derivation (v0.3.9)
			if p.UnlockedSkills["anchor_mastery"] {
				snap.Dungeon.BuildLabel = "Stabilizer"
			} else if p.UnlockedSkills["extended_distract"] || p.UnlockedSkills["threat_awareness"] {
				snap.Dungeon.BuildLabel = "Hunter"
			} else if p.UnlockedSkills["stability_training"] || p.UnlockedSkills["pressure_sense"] {
				snap.Dungeon.BuildLabel = "Diver"
			} else if p.UnlockedSkills["endurance_1"] || p.UnlockedSkills["endurance_2"] || p.UnlockedSkills["exit_instinct"] {
				snap.Dungeon.BuildLabel = "Sentinel"
			} else {
				snap.Dungeon.BuildLabel = "Adventurer"
			}
			// NextBandThreshold when pressure_sense unlocked
			if p.UnlockedSkills["pressure_sense"] {
				// compute next threshold
				cur := d.Pressure
				next := 6
				switch {
				case cur >= 16:
					next = 0
				case cur >= 11:
					next = 16
				case cur >= 6:
					next = 11
				default:
					next = 6
				}
				snap.Dungeon.NextBandThreshold = next - cur
			}
		}
		// run stats
		rs := r.runStats[a.ID()]
		snap.Dungeon.FragmentsEarnedThisRun = rs.FragmentsEarnedThisRun
		snap.Dungeon.HighestPressureThisRun = rs.HighestPressureThisRun

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
		// attach progression info for board mode
		if p, ok := r.progressByAgent[a.ID()]; ok && p != nil {
			snap.Fragments = p.Fragments
			snap.SkillPoints = p.SkillPoints
			ul := make([]string, 0, len(p.UnlockedSkills))
			for k := range p.UnlockedSkills {
				ul = append(ul, k)
			}
			snap.UnlockedSkills = ul
			// populate available skills listing (always present)
			skills := agent.AllSkills()
			avail := make([]agent.Skill, 0, len(skills))
			for _, v := range skills {
				avail = append(avail, v)
			}
			snap.AvailableSkills = avail
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
func dungeonEnergyDelta(action agent.Action, _ int) int {
	// v0.3.9: Movement always costs -1. Observe costs -1 (tightened).
	switch action {
	case agent.MOVE_N, agent.MOVE_S, agent.MOVE_E, agent.MOVE_W:
		return -agent.MoveEnergyCost
	case agent.OBSERVE:
		return -1
	case agent.WAIT:
		return agent.WaitEnergyRestore
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

// CalculateFragments deterministically computes fragment rewards for a dungeon run.
func CalculateFragments(maxPressure int, instabilityBand int, exited bool, objectiveCompleted bool, coreIntegrity int) int {
	_ = instabilityBand
	_ = exited
	base := 2
	pressureBonus := maxPressure / 3
	objectiveBonus := 0
	if objectiveCompleted {
		objectiveBonus = 3
	}
	if coreIntegrity < 0 {
		coreIntegrity = 0
	}
	integrityBonus := (coreIntegrity * 2) / 100
	return base + objectiveBonus + pressureBonus + integrityBonus
}

// applyDungeonRewards updates progress for the agent and persists it.
func (r *Runtime) applyDungeonRewards(agentID string, inst *dungeon.Instance, exited bool) {
	if inst == nil {
		return
	}
	maxPressure := inst.Pressure
	band := inst.InstabilityBand()
	fragments := CalculateFragments(maxPressure, band, exited, inst.ObjectiveCompleted, inst.CoreIntegrity)
	// ensure progress entry exists
	p, ok := r.progressByAgent[agentID]
	if !ok || p == nil {
		if np, err := persist.LoadProgress(agentID); err == nil {
			p = np
		} else {
			p = persist.DefaultProgress()
		}
		r.progressByAgent[agentID] = p
	}
	p.Fragments += fragments
	p.TotalDungeons += 1
	if maxPressure > p.HighestPressureReached {
		p.HighestPressureReached = maxPressure
	}
	p.SkillPoints = p.Fragments / 10
	// persist immediately
	_ = persist.SaveProgress(agentID, p)
	// update run stats for snapshot presentation
	rs := r.runStats[agentID]
	rs.FragmentsEarnedThisRun = fragments
	if maxPressure > rs.HighestPressureThisRun {
		rs.HighestPressureThisRun = maxPressure
	}
	r.runStats[agentID] = rs
}
