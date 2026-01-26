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
