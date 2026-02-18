package runtime

import (
	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
)

type Snapshot struct {
	Tick     int
	SelfID   string
	Position core.Position
	Health   int
	Energy   int
	Visible  []core.TileView
	Known    []core.TileView

	// Presentation-only metadata for v0.3.0 UI.
	Mode  string
	Board agent.BoardView
	// Board-mode progression hints
	Fragments      int
	SkillPoints    int
	UnlockedSkills []string
	LastSignalID   string
	RunSummary     *agent.RunSummaryView
	// AvailableSkills is populated when the client requests upgrade UI
	AvailableSkills []agent.Skill
	Dungeon         agent.DungeonView
	// One-shot event message propagated to the agent (e.g., forced eject)
	Event string
	// Ejected indicates the agent was forcibly removed from a dungeon
	// during the prior authoritative tick, and EjectReason provides a
	// short label (e.g., "pressure"). These are delivery-only fields
	// intended for client presentation.
	Ejected     bool
	EjectReason string
}

func (s Snapshot) KnownTiles() []core.TileView {
	// Keep this for backward compatibility: Runtime no longer fabricates
	// Known; agents should treat Snapshot.Known as empty and rely on their
	// Memory for persistent belief. This method returns the raw Known slice
	// (typically empty).
	return s.Known
}

// VisibleTiles returns the tiles seen in the current tick.
func (s Snapshot) VisibleTiles() []core.TileView { return s.Visible }

// TickValue returns the current tick for compatibility with agent-side accessors.
func (s Snapshot) TickValue() int { return s.Tick }

// PositionValue returns the agent's current position for agent-side accessors.
// This is a lightweight accessor that exposes the authoritative position but
// does not expose any memory or age information.
func (s Snapshot) PositionValue() core.Position { return s.Position }

// ViewMode exposes the UI hint for render mode.
func (s Snapshot) ViewMode() string { return s.Mode }

// BoardValue exposes the current signal board view.
func (s Snapshot) BoardValue() agent.BoardView { return s.Board }

// DungeonValue exposes the current dungeon HUD.
func (s Snapshot) DungeonValue() agent.DungeonView { return s.Dungeon }

// EventValue exposes a one-shot event message attached to this snapshot.
func (s Snapshot) EventValue() string { return s.Event }

// Progression accessors
func (s Snapshot) FragmentsValue() int                 { return s.Fragments }
func (s Snapshot) SkillPointsValue() int               { return s.SkillPoints }
func (s Snapshot) UnlockedSkillsValue() []string       { return s.UnlockedSkills }
func (s Snapshot) AvailableSkillsValue() []agent.Skill { return s.AvailableSkills }
func (s Snapshot) LastSignalIDValue() string           { return s.LastSignalID }
func (s Snapshot) RunSummaryValue() *agent.RunSummaryView {
	return s.RunSummary
}
