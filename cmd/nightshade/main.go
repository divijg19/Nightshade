package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	appclient "github.com/divijg19/Nightshade/internal/app/client"
	appserver "github.com/divijg19/Nightshade/internal/app/server"
	"github.com/divijg19/Nightshade/internal/app/single"
	"github.com/divijg19/Nightshade/internal/app/transport"
	"github.com/divijg19/Nightshade/internal/persist"
)

func defaultSocket() string {
	if s := os.Getenv("NIGHTSHADE_SOCKET"); s != "" {
		return s
	}
	return filepath.Join(persist.BaseDir(), "socket")
}

func runClientMode(args []string) error {
	fs := flag.NewFlagSet("client", flag.ContinueOnError)
	socket := fs.String("socket", defaultSocket(), "unix socket path (or set NIGHTSHADE_SOCKET)")
	addr := fs.String("addr", "", "tcp address (e.g. :4000) for multiplayer transport")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, _, pubB64, err := persist.EnsureIdentity()
	if err != nil {
		return fmt.Errorf("identity: %w", err)
	}
	network := "unix"
	address := *socket
	if *addr != "" {
		network = "tcp"
		address = *addr
	}
	t, err := transport.NewNetworkTransport(network, address, pubB64)
	if err != nil {
		return transport.WrapNetworkError(err)
	}
	return appclient.RunClient(t)
}

func runServerMode(args []string) error {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	socket := fs.String("socket", appserver.DefaultSocket(), "unix socket path (or set NIGHTSHADE_SOCKET)")
	addr := fs.String("addr", "", "tcp address (e.g. :4000) for multiplayer transport")
	dev := fs.Bool("dev", false, "enable dev mode: faster ticks, verbose logs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return appserver.RunContext(ctx, appserver.Options{Socket: *socket, Addr: *addr, Dev: *dev})
}

func runSingleProcess(args []string) error {
	fs := flag.NewFlagSet("nightshade", flag.ContinueOnError)
	dev := fs.Bool("dev", false, "enable dev mode: faster ticks")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return single.Run(single.Options{Dev: *dev})
}

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "server":
			if err := runServerMode(args[1:]); err != nil {
				log.Fatalf("nightshade server: %v", err)
			}
			return
		case "client":
			if err := runClientMode(args[1:]); err != nil {
				log.Fatalf("nightshade client: %v", err)
			}
			return
		}
	}
	if err := runSingleProcess(args); err != nil {
		log.Fatalf("nightshade: %v", err)
	}
}
