package agent

type Action int
type Snapshot interface{}

type Agent interface {
	ID() string
	Decide(snapshot Snapshot) Action
}

const (
	MOVE_N  Action = 0
	MOVE_S  Action = 1
	MOVE_E  Action = 2
	MOVE_W  Action = 3
	GATHER  Action = 4
	ATTACK  Action = 5
	HIDE    Action = 6
	OBSERVE Action = 7
	WAIT    Action = 8
)

// CautionThreshold defines how many ticks since last observation make a
// tile "risky". If Age > CautionThreshold agents will hesitate.
const CautionThreshold = 3

// ParanoiaThreshold defines how old a belief must be before it becomes
// a hallucinated visible item injected into an agent's Observation.Visible.
// See v0.0.7 spec: beliefs with Age > ParanoiaThreshold are hallucinated.
const ParanoiaThreshold = 6
