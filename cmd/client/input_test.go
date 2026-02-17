package main

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
)

func TestNormalizeKey(t *testing.T) {
	cases := []struct {
		in   byte
		want rune
		ok   bool
	}{
		{in: 'W', want: 'w', ok: true},
		{in: 'w', want: 'w', ok: true},
		{in: 13, want: '.', ok: true},
		{in: 10, want: '.', ok: true},
		{in: 0, want: 0, ok: false},
	}
	for _, c := range cases {
		got, ok := normalizeKey(c.in)
		if ok != c.ok || got != c.want {
			t.Fatalf("normalizeKey(%v) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestActionFromKey(t *testing.T) {
	cases := []struct {
		in   rune
		want string
		ok   bool
	}{
		{in: 'w', want: "w", ok: true},
		{in: 'a', want: "a", ok: true},
		{in: 's', want: "s", ok: true},
		{in: 'd', want: "d", ok: true},
		{in: 'e', want: "e", ok: true},
		{in: '.', want: ".", ok: true},
		{in: 'x', want: "", ok: false},
	}
	for _, c := range cases {
		got, ok := actionFromKey(c.in)
		if ok != c.ok || got != c.want {
			t.Fatalf("actionFromKey(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestDescribeActionKey(t *testing.T) {
	for _, c := range []struct {
		in   rune
		want string
		ok   bool
	}{
		{'w', "Move NORTH (-1)", true},
		{'a', "Move WEST (-1)", true},
		{'s', "Move SOUTH (-1)", true},
		{'d', "Move EAST (-1)", true},
		{'.', "Wait (+1)", true},
		{'e', "Observe (-1)", true},
		{'?', "", false},
	} {
		got, ok := describeActionKey(c.in)
		if ok != c.ok {
			t.Fatalf("describeActionKey(%q) ok=%v want %v", c.in, ok, c.ok)
		}
		if got != c.want {
			t.Fatalf("describeActionKey(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestMovementDelta(t *testing.T) {
	for _, c := range []struct {
		in     rune
		dx, dy int
		ok     bool
	}{
		{'w', 0, -1, true},
		{'a', -1, 0, true},
		{'s', 0, 1, true},
		{'d', 1, 0, true},
		{'.', 0, 0, false},
		{'e', 0, 0, false},
	} {
		dx, dy, ok := movementDelta(c.in)
		if ok != c.ok {
			t.Fatalf("movementDelta(%q) ok=%v want %v", c.in, ok, c.ok)
		}
		if ok && (dx != c.dx || dy != c.dy) {
			t.Fatalf("movementDelta(%q)=(%d,%d) want (%d,%d)", c.in, dx, dy, c.dx, c.dy)
		}
	}
}

func TestBuildIntrospectionLine(t *testing.T) {
	obs := agent.Observation{
		Known: []agent.Belief{
			{Tile: core.TileView{}, Age: 0, ScarLevel: 0},
			{Tile: core.TileView{}, Age: agent.CautionThreshold, ScarLevel: 1},
			{Tile: core.TileView{}, Age: agent.ParanoiaThreshold + 1, ScarLevel: 0},
		},
	}
	line := buildIntrospectionLine(obs)
	if line == "" {
		t.Fatalf("expected non-empty introspection line")
	}
}

func TestQuickDiveSelectsHighestCorruption(t *testing.T) {
	signals := []agent.SignalView{
		{ID: "S000", Decay: 8, Burned: false, Locked: false},
		{ID: "S001", Decay: 3, Burned: false, Locked: false},
		{ID: "S002", Decay: 6, Burned: false, Locked: false},
	}
	sid, ok := quickDiveSignalID(signals)
	if !ok {
		t.Fatalf("expected quick dive signal")
	}
	if sid != "S001" {
		t.Fatalf("expected S001, got %s", sid)
	}
}

func TestResumeLastOnlyWhenAvailable(t *testing.T) {
	signals := []agent.SignalView{
		{ID: "S000", Burned: false, Locked: false},
		{ID: "S001", Burned: true, Locked: false},
		{ID: "S002", Burned: false, Locked: true},
	}
	if !canResumeSignal(signals, "S000") {
		t.Fatalf("expected resume allowed for S000")
	}
	if canResumeSignal(signals, "S001") {
		t.Fatalf("expected resume denied for burned signal")
	}
	if canResumeSignal(signals, "S002") {
		t.Fatalf("expected resume denied for locked signal")
	}
}
