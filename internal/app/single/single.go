package single

import (
	"context"
	"time"

	"github.com/divijg19/Nightshade/internal/agent"
	appclient "github.com/divijg19/Nightshade/internal/app/client"
	"github.com/divijg19/Nightshade/internal/app/common"
	"github.com/divijg19/Nightshade/internal/app/transport"
	"github.com/divijg19/Nightshade/internal/persist"
	"github.com/divijg19/Nightshade/internal/runtime"
)

type Options struct {
	Dev           bool
	ClientOptions appclient.RunOptions
}

func Run(opts Options) error {
	return RunContext(context.Background(), opts)
}

func RunContext(parent context.Context, opts Options) error {
	_, _, pubB64, err := persist.EnsureIdentity()
	if err != nil {
		return err
	}

	rh := agent.NewRemoteHumanFromExisting(pubB64, agent.NewMemory(), agent.MaxEnergy)
	rh.SetConnected(true)
	npc := agent.NewOscillating("npc-osc")
	rt := runtime.New([]agent.Agent{rh, npc})

	inMem := transport.NewInMemoryTransport(64)
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	go func() {
		<-ctx.Done()
		_ = inMem.Close()
	}()

	tickDone := make(chan struct{})
	go func() {
		defer close(tickDone)
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
			_ = rt.TickOnce()
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
	}()

	shutdownInputDone := make(chan struct{})
	go func() {
		defer close(shutdownInputDone)
		<-ctx.Done()
		for {
			select {
			case rh.RecvInput <- "CTRL_C":
				return
			case <-tickDone:
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}()

	bridgeDone := make(chan struct{}, 2)
	go func() {
		defer func() { bridgeDone <- struct{}{} }()
		for {
			select {
			case <-ctx.Done():
				return
			case obs := <-rh.SendObservation:
				if err := inMem.Publish(transport.Snapshot{Observation: obs, Energy: rh.Energy()}); err != nil {
					return
				}
			}
		}
	}()

	go func() {
		defer func() { bridgeDone <- struct{}{} }()
		for {
			select {
			case <-ctx.Done():
				return
			case cmd, ok := <-inMem.Commands():
				if !ok {
					return
				}
				select {
				case rh.RecvInput <- cmd.Key:
				default:
				}
			}
		}
	}()

	bootstrap := common.BootstrapObservation(rt, rh.ID())
	_ = inMem.Publish(transport.Snapshot{Observation: bootstrap, Energy: rh.Energy()})

	clientErr := appclient.RunClientWithContext(ctx, inMem, opts.ClientOptions)
	cancel()
	rh.SetConnected(false)
	<-bridgeDone
	<-bridgeDone
	<-tickDone
	<-shutdownInputDone
	return clientErr
}
