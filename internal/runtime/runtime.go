package runtime

import (
	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
	"github.com/divijg19/Nightshade/internal/dungeon"
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
	return &Runtime{
		tick:           0,
		agents:         agents,
		world:          w,
		board:          signal.NewBoard(5),
		boardCursor:    map[string]int{},
		dungeonByAgent: map[string]*dungeon.Instance{},
		signalByAgent:  map[string]string{},
		pendingEvents:    map[string]string{},
		pendingEjects:    map[string]string{},
		dungeonNarration: map[string]map[string]bool{},
	}
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
