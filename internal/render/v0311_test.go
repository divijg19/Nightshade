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
			Grid:            grid11(),
			Pressure:        7,
			MaxPressure:     20,
			InstabilityBand: 2,
			CoreIntegrity:   81,
			Threat:          "MEDIUM",
			ExitChanneling:  true,
			ExitChannelTick: 9,
		},
	}
	plain := stripANSI11(RenderForTest(obs))
	if !strings.Contains(plain, "DUNGEON  t=11  DANGEROUS") {
		t.Fatalf("missing canonical dungeon header")
	}
	if !strings.Contains(plain, "◼ 7/20") || !strings.Contains(plain, "◆ 81") || !strings.Contains(plain, "▲ MEDIUM") || !strings.Contains(plain, "⚡ 100/100") {
		t.Fatalf("missing canonical state line")
	}
	if strings.Count(plain, "Signal stabilized.") != 1 {
		t.Fatalf("expected one event line")
	}
	if !strings.Contains(plain, "⏳") {
		t.Fatalf("missing exit channel line")
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
	re := regexp.MustCompile(`◼\s+7/20\s+([█░]{20})`)
	m := re.FindStringSubmatch(plain)
	if len(m) < 2 {
		t.Fatalf("expected deterministic 20-segment pressure bar")
	}
}

func TestV0311_GlyphCanonAndAlignment(t *testing.T) {
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
			Enemies: []agent.EnemyView{
				{Kind: "HUNTER", X: 2, Y: 2},
				{Kind: "SENTINEL", X: 4, Y: 2, TargetLocked: true},
				{Kind: "WARDEN", X: 2, Y: 4},
			},
		},
	}
	plain := stripANSI11(RenderForTest(obs))
	for _, g := range []string{"#", "·", "A", "◇", "H", "S", "W"} {
		if !strings.Contains(plain, g) {
			t.Fatalf("expected glyph %q in render output", g)
		}
	}
	rows := []string{}
	for _, ln := range strings.Split(plain, "\n") {
		trim := strings.TrimSpace(strings.TrimRight(ln, "\r"))
		r := []rune(trim)
		if len(r) == 7 {
			rows = append(rows, trim)
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
			{ID: "S000", Type: "NULL", Anchor: "MEMORY_VAULT", Decay: 8, Corruption: 4},
			{ID: "S001", Type: "FRACTURE", Anchor: "RECOVERY_NODE", Decay: 6, Corruption: 5},
		}},
	}
	plain := stripANSI11(RenderForTest(obs))
	if !strings.Contains(plain, "== SIGNAL BOARD ==") {
		t.Fatalf("missing board header")
	}
	if !strings.Contains(plain, "◆ 4") || !strings.Contains(plain, "◆ 5") {
		t.Fatalf("missing corruption-only board metrics")
	}
	if !strings.Contains(plain, "1–5 Enter   Q Quick Dive   R Resume") {
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
	if !strings.Contains(plain, "⏳ ███░░") {
		t.Fatalf("expected deterministic 5-segment channel bar")
	}
}
