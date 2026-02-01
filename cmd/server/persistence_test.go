package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
	"github.com/divijg19/Nightshade/internal/persist"
)

func TestServerRehydrationRestoresEnergyAndMemory(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("NIGHTSHADE_DIR", dir)

	agentID := base64.StdEncoding.EncodeToString([]byte("agent-a"))
	agentDir := filepath.Join(persist.BaseDir(), "agents", agentID)

	// write state.json
	st := map[string]interface{}{"energy": 37}
	if err := persist.WriteJSON(filepath.Join(agentDir, "state.json"), st); err != nil {
		t.Fatalf("write state: %v", err)
	}

	// write memory.json with one tile
	tiles := []map[string]int{{"x": 0, "y": 0, "glyph": int('A'), "lastSeen": 10, "scarLevel": 2}}
	memOut := map[string]interface{}{"tiles": tiles}
	if err := persist.WriteJSON(filepath.Join(agentDir, "memory.json"), memOut); err != nil {
		t.Fatalf("write memory: %v", err)
	}

	// Server-like rehydration
	var energy int = agent.MaxEnergy
	var raw struct {
		Tiles []struct {
			X         int `json:"x"`
			Y         int `json:"y"`
			Glyph     int `json:"glyph"`
			LastSeen  int `json:"lastSeen"`
			ScarLevel int `json:"scarLevel"`
		} `json:"tiles"`
	}
	// read state
	var stIn struct {
		Energy int `json:"energy"`
	}
	_ = persist.ReadJSON(filepath.Join(agentDir, "state.json"), &stIn)
	energy = stIn.Energy
	_ = persist.ReadJSON(filepath.Join(agentDir, "memory.json"), &raw)

	mem := agent.NewMemory()
	for _, trow := range raw.Tiles {
		pos := core.Position{X: trow.X, Y: trow.Y}
		mt := agent.MemoryTile{Tile: core.TileView{Position: pos, Glyph: rune(trow.Glyph), Visible: true}, LastSeen: trow.LastSeen, ScarLevel: trow.ScarLevel}
		mem.SetMemoryTile(pos, mt)
	}

	rh := agent.NewRemoteHumanFromExisting(agentID, mem, energy)
	if rh.Energy() != 37 {
		t.Fatalf("energy not restored: %d", rh.Energy())
	}
	if mt, ok := rh.Memory().GetMemoryTile(core.Position{X: 0, Y: 0}); !ok {
		t.Fatalf("memory tile missing")
	} else {
		if mt.LastSeen != 10 || mt.ScarLevel != 2 {
			t.Fatalf("memory tile fields incorrect: %+v", mt)
		}
	}
}
