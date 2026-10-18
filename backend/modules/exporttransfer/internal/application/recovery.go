package application

import "context"

type RecoveryStore interface {
	SettleExpired(context.Context, int) (int, error)
}

// Recovery terminalizes expired intents without ever invoking a transport.
func Recovery(store RecoveryStore, withinAdmin func(context.Context, func(context.Context) error) error) func(context.Context, int) (int, error) {
	return func(ctx context.Context, limit int) (int, error) {
		var count int
		err := withinAdmin(ctx, func(txCtx context.Context) error {
			var err error
			count, err = store.SettleExpired(txCtx, limit)
			return err
		})
		return count, err
	}
}
