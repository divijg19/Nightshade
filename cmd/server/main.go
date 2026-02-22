package main

import (
	"context"
	"log"
	"net"
	"sync"

	"github.com/divijg19/Nightshade/internal/agent"
	appserver "github.com/divijg19/Nightshade/internal/app/server"
	"github.com/divijg19/Nightshade/internal/runtime"
)

func handleConn(conn net.Conn, agents map[string]*agent.RemoteHuman, cancels map[string]context.CancelFunc, mu *sync.Mutex, rt *runtime.Runtime, startRuntime func()) {
	appserver.HandleConn(conn, agents, cancels, mu, rt, startRuntime)
}

func main() {
	opts := appserver.ParseOptionsFromFlags()
	if err := appserver.Run(opts); err != nil {
		log.Fatalf("server: %v", err)
	}
}
