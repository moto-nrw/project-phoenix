package ports

import (
	"context"
	"time"
)

// CommandObservation describes one owner call, including attempts rolled back
// by a retry. Labels name capabilities, never uploaded data or identifiers.
type CommandObservation struct {
	Owner     string
	Operation string
	Duration  time.Duration
	Failed    bool
}

type commandObserverKey struct{}

func WithCommandObserver(ctx context.Context, observer func(CommandObservation)) context.Context {
	return context.WithValue(ctx, commandObserverKey{}, observer)
}

func observeCommand(ctx context.Context, owner, operation string, started time.Time, err *error) {
	if observer, ok := ctx.Value(commandObserverKey{}).(func(CommandObservation)); ok {
		observer(CommandObservation{Owner: owner, Operation: operation, Duration: time.Since(started), Failed: *err != nil})
	}
}

// ObserveCommand covers retained owner capabilities whose types have not yet
// moved into these ports (invitation and consent-history commands).
func ObserveCommand(ctx context.Context, owner, operation string, command func() error) (err error) {
	defer observeCommand(ctx, owner, operation, time.Now(), &err)
	return command()
}
