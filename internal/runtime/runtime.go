package runtime

import (
	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
	"github.com/divijg19/Nightshade/internal/dungeon"
	"github.com/divijg19/Nightshade/internal/persist"
	"github.com/divijg19/Nightshade/internal/signal"
	"github.com/divijg19/Nightshade/internal/world"
)

const defaultVisibilityRadius = 2

type Runtime struct {
	tick        int
	agents      []agent.Agent
	world       *world.World
	board       *signal.Board
	boardCursor map[string]int
	// v0.3.0: dungeon commitment spine.
	dungeonByAgent map[string]*dungeon.Instance
	signalByAgent  map[string]string // agentID -> locked signal ID (while inside dungeon)

	// v0.3.1: per-agent one-shot events and narration tracking
	pendingEvents    map[string]string          // agentID -> event message shown once in next snapshot
	pendingEjects    map[string]string          // agentID -> eject reason (delivered in next snapshot)
	dungeonNarration map[string]map[string]bool // agentID -> map[eventName]seen

	// v0.3.5: per-agent cooldowns for dungeon actions
	hideCooldown     map[string]int
	distractCooldown map[string]int
	// progression store loaded per-agent
	progressByAgent map[string]*persist.Progress
	// per-run stats for currently-bound dungeons
	runStats map[string]struct {
		FragmentsEarnedThisRun int
		HighestPressureThisRun int
	}
	// v0.3.11 run summary delivery (one-frame, enter-to-dismiss)
	runSummaryByAgent  map[string]*agent.RunSummaryView
	runSummaryAwaitAck map[string]bool
	runSummaryShown    map[string]bool
}

func New(agents []agent.Agent) *Runtime {
	// Use the package bounds constants to construct the world so tests
	// that reference `world.Width`/`world.Height` match runtime size.
	w := world.New(world.Width, world.Height)

	for i, a := range agents {
		w.SetPosition(a.ID(), world.Position{
			X: i,
			Y: 0,
		})
	}
	rt := &Runtime{
		tick:             0,
		agents:           agents,
		world:            w,
		board:            signal.NewBoard(5),
		boardCursor:      map[string]int{},
		dungeonByAgent:   map[string]*dungeon.Instance{},
		signalByAgent:    map[string]string{},
		pendingEvents:    map[string]string{},
		pendingEjects:    map[string]string{},
		dungeonNarration: map[string]map[string]bool{},
		hideCooldown:     map[string]int{},
		distractCooldown: map[string]int{},
		progressByAgent:  map[string]*persist.Progress{},
		runStats: map[string]struct {
			FragmentsEarnedThisRun int
			HighestPressureThisRun int
		}{},
		runSummaryByAgent:  map[string]*agent.RunSummaryView{},
		runSummaryAwaitAck: map[string]bool{},
		runSummaryShown:    map[string]bool{},
	}
	// Preload progress for all agents (backward-compatible: missing files yield defaults)
	for _, a := range agents {
		if p, err := persist.LoadProgress(a.ID()); err == nil {
			rt.progressByAgent[a.ID()] = p
		} else {
			rt.progressByAgent[a.ID()] = persist.DefaultProgress()
		}
	}
	// Apply per-agent endurance bonuses to agent instances
	for _, a := range agents {
		if p, ok := rt.progressByAgent[a.ID()]; ok && p != nil {
			bonus := 0
			if p.UnlockedSkills["endurance_2"] {
				bonus = 10
			} else if p.UnlockedSkills["endurance_1"] {
				bonus = 5
			}
			if setter, ok := a.(interface{ SetMaxEnergyBonus(int) }); ok {
				setter.SetMaxEnergyBonus(bonus)
			}
		}
	}
	return rt
}

func (r *Runtime) Tick() int {
	return r.tick
}

func (r *Runtime) advanceTick() {
	r.tick++
}

func (r *Runtime) SnapshotForDebug(agentID string) (Snapshot, bool) {
	for _, a := range r.agents {
		if a.ID() == agentID {
			return r.snapshotFor(a, agent.Action(-1)), true
		}
	}
	return Snapshot{}, false
}

// MarkerPosition returns the authoritative marker position in world
// coordinates. This is a debug accessor and does not expose agent memory
// or change any semantics.
func (r *Runtime) MarkerPosition() core.Position {
	mp := r.world.MarkerPosition()
	return core.Position{X: mp.X, Y: mp.Y}
}
