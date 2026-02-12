package agent

import (
	"errors"

	"github.com/divijg19/Nightshade/internal/persist"
)

// Skill represents a static skill definition.
type Skill struct {
	ID           string
	Name         string
	Description  string
	Prerequisite string
}

// AllSkills returns the static skill registry.
func AllSkills() map[string]Skill {
	skills := map[string]Skill{}
	// SURVIVAL
	skills["endurance_1"] = Skill{ID: "endurance_1", Name: "Endurance I", Description: "Increase max energy by 5.", Prerequisite: ""}
	skills["endurance_2"] = Skill{ID: "endurance_2", Name: "Endurance II", Description: "Increase max energy by 10.", Prerequisite: "endurance_1"}
	skills["anchor_mastery"] = Skill{ID: "anchor_mastery", Name: "Anchor Mastery", Description: "Anchor slows pressure every 3 ticks.", Prerequisite: ""}
	skills["exit_instinct"] = Skill{ID: "exit_instinct", Name: "Exit Instinct", Description: "Exit succeeds even exhausted in CRITICAL.", Prerequisite: ""}
	// CONTROL
	skills["extended_distract"] = Skill{ID: "extended_distract", Name: "Extended Distract", Description: "Distract lasts longer.", Prerequisite: ""}
	skills["deep_concealment"] = Skill{ID: "deep_concealment", Name: "Deep Concealment", Description: "Hide lasts longer.", Prerequisite: ""}
	skills["stability_training"] = Skill{ID: "stability_training", Name: "Stability Training", Description: "Reduce CRITICAL move penalty.", Prerequisite: ""}
	// INSIGHT
	skills["efficient_observe"] = Skill{ID: "efficient_observe", Name: "Efficient Observe", Description: "OBSERVE cost reduced.", Prerequisite: ""}
	skills["threat_awareness"] = Skill{ID: "threat_awareness", Name: "Threat Awareness", Description: "Enemy locks telegraphed earlier.", Prerequisite: ""}
	skills["pressure_sense"] = Skill{ID: "pressure_sense", Name: "Pressure Sense", Description: "Show threshold for next band.", Prerequisite: ""}
	return skills
}

// UnlockSkill validates and marks a skill unlocked in-progress.
// This function mutates the provided Progress; caller is responsible for persisting.
func UnlockSkill(progress *persist.Progress, skillID string) error {
	if progress == nil {
		return errors.New("nil progress")
	}
	skills := AllSkills()
	sk, ok := skills[skillID]
	if !ok {
		return errors.New("skill not found")
	}
	if progress.UnlockedSkills[skillID] {
		return errors.New("already unlocked")
	}
	if sk.Prerequisite != "" {
		if !progress.UnlockedSkills[sk.Prerequisite] {
			return errors.New("prerequisite not met")
		}
	}
	// SkillPoints available must be > len(unlocked)
	if progress.SkillPoints <= len(progress.UnlockedSkills) {
		return errors.New("insufficient skill points")
	}
	progress.UnlockedSkills[skillID] = true
	return nil
}
