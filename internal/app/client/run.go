package client

import (
	"github.com/divijg19/Nightshade/internal/app/transport"
	"github.com/divijg19/Nightshade/internal/ui"
)

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
	defer t.Close()

	app := ui.New(networkAdapter{transport: t})
	go func() {
		for snap := range t.Snapshots() {
			app.SendSnapshot(snap.Observation, snap.Energy)
		}
		app.NotifyConnectionClosed()
	}()

	_, err := app.Run()
	return err
}
