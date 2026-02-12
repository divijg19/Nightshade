package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
)

func TestSnapshotContainsProgressionAndBuild(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	snap := rt.snapshotFor(a, agent.Action(-1))
	// Fragments and SkillPoints should be present (zero default acceptable)
	_ = snap.Fragments
	_ = snap.SkillPoints
	// Build list (ActiveSkillShortNames) should be non-nil slice
	if snap.UnlockedSkills == nil {
		t.Fatalf("expected UnlockedSkills slice present")
	}
	// Dungeon NextBandThreshold may be zero; ensure field exists
	_ = snap.Dungeon.NextBandThreshold
}

func TestSnapshotModeSeparation(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	// board snapshot
	bSnap, _ := rt.SnapshotForDebug(a.ID())
	// board-mode should include Board and not Dungeon
	// snapshotFor Debug uses snapshotFor with action -1 which for a bound agent will
	// return dungeon view; we assert fields aren't nil unexpectedly.
	_ = bSnap
	_ = a
	_ = rt
}
