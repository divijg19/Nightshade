package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/divijg19/Nightshade/internal/app/transport"
	"github.com/divijg19/Nightshade/internal/terminal"
	"github.com/divijg19/Nightshade/internal/ui"
)

const terminalRestoreSequence = "\x1b[?25h\x1b[?1049l\x1b[0m"

type RunOptions struct {
	ForceASCII   bool
	ForceNoColor bool
	ShowSplash   bool
}

type networkAdapter struct {
	transport transport.Transport
}

func (a networkAdapter) SendInput(key string) error {
	return a.transport.Send(transport.Command{Key: key})
}

func (a networkAdapter) Disconnect() error {
	return a.transport.Close()
}

func RunClient(t transport.Transport) error {
	return RunClientWithContext(context.Background(), t, RunOptions{})
}

func RunClientWithOptions(t transport.Transport, opts RunOptions) (runErr error) {
	return RunClientWithContext(context.Background(), t, opts)
}

func RunClientWithContext(ctx context.Context, t transport.Transport, opts RunOptions) (runErr error) {
	defer t.Close()
	defer func() {
		r := recover()
		restoreErr := recoverAndRestoreTerminal(os.Stdout, r)
		if runErr == nil {
			runErr = restoreErr
		}
	}()

	caps := terminal.Detect(terminal.Options{ForceASCII: opts.ForceASCII, ForceNoColor: opts.ForceNoColor})
	ui.SetPresentationOptions(ui.PresentationOptions{
		ASCIIMode: caps.ASCIIMode,
		ColorLevel: func(level terminal.ColorLevel) ui.ColorLevel {
			switch level {
			case terminal.ColorTrue:
				return ui.ColorTrue
			case terminal.Color256:
				return ui.Color256
			case terminal.ColorANSI:
				return ui.ColorANSI
			default:
				return ui.ColorNone
			}
		}(caps.ColorLevel),
	})

	app := ui.NewWithOptions(networkAdapter{transport: t}, ui.AppOptions{ShowSplash: opts.ShowSplash})
	var closeOnce sync.Once
	notifyClosed := func() {
		closeOnce.Do(func() {
			app.NotifyConnectionClosed()
		})
	}

	go func() {
		select {
		case <-ctx.Done():
			_ = t.Close()
			notifyClosed()
		default:
		}

		<-ctx.Done()
		_ = t.Close()
		notifyClosed()
	}()

	go func() {
		for snap := range t.Snapshots() {
			app.SendSnapshot(snap.Observation, snap.Energy)
		}
		notifyClosed()
	}()

	_, err := app.Run()
	return err
}

func recoverAndRestoreTerminal(out io.Writer, recovered any) error {
	if out != nil {
		if _, err := io.WriteString(out, terminalRestoreSequence); err != nil {
			return err
		}
	}
	if recovered != nil {
		return fmt.Errorf("ui panic recovered: %v", recovered)
	}
	return nil
}
