package persist

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultProgressAndAtomicSaveLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NIGHTSHADE_DIR", dir)
	id := "P1"
	p := DefaultProgress()
	p.Fragments = 15
	if err := SaveProgress(id, p); err != nil {
		t.Fatalf("save: %v", err)
	}
	// file should exist
	path := filepath.Join(dir, "agents", id, "progress.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected progress file: %v", err)
	}
	// reload
	rp, err := LoadProgress(id)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if rp.Fragments != 15 {
		t.Fatalf("expected fragments 15 got %d", rp.Fragments)
	}
}

func TestCorruptedJSONFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NIGHTSHADE_DIR", dir)
	id := "P2"
	pdir := filepath.Join(dir, "agents", id)
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// write corrupted file
	path := filepath.Join(pdir, "progress.json")
	if err := os.WriteFile(path, []byte("{not valid json}"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	rp, err := LoadProgress(id)
	if err != nil {
		t.Fatalf("load should not return error on corrupt: %v", err)
	}
	if rp.Fragments != 0 || rp.SkillPoints != 0 {
		t.Fatalf("expected default progress on corrupt file")
	}
}

func TestTwoAgentsDoNotShareProgressPointers(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NIGHTSHADE_DIR", dir)
	a := "AA"
	b := "BB"
	pa := DefaultProgress()
	pa.Fragments = 10
	pb := DefaultProgress()
	pb.Fragments = 20
	if err := SaveProgress(a, pa); err != nil {
		t.Fatalf("save a: %v", err)
	}
	if err := SaveProgress(b, pb); err != nil {
		t.Fatalf("save b: %v", err)
	}
	ra, _ := LoadProgress(a)
	rb, _ := LoadProgress(b)
	if ra == rb {
		t.Fatalf("expected separate progress instances, got same pointer")
	}
	if ra.Fragments == rb.Fragments {
		t.Fatalf("expected different fragments values between agents")
	}
}
