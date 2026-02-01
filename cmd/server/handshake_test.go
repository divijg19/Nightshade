package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/divijg19/Nightshade/internal/agent"
	nnet "github.com/divijg19/Nightshade/internal/net"
)

// helper to send a frame on conn
func sendFrame(t *testing.T, c net.Conn, v interface{}) {
	t.Helper()
	if err := nnet.WriteFrame(c, v); err != nil {
		t.Fatalf("write frame: %v", err)
	}
}

func TestHandleConn_RejectsInvalidPublicKey(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	agents := map[string]*agent.RemoteHuman{}
	var mu sync.Mutex

	// run handler (no runtime available)
	go handleConn(c2, agents, nil, &mu, nil, nil)

	// send empty public key
	sendFrame(t, c1, map[string]interface{}{"type": "hello", "public_key": ""})
	// allow some time
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	if len(agents) != 0 {
		t.Fatalf("agents should be empty on invalid key")
	}
	mu.Unlock()
}

func TestHandleConn_AcceptsValidPublicKey(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	var mu sync.Mutex
	agents := map[string]*agent.RemoteHuman{}

	go handleConn(c2, agents, nil, &mu, nil, nil)

	// generate ed25519 keypair and send base64 public key
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	sendFrame(t, c1, map[string]interface{}{"type": "hello", "public_key": pubB64})

	// allow handler to process
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	// verify key is present
	if _, ok := agents[pubB64]; !ok {
		t.Fatalf("agent id not registered")
	}
	mu.Unlock()
}
