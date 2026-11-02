package api

import (
	"sync"
	"sync/atomic"
)

// seedLoopWorkers is how many requests a loop over families sends at once.
// Each family waits mostly for the server, so four of them overlap well on
// the four-core CI runner, where the server, PostgreSQL and the seeder share
// the machine, and on the demo server, whose process seeds up to three
// schools at the same time.
//
// The regular bookings stay sequential: the server takes one lock per
// enrollment phase for every booking of an existing child, and four workers
// made that step slower in CI (11-20 s before, 19 s with them).
const seedLoopWorkers = 4

// worker is a copy of the runtime with its own client on the same adapter,
// bound to the same tenant session. A Client records its last request and
// its bound session, so parallel workers must not share one. The copy shares
// every other field: a worker only reads them.
func (r *Runtime) worker() *Runtime {
	worker := *r
	worker.Client = NewClientWithAdapter(r.Client.adapter, r.Client.verbose)
	worker.Client.BindAuth(r.TenantAuth)
	return &worker
}

// forEachParallel runs fn for every item on seedLoopWorkers workers, each
// with its own runtime copy. After the first failure no further item starts;
// of the items that failed, it returns the failure of the first in item
// order.
func forEachParallel[T any](rt *Runtime, items []T, fn func(worker *Runtime, index int, item T) error) error {
	errs := make([]error, len(items))
	slots := make(chan struct{}, seedLoopWorkers)
	var failed atomic.Bool
	var wg sync.WaitGroup
	for index, item := range items {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			if failed.Load() {
				return
			}
			if err := fn(rt.worker(), index, item); err != nil {
				errs[index] = err
				failed.Store(true)
			}
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
