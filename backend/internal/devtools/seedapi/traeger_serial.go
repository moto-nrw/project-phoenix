package api

import (
	"context"
	"sync"
)

// traegerSerialStep runs a step that reads and then writes the state of the
// whole Träger (contracts, monthly runs, billing letters) while no other
// school of the same Träger runs one in this process. Every demo school
// hangs under the one Demo-Träger of its database, and the demo process seeds
// up to three of them at once: in parallel, two schools both found no open
// monthly run and both started one (409 parent_billing.care_run_open), or
// took the same contract for a notice or a paper signature. Each step keeps
// its own name, so a failure still names the step that failed.
type traegerSerialStep struct{ Step }

// traegerLocks holds one lock per organization ID: a channel of capacity
// one, so a wait ends when the context does.
var traegerLocks sync.Map

func (s traegerSerialStep) Run(ctx context.Context, rt *Runtime) error {
	if rt == nil || rt.Bootstrap == nil || rt.Bootstrap.OrganizationID <= 0 {
		return s.Step.Run(ctx, rt)
	}
	lock, _ := traegerLocks.LoadOrStore(rt.Bootstrap.OrganizationID, make(chan struct{}, 1))
	held := lock.(chan struct{})
	select {
	case held <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-held }()
	return s.Step.Run(ctx, rt)
}

// traegerSerial wraps the steps of the Träger block in traegerSerialStep.
func traegerSerial(steps ...Step) []Step {
	wrapped := make([]Step, len(steps))
	for i, step := range steps {
		wrapped[i] = traegerSerialStep{Step: step}
	}
	return wrapped
}
