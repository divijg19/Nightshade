package agent

import (
	"encoding/base64"
	"sync"
	"time"

	"github.com/divijg19/Nightshade/internal/core"
)

// RemoteHuman is a headless human-compatible Agent used by the server.
// It executes the exact cognition pipeline as `Human` but exposes channels
// for sending observations to a client and receiving a single-key input
// per tick. It does not perform any terminal I/O.
type RemoteHuman struct {
	id     string
	memory *Memory
	energy int

	// Channels populated by server connection goroutines.
	SendObservation chan Observation // server -> client
	RecvInput       chan string      // client -> server (single-key string)

	// reconnect hint: when a new client binds to this agent, the server
	// may replace the channels to point at new connection handlers.
	connMu    sync.RWMutex
	connected bool
}

func NewRemoteHumanFromExisting(id string, mem *Memory, energy int) *RemoteHuman {
	return &RemoteHuman{
		id:              id,
		memory:          mem,
		energy:          energy,
		SendObservation: make(chan Observation, 1),
		RecvInput:       make(chan string, 1),
	}
}

func (r *RemoteHuman) ID() string { return r.id }

// IDBase64 returns the base64-encoded public key / agent id if the id is a
// raw key; helpers/tests may rely on this for display.
func (r *RemoteHuman) IDBase64() string {
	return base64.StdEncoding.EncodeToString([]byte(r.id))
}

// Memory accessor for server-side persistence/testing
func (r *RemoteHuman) Memory() *Memory { return r.memory }
func (r *RemoteHuman) Energy() int     { return r.energy }

// AdjustEnergy adjusts the agent's energy by delta and clamps to allowed range.
func (r *RemoteHuman) AdjustEnergy(delta int) {
	r.energy += delta
	if r.energy > MaxEnergy {
		r.energy = MaxEnergy
	}
	if r.energy < MinEnergy {
		r.energy = MinEnergy
	}
}

// SetConnected marks whether a client is currently attached to this RemoteHuman.
// This is used by the server to indicate that TickOnce should block waiting
// for inputs for this agent.
func (r *RemoteHuman) SetConnected(v bool) {
	r.connMu.Lock()
	r.connected = v
	r.connMu.Unlock()
}

// IsConnected reports whether a client is currently attached to this agent.
func (r *RemoteHuman) IsConnected() bool {
	r.connMu.RLock()
	v := r.connected
	r.connMu.RUnlock()
	return v
}

// Decide implements agent.Agent. It mirrors the `Human.Decide` cognition
// pipeline but without any terminal rendering. Instead it sends the
// constructed Observation over `SendObservation` and waits (with a
// reasonable timeout) for a single-key input on `RecvInput`.
func (r *RemoteHuman) Decide(snapshot Snapshot) Action {
	// Backwards-compatible Decide: send an observation (as older code did),
	// then read input from RecvInput and invoke DecideWithInput.
	r.Observe(snapshot)

	var input string
	select {
	case in := <-r.RecvInput:
		input = in
	case <-time.After(5 * time.Second):
		input = ""
	}
	return r.DecideWithInput(snapshot, input)
}

// DecideWithInput performs the same cognition pipeline as Decide but uses the
// provided input string instead of reading from the channel. This allows the
// runtime to collect inputs deterministically during the Input phase and then
// call DecideWithInput during the Decision phase.
func (r *RemoteHuman) DecideWithInput(snapshot Snapshot, input string) Action {
	// 1. Update memory from visible
	var prev map[core.Position]MemoryTile
	if r.memory != nil {
		prev = r.memory.UpdateFromVisible(snapshot)
	}
	// Prepare position and tick for contagion/conflict calculations
	pos := core.Position{}
	if p, ok := snapshot.(interface{ PositionValue() core.Position }); ok {
		pos = p.PositionValue()
	}
	tick := 0
	if t, ok := snapshot.(interface{ TickValue() int }); ok {
		tick = t.TickValue()
	}

	// 2. Apply contagion (belief signals must have been emitted by the
	// runtime emission pass before this method is called).
	_ = applyBeliefContagion(r.id, pos, tick, r.memory, r.energy)

	// 4. Detect & apply conflicts
	detectAndApplyConflicts(r.memory, prev, tick)

	// 5. Build Observation
	effectiveParanoia := ParanoiaThreshold
	effectiveCaution := CautionThreshold
	if r.energy < LowEnergyThreshold {
		effectiveParanoia = ParanoiaThreshold - 2
		effectiveCaution = CautionThreshold - 1
	}
	obs := buildObservation(r.memory, snapshot, prev, r.energy, effectiveParanoia)

	// 6. Translate provided input to intended Action
	intended := keyToAction(input)

	// 8. Apply caution override
	if intended == MOVE_N || intended == MOVE_S || intended == MOVE_E || intended == MOVE_W {
		if posv, ok := snapshot.(interface{ PositionValue() core.Position }); ok {
			if tgt, ok2 := computeTarget(posv.PositionValue(), intended); ok2 {
				if mt, found := r.memory.GetMemoryTile(tgt); found {
					age := obs.Tick - mt.LastSeen
					if age > effectiveCaution {
						intended = OBSERVE
					}
				}
			}
		}
	}

	// 9. Critical energy collapse
	final := intended
	if r.energy < CriticalEnergyThreshold {
		final = WAIT
	}

	// 10. Apply energy effects with dungeon band modifiers (presentation
	// hints come from snapshot.Dungeon; enforce costs here deterministically)
	// Determine band-based modifiers if a DungeonView is present.
	extraMove := 0
	extraObserve := 0
	waitRestore := WaitEnergyRestore
	if dv, ok := snapshot.(interface{ DungeonValue() DungeonView }); ok {
		d := dv.DungeonValue()
		if d.InstabilityBand == 1 {
			extraObserve = 1
		}
		if d.InstabilityBand == 2 {
			waitRestore = 0
		}
		if d.InstabilityBand >= 3 {
			extraMove = 1
		}
	}

	switch final {
	case MOVE_N, MOVE_S, MOVE_E, MOVE_W:
		r.energy -= (MoveEnergyCost + extraMove)
	case OBSERVE:
		r.energy -= (ObserveEnergyCost + extraObserve)
	case WAIT:
		r.energy += waitRestore
	}
	if r.energy > MaxEnergy {
		r.energy = MaxEnergy
	}
	if r.energy < MinEnergy {
		r.energy = MinEnergy
	}

	// 11. OBSERVE healing
	if final == OBSERVE && r.memory != nil {
		for pos, mt := range r.memory.tiles {
			if mt.ScarLevel > 0 {
				mt.ScarLevel -= 1
				if mt.ScarLevel < 0 {
					mt.ScarLevel = 0
				}
				r.memory.tiles[pos] = mt
			}
		}
	}

	return final
}

// Observe builds an Observation from the given Snapshot and the agent's
// Memory, then sends it over the SendObservation channel (non-blocking).
func (r *RemoteHuman) Observe(snapshot Snapshot) {
	var prev map[core.Position]MemoryTile
	if r.memory != nil {
		prev = r.memory.UpdateFromVisible(snapshot)
	}
	effectiveParanoia := ParanoiaThreshold
	if r.energy < LowEnergyThreshold {
		effectiveParanoia = ParanoiaThreshold - 2
	}
	obs := buildObservation(r.memory, snapshot, prev, r.energy, effectiveParanoia)
	// Copy presentation-only metadata from snapshot if available.
	if vm, ok := snapshot.(interface{ ViewMode() string }); ok {
		obs.Mode = vm.ViewMode()
	}
	if bv, ok := snapshot.(interface{ BoardValue() BoardView }); ok {
		b := bv.BoardValue()
		if len(b.Signals) > 0 {
			obs.Board = &b
		}
	}
	if dv, ok := snapshot.(interface{ DungeonValue() DungeonView }); ok {
		d := dv.DungeonValue()
		// Always include dungeon view when the snapshot indicates dungeon mode.
		// This allows the client/renderer to display pressure starting at 0.
		if obs.Mode == "dungeon" || d.MaxPressure > 0 || len(d.Grid) > 0 {
			obs.Dungeon = &d
			if obs.Mode == "" {
				obs.Mode = "dungeon"
			}
		}
	}
	// Copy one-shot server event if present.
	if ev, ok := snapshot.(interface{ EventValue() string }); ok {
		if e := ev.EventValue(); e != "" {
			obs.Event = e
		}
	}
	// Compute positional presence cues from belief signals (best-effort, non-leaky):
	// if other agents have emitted belief signals within BeliefRadius, add an
	// ephemeral PresenceCue. Do not include identities.
	sigs := GetBeliefSignals()
	for id, sig := range sigs {
		if id == r.id {
			continue
		}
		// compute manhattan
		d := 0
		var pos core.Position
		if p, ok := snapshot.(interface{ PositionValue() core.Position }); ok {
			pos = p.PositionValue()
			dx := sig.Position.X - pos.X
			if dx < 0 {
				dx = -dx
			}
			dy := sig.Position.Y - pos.Y
			if dy < 0 {
				dy = -dy
			}
			d = dx + dy
		}
		if d <= BeliefRadius {
			// classify agent kinds heuristically by id prefix (server-side ids
			// for NPCs use "npc-" prefixes). This is only for presentation.
			ptype := PresenceHumanOther
			if len(id) >= 4 && id[:4] == "npc-" {
				ptype = PresenceNPC
			}
			obs.Presence = append(obs.Presence, PresenceCue{Type: ptype, Position: sig.Position})
		}
	}
	select {
	case r.SendObservation <- obs:
	default:
	}
}

// EmitBeliefs emits the agent's BeliefSignal for the provided snapshot
// without applying contagion. The runtime will call this for all agents
// before running the contagion/decision pass to ensure simultaneous
// emission semantics.
func (r *RemoteHuman) EmitBeliefs(snapshot Snapshot) {
	pos := core.Position{}
	if p, ok := snapshot.(interface{ PositionValue() core.Position }); ok {
		pos = p.PositionValue()
	}
	beliefs := []Belief{}
	if r.memory != nil {
		for _, mt := range r.memory.All() {
			age := 0
			if t, ok := snapshot.(interface{ TickValue() int }); ok {
				age = t.TickValue() - mt.LastSeen
			}
			beliefs = append(beliefs, Belief{Tile: mt.Tile, Age: age, ScarLevel: mt.ScarLevel})
		}
	}
	tick := 0
	if t, ok := snapshot.(interface{ TickValue() int }); ok {
		tick = t.TickValue()
	}
	emitBeliefSignal(r.id, tick, pos, beliefs)
}
