package signal

import (
	"fmt"
)

type SignalType string

type AnchorType string

type ZoneType string

type PresenceLevel string

const (
	SignalFracture SignalType = "FRACTURE"
	SignalNull     SignalType = "NULL"

	AnchorMemoryVault  AnchorType = "MEMORY_VAULT"
	AnchorRecoveryNode AnchorType = "RECOVERY_NODE"

	ZoneStable  ZoneType = "STABLE"
	ZoneEchoing ZoneType = "ECHOING"

	PresenceLow    PresenceLevel = "LOW"
	PresenceMedium PresenceLevel = "MEDIUM"
)

type Signal struct {
	ID         string
	Type       SignalType
	Anchor     AnchorType
	Zone       ZoneType
	Presence   PresenceLevel
	DecayTicks int
	LockedBy   string // empty means unlocked
	Burned     bool   // true when permanently burned/unenterable
}

type Board struct {
	maxSignals int
	nextID     int
	signals    []Signal
}

func NewBoard(maxSignals int) *Board {
	b := &Board{maxSignals: maxSignals}
	// Seed with a small deterministic pool.
	for i := 0; i < maxSignals; i++ {
		b.signals = append(b.signals, b.spawn())
	}
	return b
}

func (b *Board) Signals() []Signal {
	out := make([]Signal, len(b.signals))
	copy(out, b.signals)
	return out
}

func (b *Board) Find(id string) (Signal, bool) {
	for _, s := range b.signals {
		if s.ID == id {
			return s, true
		}
	}
	return Signal{}, false
}

func (b *Board) Lock(id string, agentID string) bool {
	for i := range b.signals {
		if b.signals[i].ID != id {
			continue
		}
		if b.signals[i].Burned {
			return false
		}
		if b.signals[i].LockedBy != "" && b.signals[i].LockedBy != agentID {
			return false
		}
		b.signals[i].LockedBy = agentID
		return true
	}
	return false
}

// Burn marks a signal as burned (permanently unenterable) and clears locks.
func (b *Board) Burn(id string) {
	for i := range b.signals {
		if b.signals[i].ID != id {
			continue
		}
		b.signals[i].Burned = true
		b.signals[i].LockedBy = ""
		return
	}
}

func (b *Board) Remove(id string) {
	out := b.signals[:0]
	for _, s := range b.signals {
		if s.ID == id {
			continue
		}
		out = append(out, s)
	}
	b.signals = out
}

func (b *Board) Tick() {
	// decay & drop
	out := b.signals[:0]
	for _, s := range b.signals {
		s.DecayTicks--
		// Preserve signals that are currently locked or already burned so
		// they remain visible on the board even if their decay expired.
		if s.DecayTicks <= 0 && s.LockedBy == "" && !s.Burned {
			continue
		}
		out = append(out, s)
	}
	b.signals = out

	// refill deterministically
	for len(b.signals) < b.maxSignals {
		b.signals = append(b.signals, b.spawn())
	}
}

func (b *Board) spawn() Signal {
	id := fmt.Sprintf("S%03d", b.nextID)
	b.nextID++

	// deterministic cycle; no RNG.
	typeCycle := []SignalType{SignalFracture, SignalNull}
	anchorCycle := []AnchorType{AnchorMemoryVault, AnchorRecoveryNode}
	zoneCycle := []ZoneType{ZoneStable, ZoneEchoing}
	presenceCycle := []PresenceLevel{PresenceLow, PresenceMedium}

	i := b.nextID
	return Signal{
		ID:         id,
		Type:       typeCycle[i%len(typeCycle)],
		Anchor:     anchorCycle[(i/2)%len(anchorCycle)],
		Zone:       zoneCycle[(i/3)%len(zoneCycle)],
		Presence:   presenceCycle[(i/5)%len(presenceCycle)],
		DecayTicks: 6 + (i % 4),
		LockedBy:   "",
	}
}
