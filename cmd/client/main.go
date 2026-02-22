package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

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
	flag.Parse()

	if err := runTUI(*socketFlag); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
}
