package agent

import (
	"encoding/base64"
	"time"

	"github.com/divijg19/Nightshade/internal/core"
)

// RemoteHuman is a headless human-compatible Agent used by the server.
// It executes the exact cognition pipeline as `Human` but exposes channels
// for sending observations to a client and receiving a single-key input
// per tick. It does not perform any terminal I/O.
type RemoteHuman struct {
    id string
    memory *Memory
    energy int

    // Channels populated by server connection goroutines.
    SendObservation chan Observation // server -> client
    RecvInput chan string           // client -> server (single-key string)

    // reconnect hint: when a new client binds to this agent, the server
    // may replace the channels to point at new connection handlers.
}

func NewRemoteHumanFromExisting(id string, mem *Memory, energy int) *RemoteHuman {
    return &RemoteHuman{
        id: id,
        memory: mem,
        energy: energy,
        SendObservation: make(chan Observation, 1),
        RecvInput: make(chan string, 1),
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
func (r *RemoteHuman) Energy() int { return r.energy }

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

    // 10. Apply energy effects
    switch final {
    case MOVE_N, MOVE_S, MOVE_E, MOVE_W:
        r.energy -= MoveEnergyCost
    case OBSERVE:
        r.energy -= ObserveEnergyCost
    case WAIT:
        r.energy += WaitEnergyRestore
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
