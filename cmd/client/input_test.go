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
