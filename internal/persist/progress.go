package persist

import (
	"path/filepath"
)

// Progress captures persistent agent progression state.
type Progress struct {
	Fragments              int             `json:"fragments"`
	SkillPoints            int             `json:"skill_points"`
	TotalDungeons          int             `json:"total_dungeons"`
	HighestPressureReached int             `json:"highest_pressure_reached"`
	UnlockedSkills         map[string]bool `json:"unlocked_skills"`
	DungeonIntroShown      bool            `json:"dungeon_intro_shown,omitempty"`
	LastSignalID           string          `json:"last_signal_id,omitempty"`
}

// defaultProgress returns an initialized Progress structure.
func DefaultProgress() *Progress { return defaultProgress() }

func defaultProgress() *Progress {
	return &Progress{
		Fragments:              0,
		SkillPoints:            0,
		TotalDungeons:          0,
		HighestPressureReached: 0,
		UnlockedSkills:         map[string]bool{},
		DungeonIntroShown:      false,
		LastSignalID:           "",
	}
}

// progressPath returns the file path for an agent's progress file.
func progressPath(agentID string) string {
	return filepath.Join(BaseDir(), "agents", agentID, "progress.json")
}

// LoadProgress loads agent progress from disk; returns default progress
// if file missing or unreadable. Never panics.
func LoadProgress(agentID string) (*Progress, error) {
	p := defaultProgress()
	path := progressPath(agentID)
	if err := ReadJSON(path, p); err != nil {
		// Missing or unreadable file — return default without error to be
		// backward-compatible.
		return p, nil
	}
	if p.UnlockedSkills == nil {
		p.UnlockedSkills = map[string]bool{}
	}
	return p, nil
}

// SaveProgress persists progress atomically to the agent's progress file.
func SaveProgress(agentID string, p *Progress) error {
	path := progressPath(agentID)
	// Recalculate SkillPoints deterministically from Fragments before saving.
	p.SkillPoints = p.Fragments / 10
	return WriteJSONAtomic(path, p, 0o644)
}
