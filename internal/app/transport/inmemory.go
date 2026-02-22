package transport

import (
	"errors"
	"sync"
)

var ErrTransportClosed = errors.New("transport closed")

type InMemoryTransport struct {
	commands  chan Command
	snapshots chan Snapshot
	closeCh   chan struct{}
	closeOnce sync.Once
}

func NewInMemoryTransport(buffer int) *InMemoryTransport {
	if buffer < 1 {
		buffer = 1
	}
	return &InMemoryTransport{
		commands:  make(chan Command, buffer),
		snapshots: make(chan Snapshot, buffer),
		closeCh:   make(chan struct{}),
	}
}

func (t *InMemoryTransport) Send(cmd Command) error {
	select {
	case <-t.closeCh:
		return ErrTransportClosed
	case t.commands <- cmd:
		return nil
	}
}

func (t *InMemoryTransport) Snapshots() <-chan Snapshot { return t.snapshots }
func (t *InMemoryTransport) Commands() <-chan Command   { return t.commands }

func (t *InMemoryTransport) Publish(s Snapshot) error {
	select {
	case <-t.closeCh:
		return ErrTransportClosed
	case t.snapshots <- s:
		return nil
	}
}

func (t *InMemoryTransport) Close() error {
	t.closeOnce.Do(func() {
		close(t.closeCh)
		close(t.snapshots)
		close(t.commands)
	})
	return nil
}

var _ Transport = (*InMemoryTransport)(nil)
