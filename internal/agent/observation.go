package agent

import "github.com/divijg19/Nightshade/internal/core"

// Belief pairs a remembered tile with its age (in ticks since last seen).
type Belief struct {
	Tile core.TileView
	Age  int

	// ScarLevel indicates accumulated cognitive scarring for this belief.
	ScarLevel int
}

// Observation is the agent-side interpretation of a runtime Snapshot.
// It separates ephemeral Visible tiles from the agent's persistent Known
// belief (built from Memory). Known contains Belief entries computed from
// Memory.LastSeen and the current Tick.
type Observation struct {
	Visible []core.TileView
	Known   []Belief
	Tick    int
	// Position is the agent's current world position (for centering view)
	Position core.Position
	// Presence contains ephemeral positional cues about nearby agents/NPCs.
	// These are belief-derived impressions only and must not expose runtime truth.
	Presence []PresenceCue
}

// PresenceType enumerates the kinds of perceived presences.
type PresenceType string

const (
	PresenceSelf      PresenceType = "Self"
	PresenceHumanOther PresenceType = "HumanOther"
	PresenceNPC       PresenceType = "NPC"
)

// PresenceCue is an ephemeral perception about another agent or self.
// The Position is where the agent believes the presence is located (belief-derived).
type PresenceCue struct {
	Type     PresenceType     `json:"type"`
	Position core.Position    `json:"position"`
}
