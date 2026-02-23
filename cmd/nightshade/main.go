package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	appclient "github.com/divijg19/Nightshade/internal/app/client"
	appserver "github.com/divijg19/Nightshade/internal/app/server"
	"github.com/divijg19/Nightshade/internal/app/single"
	"github.com/divijg19/Nightshade/internal/app/transport"
	"github.com/divijg19/Nightshade/internal/persist"
	"github.com/divijg19/Nightshade/internal/terminal"
)

const version = "v0.3.18"

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
	ascii := fs.Bool("ascii", false, "force ASCII glyph fallback")
	noColor := fs.Bool("no-color", false, "disable terminal color output")
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
	return appclient.RunClientWithOptions(t, appclient.RunOptions{ForceASCII: *ascii, ForceNoColor: *noColor})
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
	ascii := fs.Bool("ascii", false, "force ASCII glyph fallback")
	noColor := fs.Bool("no-color", false, "disable terminal color output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return single.Run(single.Options{Dev: *dev, ClientOptions: appclient.RunOptions{ForceASCII: *ascii, ForceNoColor: *noColor, ShowSplash: true}})
}

func runDiagnose(forceASCII bool, forceNoColor bool) error {
	caps := terminal.Detect(terminal.Options{ForceASCII: forceASCII, ForceNoColor: forceNoColor})
	for _, line := range terminal.DiagnosticLines(caps) {
		fmt.Println(line)
	}
	return nil
}

func printErrorAndExit(prefix string, err error) {
	if err == nil {
		return
	}
	if prefix != "" {
		fmt.Fprintf(os.Stderr, "%s: %v\n", prefix, err)
	} else {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
	os.Exit(1)
}

func main() {
	args := os.Args[1:]
	for _, a := range args {
		if a == "--version" {
			fmt.Println(version)
			return
		}
	}
	for _, a := range args {
		if a == "--diagnose" {
			forceASCII := hasFlag(args, "--ascii")
			forceNoColor := hasFlag(args, "--no-color")
			printErrorAndExit("nightshade diagnose", runDiagnose(forceASCII, forceNoColor))
			return
		}
	}

	if len(args) > 0 {
		switch args[0] {
		case "server":
			printErrorAndExit("nightshade server", runServerMode(args[1:]))
			return
		case "client":
			printErrorAndExit("nightshade client", runClientMode(args[1:]))
			return
		}
	}
	if strings.TrimSpace(strings.Join(args, "")) == "" {
		printErrorAndExit("nightshade", runSingleProcess(args))
		return
	}
	printErrorAndExit("nightshade", runSingleProcess(args))
}

func hasFlag(args []string, flagName string) bool {
	for _, arg := range args {
		if arg == flagName {
			return true
		}
	}
	return false
}
