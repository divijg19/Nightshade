package render

import (
	"regexp"
	"strings"
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
)

func stripANSI(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

func testGrid() [][]rune {
	return [][]rune{
		[]rune("#######"),
		[]rune("#.....#"),
		[]rune("#.....#"),
		[]rune("#..A..#"),
		[]rune("#.....#"),
		[]rune("#.....#"),
		[]rune("###X###"),
	}
}

func TestV0310_HeaderWithInstabilityAndEnrage(t *testing.T) {
	obs := agent.Observation{
		Tick:     7,
		Mode:     "dungeon",
		Position: core.Position{X: 1, Y: 1},
		Dungeon: &agent.DungeonView{
			Grid:             testGrid(),
			Pressure:         16,
			MaxPressure:      20,
			InstabilityBand:  3,
			InstabilityLabel: "Critical",
			Enraged:          true,
			CoreIntegrity:    72,
			Threat:           "HIGH",
		},
	}
	out := stripANSI(RenderForTest(obs))
	if !strings.Contains(out, "DUNGEON  tick 7") {
		t.Fatalf("missing dungeon header")
	}
	if !strings.Contains(out, "[CRITICAL]") {
		t.Fatalf("missing instability label")
	}
	if !strings.Contains(out, "ENRAGE 1") {
		t.Fatalf("missing enrage label")
	}
}

func TestV0310_PressureBarRendering(t *testing.T) {
	obs := agent.Observation{
		Tick:     1,
		Mode:     "dungeon",
		Position: core.Position{X: 1, Y: 1},
		Dungeon: &agent.DungeonView{
			Grid:            testGrid(),
			Pressure:        4,
			MaxPressure:     8,
			InstabilityBand: 0,
			CoreIntegrity:   95,
			Threat:          "LOW",
		},
	}
	out := stripANSI(RenderForTest(obs))
	if !strings.Contains(out, "||||----") {
		t.Fatalf("expected pressure bar in output")
	}
}

func TestV0310_ExitChannelBarRendering(t *testing.T) {
	obs := agent.Observation{
		Tick:     4,
		Mode:     "dungeon",
		Position: core.Position{X: 1, Y: 1},
		Dungeon: &agent.DungeonView{
			Grid:            testGrid(),
			Pressure:        17,
			MaxPressure:     20,
			InstabilityBand: 3,
			Enraged:         true,
			ExitChanneling:  true,
			ExitChannelTick: 2,
			CoreIntegrity:   70,
			Threat:          "HIGH",
		},
	}
	out := stripANSI(RenderForTest(obs))
	if !strings.Contains(out, "EXIT CHANNEL: ███░░") {
		t.Fatalf("expected exit channel bar")
	}
}

func TestV0310_OneLineNarrationRule(t *testing.T) {
	obs := agent.Observation{
		Tick:     9,
		Mode:     "dungeon",
		Position: core.Position{X: 1, Y: 1},
		Event:    "Signal stabilized.",
		Dungeon: &agent.DungeonView{
			Grid:            testGrid(),
			Pressure:        8,
			MaxPressure:     20,
			InstabilityBand: 1,
			CoreIntegrity:   90,
			Threat:          "MEDIUM",
			Event:           "The air feels unstable.",
			ExitChanneling:  true,
			ExitChannelTick: 7,
		},
	}
	plain := stripANSI(RenderForTest(obs))
	if strings.Count(plain, "Signal stabilized.") != 1 {
		t.Fatalf("expected single top-priority event")
	}
	if strings.Contains(plain, "The air feels unstable.") {
		t.Fatalf("expected lower-priority event to be filtered")
	}
	if strings.Contains(plain, "EXIT CHANNEL:") {
		t.Fatalf("expected channel progress hidden when higher-priority event exists")
	}
}

func TestV0310_BoardCompressionLayout(t *testing.T) {
	obs := agent.Observation{
		Tick: 210,
		Mode: "board",
		Board: &agent.BoardView{Signals: []agent.SignalView{
			{ID: "S000", Type: "NULL", Decay: 8},
			{ID: "S001", Type: "FRACTURE", Decay: 3},
		}},
	}
	plain := stripANSI(RenderForTest(obs))
	if !strings.Contains(plain, "WORLD Epoch 2") {
		t.Fatalf("missing compressed board header")
	}
	if !strings.Contains(plain, "[1] NULL  S:8  C:4") {
		t.Fatalf("missing compact board row")
	}
	if !strings.Contains(plain, "Q Quick Dive") || !strings.Contains(plain, "R Resume Last") {
		t.Fatalf("missing board hint line with shortcuts")
	}
}
