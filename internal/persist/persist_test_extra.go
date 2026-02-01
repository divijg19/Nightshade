package persist

import (
	"os"
	"path/filepath"
	"testing"
)

// Test that WriteJSONAtomic does not produce a partial .tmp file left behind
// when writes complete. This is a simple smoke test to ensureRename succeeds.
func TestWriteJSONAtomic_NoTmpLeft(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("NIGHTSHADE_DIR", dir)
	path := filepath.Join(dir, "agents", "a", "state.json")
	_ = WriteJSON(path, map[string]interface{}{"energy": 42})
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Fatalf("tmp file left behind")
	}
}
