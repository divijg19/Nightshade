package main

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/divijg19/Nightshade/internal/agent"
	nnet "github.com/divijg19/Nightshade/internal/net"
	"github.com/divijg19/Nightshade/internal/persist"
	"github.com/divijg19/Nightshade/internal/ui"
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

type netClient struct {
	conn net.Conn
	mu   chan struct{}
}

func newNetClient(conn net.Conn) *netClient {
	mu := make(chan struct{}, 1)
	mu <- struct{}{}
	return &netClient{conn: conn, mu: mu}
}

func (n *netClient) SendInput(key string) error {
	<-n.mu
	defer func() { n.mu <- struct{}{} }()
	return nnet.WriteFrame(n.conn, inputMsg{Type: "input", Key: key})
}

func (n *netClient) Disconnect() error {
	<-n.mu
	defer func() { n.mu <- struct{}{} }()
	return nnet.WriteFrame(n.conn, disconnectMsg{Type: "disconnect"})
}

func runTUI(socket string) error {
	_, _, pubB64, err := persist.EnsureIdentity()
	if err != nil {
		return fmt.Errorf("identity: %w", err)
	}

	attempts := 0
	start := time.Now()
	for {
		if time.Since(start) > 10*time.Second {
			return fmt.Errorf("timeout: failed to establish connection within 10s")
		}
		conn, err := net.Dial("unix", socket)
		if err != nil {
			attempts++
			if attempts >= 10 {
				return fmt.Errorf("failed to connect after 10 attempts")
			}
			time.Sleep(1 * time.Second)
			continue
		}
		attempts = 0

		if err := nnet.WriteFrame(conn, helloMsg{Type: "hello", PublicKey: pubB64}); err != nil {
			_ = conn.Close()
			time.Sleep(1 * time.Second)
			continue
		}

		client := newNetClient(conn)
		app := ui.New(client)

		go func() {
			firstFrame := true
			lastEnergy := 0
			for {
				var frame map[string]interface{}
				if firstFrame {
					_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
				}
				if err := nnet.ReadFrame(conn, &frame); err != nil {
					app.NotifyConnectionClosed()
					return
				}
				if firstFrame {
					_ = conn.SetReadDeadline(time.Time{})
					firstFrame = false
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
				if e, ok := frame["energy"].(float64); ok {
					lastEnergy = int(e)
				}
				app.SendSnapshot(obs, lastEnergy)
			}
		}()

		reason, err := app.Run()
		_ = conn.Close()
		if err != nil {
			return err
		}
		if reason == ui.ExitQuit {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
}
