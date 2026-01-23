package agent

type Action int
type Snapshot interface{}

type Agent interface {
	ID() string
	Decide(snapshot Snapshot) Action
}
