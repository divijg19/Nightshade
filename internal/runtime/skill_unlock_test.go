package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/persist"
)

// ensure isolation: use temp NIGHTSHADE_DIR
func withTempPersist(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("", "nightshade-test-")
	if err != nil {
		t.Fatalf("tmpdir: %v", err)
	}
	t.Setenv("NIGHTSHADE_DIR", d)
	return d
}

func TestInvalidUnlockScenarios(t *testing.T) {
	dir := withTempPersist(t)
	defer os.RemoveAll(dir)

	agentID := "A"
	// start with default progress on disk
	p := persist.DefaultProgress()
	if err := persist.SaveProgress(agentID, p); err != nil {
		t.Fatalf("save progress: %v", err)
	}

	// 1. nonexistent skill
	pn := persist.DefaultProgress()
	if err := agent.UnlockSkill(pn, "no-such-skill"); err == nil {
		t.Fatalf("expected error unlocking nonexistent skill")
	}
	// disk must be unchanged
	if _, err := os.Stat(filepath.Join(dir, "agents", agentID, "progress.json")); err != nil {
		t.Fatalf("expected progress file exists: %v", err)
	}

	// 2. prereq not met
	p2 := persist.DefaultProgress()
	// attempt to unlock endurance_2 (requires endurance_1)
	if err := agent.UnlockSkill(p2, "endurance_2"); err == nil {
		t.Fatalf("expected prerequisite error when unlocking endurance_2")
	}

	// 3. insufficient skill points
	p3 := persist.DefaultProgress()
	p3.Fragments = 0 // skillpoints 0
	if err := agent.UnlockSkill(p3, "endurance_1"); err == nil {
		t.Fatalf("expected insufficient skill points error")
	}

	// 4. already unlocked
	p4 := persist.DefaultProgress()
	p4.Fragments = 20 // 2 skill points
	p4.UnlockedSkills["endurance_1"] = true
	if err := agent.UnlockSkill(p4, "endurance_1"); err == nil {
		t.Fatalf("expected already unlocked error")
	}
}

func TestValidUnlockAndPersistence(t *testing.T) {
	dir := withTempPersist(t)
	defer os.RemoveAll(dir)

	agentID := "B"
	// give enough fragments for 2 skill points
	p := persist.DefaultProgress()
	p.Fragments = 20
	if err := persist.SaveProgress(agentID, p); err != nil {
		t.Fatalf("save progress: %v", err)
	}

	// load and unlock endurance_1 then endurance_2 (prereq)
	lp, err := persist.LoadProgress(agentID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// ensure starting state
	if lp.UnlockedSkills["endurance_1"] {
		t.Fatalf("unexpected unlocked skill in fresh progress")
	}

	if err := agent.UnlockSkill(lp, "endurance_1"); err != nil {
		t.Fatalf("unlock endurance_1: %v", err)
	}
	// persist (runtime normally persists; tests validate SaveProgress)
	if err := persist.SaveProgress(agentID, lp); err != nil {
		t.Fatalf("save after unlock: %v", err)
	}

	// reload from disk and verify
	rp, err := persist.LoadProgress(agentID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !rp.UnlockedSkills["endurance_1"] {
		t.Fatalf("expected endurance_1 persisted")
	}
	// skill points recalculated from fragments
	if rp.SkillPoints != rp.Fragments/10 {
		t.Fatalf("skillpoints recomputed on save; got %d want %d", rp.SkillPoints, rp.Fragments/10)
	}

	// attempt to unlock endurance_2 now (prereq met)
	if err := agent.UnlockSkill(rp, "endurance_2"); err != nil {
		t.Fatalf("unlock endurance_2: %v", err)
	}
	if err := persist.SaveProgress(agentID, rp); err != nil {
		t.Fatalf("save after unlock2: %v", err)
	}

	// reload runtime and confirm New() will pick it up
	a := agent.NewRemoteHumanFromExisting(agentID, agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})
	if p2, ok := rt.progressByAgent[agentID]; !ok || p2 == nil {
		t.Fatalf("runtime did not load progress for agent")
	} else {
		if !p2.UnlockedSkills["endurance_2"] {
			t.Fatalf("runtime did not reflect unlocked endurance_2")
		}
	}
}
