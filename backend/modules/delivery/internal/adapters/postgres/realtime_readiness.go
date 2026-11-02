package postgres

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// pgdriver.Listen only writes commands; it does not consume their server
// acknowledgements. Receiving our probe proves both ordered LISTEN commands
// took effect before the constructor exposes the API hub. A probe can overtake
// LISTEN on its separate connection, so repeat only this startup handshake.
func awaitRealtimeReady(ctx context.Context, db *bun.DB, listener *pgdriver.Listener) error {
	probe := fmt.Sprintf("%016x%016x", rand.Uint64(), rand.Uint64())
	for {
		if err := notifyRealtime(ctx, db, realtimeReadyChannel, probe); err != nil {
			return err
		}
		ready, err := receiveRealtimeReady(ctx, listener, probe)
		if err != nil {
			return err
		}
		if ready {
			return listener.Unlisten(ctx, realtimeReadyChannel)
		}
	}
}

func receiveRealtimeReady(ctx context.Context, listener *pgdriver.Listener, probe string) (bool, error) {
	for {
		channel, payload, err := listener.ReceiveTimeout(ctx, 50*time.Millisecond)
		if err != nil {
			var timeout net.Error
			if errors.As(err, &timeout) && timeout.Timeout() && ctx.Err() == nil {
				return false, nil
			}
			return false, err
		}
		if channel == realtimeReadyChannel && payload == probe {
			return true, nil
		}
	}
}
