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

	// Mode is a UI hint describing what the client should render.
	// It is purely informational and must not affect simulation.
	Mode string `json:"mode,omitempty"`
	// Board is populated when Mode == "board".
	Board *BoardView `json:"board,omitempty"`
	// Dungeon is populated when Mode == "dungeon".
	Dungeon *DungeonView `json:"dungeon,omitempty"`
}

// BoardView is a compact, client-facing representation of the server-owned Signal Board.
// It is presentation-only.
type BoardView struct {
	Cursor  int          `json:"cursor"`
	Signals []SignalView `json:"signals"`
}

type SignalView struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Anchor   string `json:"anchor"`
	Zone     string `json:"zone"`
	Presence string `json:"presence"`
	Decay    int    `json:"decay"`
	Locked   bool   `json:"locked"`
	Burned   bool   `json:"burned"`
}

// DungeonView is a client-facing view of dungeon pressure/risks.
// It is presentation-only.
type DungeonView struct {
	Grid            [][]rune `json:"grid,omitempty"`
	Pressure        int      `json:"pressure"`
	MaxPressure     int      `json:"max_pressure"`
	Tick            int      `json:"tick"`
	InstabilityBand int      `json:"instability_band"`

	// ExitState is a short label: "visible", "flicker", "adjacent", "collapsed", "hidden"
	ExitState    string `json:"exit_state,omitempty"`
	// Event is a one-shot narration string (populated by server)
	Event        string `json:"event,omitempty"`

	// Legacy/forward-compat fields (unused in pressure-only step).
	ExitStability int    `json:"exit_stability,omitempty"`
	AnchorType    string `json:"anchor_type,omitempty"`
	AtAnchor      bool   `json:"at_anchor,omitempty"`
	AtExit        bool   `json:"at_exit,omitempty"`
}

// PresenceType enumerates the kinds of perceived presences.
type PresenceType string

const (
	PresenceSelf       PresenceType = "Self"
	PresenceHumanOther PresenceType = "HumanOther"
	PresenceNPC        PresenceType = "NPC"
)

// PresenceCue is an ephemeral perception about another agent or self.
// The Position is where the agent believes the presence is located (belief-derived).
type PresenceCue struct {
	Type     PresenceType  `json:"type"`
	Position core.Position `json:"position"`
}
