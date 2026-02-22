package server

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/app/common"
	"github.com/divijg19/Nightshade/internal/core"
	nnet "github.com/divijg19/Nightshade/internal/net"
	"github.com/divijg19/Nightshade/internal/persist"
	"github.com/divijg19/Nightshade/internal/runtime"
)

type helloMsg struct {
	Type      string `json:"type"`
	PublicKey string `json:"public_key"`
}

type inputMsg struct {
	Type string `json:"type"`
	Key  string `json:"key"`
}

type Options struct {
	Socket string
	Addr   string
	Dev    bool
}

func DefaultSocket() string {
	if s := os.Getenv("NIGHTSHADE_SOCKET"); s != "" {
		return s
	}
	return filepath.Join(persist.BaseDir(), "socket")
}

func ParseOptionsFromFlags() Options {
	dev := flag.Bool("dev", false, "enable dev mode: faster ticks, verbose logs")
	socket := flag.String("socket", DefaultSocket(), "unix socket path (or set NIGHTSHADE_SOCKET)")
	addr := flag.String("addr", "", "tcp address (e.g. :4000) for multiplayer transport")
	flag.Parse()
	return Options{Socket: *socket, Addr: *addr, Dev: *dev}
}

func Run(opts Options) error {
	return RunContext(context.Background(), opts)
}

func RunContext(ctx context.Context, opts Options) error {
	network := "unix"
	address := opts.Socket
	if opts.Addr != "" {
		network = "tcp"
		address = opts.Addr
	} else {
		_ = os.Remove(opts.Socket)
	}

	l, err := net.Listen(network, address)
	if err != nil {
		return err
	}
	defer l.Close()
	log.Printf("server listening on %s://%s", network, address)

	go func() {
		<-ctx.Done()
		_ = l.Close()
	}()

	agents := map[string]*agent.RemoteHuman{}
	cancels := map[string]context.CancelFunc{}
	var mu sync.Mutex
	started := false
	var rt *runtime.Runtime

	startRuntimeIfNeeded := func() {
		mu.Lock()
		defer mu.Unlock()
		if started || len(agents) == 0 {
			return
		}
		started = true
		list := make([]agent.Agent, 0, len(agents)+1)
		for _, a := range agents {
			list = append(list, a)
		}
		list = append(list, agent.NewOscillating("npc-osc"))
		rt = runtime.New(list)
		go func() {
			delay := 200 * time.Millisecond
			if opts.Dev {
				delay = 50 * time.Millisecond
			}
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if opts.Dev {
					log.Printf("tick_start")
				}
				_ = rt.TickOnce()
				if opts.Dev {
					log.Printf("tick_end")
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(delay):
				}
			}
		}()
	}

	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
					return
				}
				log.Printf("accept: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			go HandleConn(c, agents, cancels, &mu, rt, startRuntimeIfNeeded)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		mu.Lock()
		for id, a := range agents {
			agentDir := filepath.Join(persist.BaseDir(), "agents", id)
			st := map[string]interface{}{"energy": a.Energy()}
			_ = persist.WriteJSON(filepath.Join(agentDir, "state.json"), st)

			tiles := []map[string]int{}
			if a.Memory() != nil {
				for _, mt := range a.Memory().All() {
					tiles = append(tiles, map[string]int{
						"x":         mt.Tile.Position.X,
						"y":         mt.Tile.Position.Y,
						"glyph":     int(mt.Tile.Glyph),
						"lastSeen":  mt.LastSeen,
						"scarLevel": mt.ScarLevel,
					})
				}
			}
			memOut := map[string]interface{}{"tiles": tiles}
			_ = persist.WriteJSON(filepath.Join(agentDir, "memory.json"), memOut)
		}
		mu.Unlock()
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(1 * time.Second):
		}
	}
}

func HandleConn(conn net.Conn, agents map[string]*agent.RemoteHuman, cancels map[string]context.CancelFunc, mu *sync.Mutex, rt *runtime.Runtime, startRuntime func()) {
	defer conn.Close()
	var h helloMsg
	if err := nnet.ReadFrame(conn, &h); err != nil {
		log.Printf("invalid_frame agent=? err=%v", err)
		return
	}
	pubB64 := h.PublicKey
	if pubB64 == "" {
		log.Printf("empty public key from client")
		return
	}
	pubBytes, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil {
		log.Printf("invalid base64 public key: %v", err)
		return
	}
	if len(pubBytes) != 32 {
		log.Printf("invalid public key length: %d", len(pubBytes))
		return
	}

	agentID := pubB64
	mu.Lock()
	rh, ok := agents[agentID]
	mu.Unlock()
	if !ok {
		agentDir := filepath.Join(persist.BaseDir(), "agents", agentID)
		energy := agent.MaxEnergy
		mem := agent.NewMemory()

		statePath := filepath.Join(agentDir, "state.json")
		var st struct {
			Energy int `json:"energy"`
		}
		if err := persist.ReadJSON(statePath, &st); err == nil {
			energy = st.Energy
		}

		memoryPath := filepath.Join(agentDir, "memory.json")
		var raw struct {
			Tiles []struct {
				X         int `json:"x"`
				Y         int `json:"y"`
				Glyph     int `json:"glyph"`
				LastSeen  int `json:"lastSeen"`
				ScarLevel int `json:"scarLevel"`
			} `json:"tiles"`
		}
		if err := persist.ReadJSON(memoryPath, &raw); err == nil {
			for _, t := range raw.Tiles {
				pos := core.Position{X: t.X, Y: t.Y}
				mt := agent.MemoryTile{Tile: core.TileView{Position: pos, Glyph: rune(t.Glyph), Visible: true}, LastSeen: t.LastSeen, ScarLevel: t.ScarLevel}
				mem.SetMemoryTile(pos, mt)
			}
		}

		rh = agent.NewRemoteHumanFromExisting(agentID, mem, energy)
		mu.Lock()
		agents[agentID] = rh
		mu.Unlock()
	}
	if startRuntime != nil {
		startRuntime()
	}
	mu.Lock()
	if cancels == nil {
		cancels = map[string]context.CancelFunc{}
	}
	if cf, exists := cancels[agentID]; exists {
		log.Printf("client_reconnected agent=%s", agentID)
		cf()
	} else {
		log.Printf("client_connected agent=%s", agentID)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancels[agentID] = cancel
	mu.Unlock()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case obs, ok := <-rh.SendObservation:
				if !ok {
					return
				}
				out := map[string]interface{}{"type": "obs", "obs": obs, "energy": rh.Energy()}
				if err := nnet.WriteFrame(conn, out); err != nil {
					log.Printf("client_disconnected agent=%s reason=error", agentID)
					cancel()
					return
				}
			}
		}
	}()

	rh.SetConnected(true)
	obs := common.BootstrapObservation(rt, agentID)
	select {
	case rh.SendObservation <- obs:
	default:
	}

	dec := bufio.NewReader(conn)
	for {
		var im inputMsg
		if err := nnet.ReadFrame(dec, &im); err != nil {
			reason := "eof"
			if !os.IsTimeout(err) {
				reason = "invalid"
			}
			log.Printf("client_disconnected agent=%s reason=%s", agentID, reason)
			cancel()
			return
		}
		switch im.Type {
		case "input":
			select {
			case rh.RecvInput <- im.Key:
			default:
			}
		case "disconnect":
			log.Printf("client_disconnected agent=%s reason=client_quit", agentID)
			cancel()
			return
		default:
			log.Printf("invalid_frame agent=%s err=unknown_type", agentID)
		}
	}
}
