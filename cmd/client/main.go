package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	appclient "github.com/divijg19/Nightshade/internal/app/client"
	"github.com/divijg19/Nightshade/internal/app/transport"
	"github.com/divijg19/Nightshade/internal/persist"
)

func defaultSocket() string {
	if s := os.Getenv("NIGHTSHADE_SOCKET"); s != "" {
		return s
	}
	return filepath.Join(persist.BaseDir(), "socket")
}

func main() {
	socketFlag := flag.String("socket", defaultSocket(), "unix socket path (or set NIGHTSHADE_SOCKET)")
	addrFlag := flag.String("addr", "", "tcp address (e.g. :4000) for multiplayer transport")
	flag.Parse()

	_, _, pubB64, err := persist.EnsureIdentity()
	if err != nil {
		fmt.Fprintf(os.Stderr, "identity: %v\n", err)
		return
	}

	network := "unix"
	address := *socketFlag
	if *addrFlag != "" {
		network = "tcp"
		address = *addrFlag
	}
	t, err := transport.NewNetworkTransport(network, address, pubB64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", transport.WrapNetworkError(err))
		return
	}
	if err := appclient.RunClient(t); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
}
