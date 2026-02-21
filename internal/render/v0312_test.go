package render

import (
	"regexp"
	"strings"
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
)

func testGrid12() [][]rune {
	return [][]rune{
		[]rune("#######"),
		[]rune("#.....#"),
		[]rune("#..A..#"),
		[]rune("#.....#"),
		[]rune("###X###"),
	}
}

func TestV0312_DungeonHierarchyOrder(t *testing.T) {
	obs := agent.Observation{
		Tick:     10,
		Mode:     "dungeon",
		Position: core.Position{X: 1, Y: 1},
		Event:    "Event line",
		Dungeon: &agent.DungeonView{
			Grid:            testGrid12(),
			Pressure:        6,
			MaxPressure:     20,
			CoreIntegrity:   90,
			Threat:          "MEDIUM",
			InstabilityBand: 1,
			ExitChanneling:  true,
			ExitChannelTick: 9,
		},
	}
	plain := stripANSI11(RenderForTest(obs))
	ixHeader := strings.Index(plain, "DUNGEON  t=10")
	ixState := strings.Index(plain, "◼ 6/20")
	ixChannel := strings.Index(plain, "⏳")
	ixGrid := strings.Index(plain, "#")
	ixEvent := strings.Index(plain, "Event line")
	if !(ixHeader >= 0 && ixState > ixHeader && ixChannel > ixState && ixGrid > ixChannel && ixEvent > ixGrid) {
		t.Fatalf("unexpected dungeon hierarchy order")
	}
	if strings.Contains(plain, "\r\n\r\n") {
		t.Fatalf("expected no extra blank lines in dungeon frame")
	}
}

func TestV0312_OneEventPerFrameInvariant(t *testing.T) {
	obs := agent.Observation{
		Tick:     4,
		Mode:     "dungeon",
		Position: core.Position{X: 1, Y: 1},
		Event:    "Top event\nLower event",
		Dungeon: &agent.DungeonView{
			Grid:            testGrid12(),
			Pressure:        8,
			MaxPressure:     20,
			CoreIntegrity:   85,
			Threat:          "HIGH",
			InstabilityBand: 2,
		},
	}
	plain := stripANSI11(RenderForTest(obs))
	if strings.Count(plain, "Top event") != 1 {
		t.Fatalf("expected exactly one top event line")
	}
	if strings.Contains(plain, "Lower event") {
		t.Fatalf("expected no duplicate/secondary narration")
	}
}

func TestV0312_StateLineOrderAndEnergyDelta(t *testing.T) {
	obs := agent.Observation{
		Tick:     6,
		Mode:     "dungeon",
		Position: core.Position{X: 1, Y: 1},
		Dungeon: &agent.DungeonView{
			Grid:            testGrid12(),
			Pressure:        12,
			MaxPressure:     20,
			CoreIntegrity:   77,
			Threat:          "HIGH",
			InstabilityBand: 3,
		},
	}
	var b strings.Builder
	RenderTo(&b, obs, 14, 3, 0, "", "→ Move NORTH (-1)  Energy 14/100 (+2)")
	plain := stripANSI11(b.String())
	if !strings.Contains(plain, "◼ 12/20   ◆ 77   ▲ HIGH   ⚡ 14/100 (+2)") {
		t.Fatalf("state line order or energy delta is incorrect")
	}
}

func TestV0312_BoardCompressionAndHeader(t *testing.T) {
	obs := agent.Observation{
		Tick: 220,
		Mode: "board",
		Board: &agent.BoardView{Cursor: 0, Signals: []agent.SignalView{
			{ID: "S000", Type: "FRACTURE", Anchor: "MEMORY_VAULT", Corruption: 12, Decay: 8},
			{ID: "S001", Type: "NULL", Anchor: "RECOVERY_NODE", Corruption: 6, Decay: 6},
			{ID: "S002", Type: "NULL", Anchor: "MEMORY_VAULT", Corruption: 0, Decay: 0, Burned: true},
		}},
	}
	plain := stripANSI11(RenderForTest(obs))
	if !strings.Contains(plain, "== SIGNAL BOARD ==") {
		t.Fatalf("missing canonical board header")
	}
	if !strings.Contains(plain, "> [1] ✖ VAULT") || !strings.Contains(plain, "  [2] ○ NODE") || !strings.Contains(plain, "  [3] ✓ VAULT") {
		t.Fatalf("missing compressed board row mapping")
	}
	if strings.Contains(plain, "S:") {
		t.Fatalf("decay should be hidden in board compression")
	}
}

func TestV0312_EnemyLockVisualAndNoWrap80(t *testing.T) {
	obs := agent.Observation{
		Tick:     8,
		Mode:     "dungeon",
		Position: core.Position{X: 1, Y: 1},
		Dungeon: &agent.DungeonView{
			Grid:            testGrid12(),
			Pressure:        5,
			MaxPressure:     20,
			CoreIntegrity:   88,
			Threat:          "LOW",
			InstabilityBand: 1,
			Enemies:         []agent.EnemyView{{Kind: "SENTINEL", X: 2, Y: 2, TargetLocked: true}},
		},
	}
	out := RenderForTest(obs)
	if !strings.Contains(out, ANSIRedBright+"S"+ANSIReset) {
		t.Fatalf("expected deterministic lock visual mapping for locked enemy")
	}
	plain := stripANSI11(out)
	for _, line := range strings.Split(plain, "\n") {
		if len([]rune(strings.TrimRight(line, "\r"))) > 80 {
			t.Fatalf("line exceeds 80 columns")
		}
	}
	re := regexp.MustCompile(`◼\s+5/20\s+([█░]{20})`)
	if !re.MatchString(plain) {
		t.Fatalf("pressure bar width invariant failed")
	}
}
