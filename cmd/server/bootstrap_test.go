package main

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/divijg19/Nightshade/internal/agent"
	nnet "github.com/divijg19/Nightshade/internal/net"
	"github.com/divijg19/Nightshade/internal/runtime"
)

// Test that handleConn sends one bootstrap Observation immediately after
// a client completes handshake and is bound to a RemoteHuman. This ensures
// clients render once on connect without requiring input.
func TestHandleConn_SendsBootstrapObservation(t *testing.T) {
    c1, c2 := net.Pipe()
    defer c1.Close()
    defer c2.Close()

    // generate keypair and agent id
    pub, _, err := ed25519.GenerateKey(rand.Reader)
    if err != nil {
        t.Fatalf("keygen: %v", err)
    }
    pubB64 := base64.StdEncoding.EncodeToString(pub)

    // prepare agents map with a RemoteHuman that runtime will include
    agents := map[string]*agent.RemoteHuman{}
    mu := sync.Mutex{}
    rh := agent.NewRemoteHumanFromExisting(pubB64, agent.NewMemory(), agent.MaxEnergy)
    agents[pubB64] = rh

    // build runtime including this agent and an NPC so positions are valid
    rt := runtime.New([]agent.Agent{rh, agent.NewOscillating("npc-osc")})

    // run handler with runtime pointer
    go handleConn(c2, agents, nil, &mu, rt, nil)

    // send hello frame from client side
    if err := nnet.WriteFrame(c1, map[string]interface{}{"type": "hello", "public_key": pubB64}); err != nil {
        t.Fatalf("write hello: %v", err)
    }

    // read one frame with timeout
    dec := bufio.NewReader(c1)
    ch := make(chan map[string]interface{}, 1)
    go func() {
        var frame map[string]interface{}
        _ = nnet.ReadFrame(dec, &frame)
        ch <- frame
    }()

    select {
    case frame := <-ch:
        if frame == nil {
            t.Fatalf("no frame received")
        }
        if frame["type"] != "obs" {
            t.Fatalf("expected obs frame, got: %v", frame["type"])
        }
    case <-time.After(500 * time.Millisecond):
        t.Fatalf("timeout waiting for bootstrap observation")
    }
}
