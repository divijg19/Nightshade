package transport

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/divijg19/Nightshade/internal/agent"
	nnet "github.com/divijg19/Nightshade/internal/net"
)

type helloMsg struct {
	Type      string `json:"type"`
	PublicKey string `json:"public_key"`
}

type inputMsg struct {
	Type string `json:"type"`
	Key  string `json:"key"`
}

type disconnectMsg struct {
	Type string `json:"type"`
}

type NetworkTransport struct {
	conn      net.Conn
	snapshots chan Snapshot
	writeMu   sync.Mutex
	closeOnce sync.Once
}

func NewNetworkTransport(network, address, publicKey string) (*NetworkTransport, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	if err := nnet.WriteFrame(conn, helloMsg{Type: "hello", PublicKey: publicKey}); err != nil {
		_ = conn.Close()
		return nil, err
	}
	t := &NetworkTransport{conn: conn, snapshots: make(chan Snapshot, 64)}
	go t.readLoop()
	return t, nil
}

func (t *NetworkTransport) Send(cmd Command) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	return nnet.WriteFrame(t.conn, inputMsg{Type: "input", Key: cmd.Key})
}

func (t *NetworkTransport) Snapshots() <-chan Snapshot {
	return t.snapshots
}

func (t *NetworkTransport) Close() error {
	var outErr error
	t.closeOnce.Do(func() {
		t.writeMu.Lock()
		_ = nnet.WriteFrame(t.conn, disconnectMsg{Type: "disconnect"})
		t.writeMu.Unlock()
		outErr = t.conn.Close()
	})
	return outErr
}

func (t *NetworkTransport) readLoop() {
	defer close(t.snapshots)
	for {
		var frame map[string]interface{}
		if err := nnet.ReadFrame(t.conn, &frame); err != nil {
			return
		}
		if frame["type"] != "obs" {
			continue
		}
		obsPayload, ok := frame["obs"]
		if !ok {
			continue
		}
		b, err := json.Marshal(obsPayload)
		if err != nil {
			continue
		}
		var obs agent.Observation
		if err := json.Unmarshal(b, &obs); err != nil {
			continue
		}
		energy := 0
		if e, ok := frame["energy"].(float64); ok {
			energy = int(e)
		}
		t.snapshots <- Snapshot{Observation: obs, Energy: energy}
	}
}

var _ Transport = (*NetworkTransport)(nil)

func WrapNetworkError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("network transport: %w", err)
}
