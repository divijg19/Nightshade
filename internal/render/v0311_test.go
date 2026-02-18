package render

import (
	"regexp"
	"strings"
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
)

func stripANSI11(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

func grid11() [][]rune {
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

func TestV0311_DungeonHeaderAndOrder(t *testing.T) {
	obs := agent.Observation{
		Tick:     11,
		Mode:     "dungeon",
		Position: core.Position{X: 1, Y: 1},
		Event:    "Signal stabilized.",
		Dungeon: &agent.DungeonView{
			Grid:             grid11(),
			Pressure:         7,
			MaxPressure:      20,
			InstabilityBand:  2,
			InstabilityLabel: "Dangerous",
			CoreIntegrity:    81,
			Threat:           "MEDIUM",
			ExitChanneling:   true,
			ExitChannelTick:  9,
		},
	}
	plain := stripANSI11(RenderForTest(obs))
	if !strings.Contains(plain, "DUNGEON tick 11") {
		t.Fatalf("missing v0.3.11 dungeon header")
	}
	if !strings.Contains(plain, "PRESSURE:") {
		t.Fatalf("missing state line")
	}
	if !strings.Contains(plain, "Signal stabilized.") {
		t.Fatalf("missing one-line event")
	}
	if !strings.Contains(plain, "EXIT CHANNEL:") {
		t.Fatalf("missing exit channel line")
	}
	if !strings.Contains(plain, "Energy 100/100") {
		t.Fatalf("missing hud energy line")
	}
}

func TestV0311_PressureBarSegments(t *testing.T) {
	obs := agent.Observation{
		Tick:     1,
		Mode:     "dungeon",
		Position: core.Position{X: 1, Y: 1},
		Dungeon: &agent.DungeonView{
			Grid:            grid11(),
			Pressure:        7,
			MaxPressure:     20,
			InstabilityBand: 1,
			CoreIntegrity:   90,
			Threat:          "LOW",
		},
	}
	plain := stripANSI11(RenderForTest(obs))
	if !strings.Contains(plain, "PRESSURE: ███████░░░░░░░░░░░░░  7 / 20") {
		t.Fatalf("expected deterministic 20-segment pressure bar")
	}
}

func TestV0311_UnicodeSubstitutionAndAlignment(t *testing.T) {
	obs := agent.Observation{
		Tick:     2,
		Mode:     "dungeon",
		Position: core.Position{X: 1, Y: 1},
		Dungeon: &agent.DungeonView{
			Grid:            grid11(),
			Pressure:        3,
			MaxPressure:     20,
			InstabilityBand: 0,
			CoreIntegrity:   99,
			Threat:          "LOW",
			Enemies:         []agent.EnemyView{{Kind: "HUNTER", X: 2, Y: 2}, {Kind: "SENTINEL", X: 4, Y: 2}, {Kind: "WARDEN", X: 2, Y: 4}},
		},
	}
	plain := stripANSI11(RenderForTest(obs))
	for _, g := range []string{"▓", "·", "◉", "◇", "⚔", "☣", "✶"} {
		if !strings.Contains(plain, g) {
			t.Fatalf("expected glyph %q in render output", g)
		}
	}
	// crude alignment check: every dungeon row should have equal rune length
	rows := []string{}
	for _, ln := range strings.Split(plain, "\n") {
		if strings.Contains(ln, "▓") || strings.Contains(ln, "·") || strings.Contains(ln, "◇") || strings.Contains(ln, "◉") {
			rows = append(rows, strings.TrimSpace(ln))
		}
	}
	if len(rows) < 3 {
		t.Fatalf("expected multiple grid rows")
	}
	w := len([]rune(rows[0]))
	for i := 1; i < len(rows); i++ {
		if len([]rune(rows[i])) != w {
			t.Fatalf("expected aligned rows, got %d and %d", w, len([]rune(rows[i])))
		}
	}
}

func TestV0311_BoardCompressionLayout(t *testing.T) {
	obs := agent.Observation{
		Tick:         250,
		Mode:         "board",
		LastSignalID: "S001",
		Board: &agent.BoardView{Signals: []agent.SignalView{
			{ID: "S000", Type: "NULL", Decay: 8, Corruption: 4},
			{ID: "S001", Type: "FRACTURE", Decay: 6, Corruption: 5},
		}},
	}
	plain := stripANSI11(RenderForTest(obs))
	if !strings.Contains(plain, "WORLD Epoch 2") {
		t.Fatalf("missing board header")
	}
	if !strings.Contains(plain, "C:▓▓▓▓░") {
		t.Fatalf("missing corruption bar")
	}
	if !strings.Contains(plain, "★") || !strings.Contains(plain, "↺") {
		t.Fatalf("missing most-corrupted/last-entered highlights")
	}
	if !strings.Contains(plain, "Q Quick Dive") || !strings.Contains(plain, "R Resume Last") {
		t.Fatalf("missing board shortcut hints")
	}
}

func TestV0311_ExitChannelBarFill(t *testing.T) {
	obs := agent.Observation{
		Tick:     13,
		Mode:     "dungeon",
		Position: core.Position{X: 1, Y: 1},
		Dungeon: &agent.DungeonView{
			Grid:            grid11(),
			Pressure:        15,
			MaxPressure:     20,
			InstabilityBand: 3,
			ExitChanneling:  true,
			ExitChannelTick: 11,
			CoreIntegrity:   75,
			Threat:          "HIGH",
		},
	}
	plain := stripANSI11(RenderForTest(obs))
	if !strings.Contains(plain, "EXIT CHANNEL: ███░░") {
		t.Fatalf("expected deterministic 5-segment channel bar")
	}
}
