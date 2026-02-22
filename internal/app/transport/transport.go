package transport

import "github.com/divijg19/Nightshade/internal/agent"

type Command struct {
	Key string
}

type Snapshot struct {
	Observation agent.Observation
	Energy      int
}

type Transport interface {
	Send(Command) error
	Snapshots() <-chan Snapshot
	Close() error
}
