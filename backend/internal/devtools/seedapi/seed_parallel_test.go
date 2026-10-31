package api

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every item runs once on its own client, at most seedLoopWorkers at a time.
func TestForEachParallelRunsEveryItemOnItsOwnClient(t *testing.T) {
	t.Parallel()
	rt := &Runtime{Client: newTestClient("http://127.0.0.1", false), TenantAuth: AuthRef{Token: "staff"}}
	items := make([]int, 20)
	for i := range items {
		items[i] = i * 10
	}
	seen := make([]int, len(items))
	var running, peak atomic.Int32
	err := forEachParallel(rt, items, func(worker *Runtime, index, item int) error {
		now := running.Add(1)
		defer running.Add(-1)
		for {
			old := peak.Load()
			if now <= old || peak.CompareAndSwap(old, now) {
				break
			}
		}
		assert.NotSame(t, rt.Client, worker.Client, "a worker never shares the runtime's client")
		assert.Equal(t, "staff", worker.Client.authRef().Token, "a worker keeps the tenant session")
		time.Sleep(5 * time.Millisecond)
		seen[index] = item
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, items, seen)
	assert.LessOrEqual(t, peak.Load(), int32(seedLoopWorkers))
	assert.Greater(t, peak.Load(), int32(1), "the items overlap")
}

// After a failure no further item starts, and the failure is returned.
func TestForEachParallelStopsAfterAFailure(t *testing.T) {
	t.Parallel()
	rt := &Runtime{Client: newTestClient("http://127.0.0.1", false)}
	items := make([]int, 50)
	var ran atomic.Int32
	err := forEachParallel(rt, items, func(_ *Runtime, _, _ int) error {
		if ran.Add(1) == 1 {
			return errors.New("first item failed")
		}
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	require.EqualError(t, err, "first item failed")
	// Only the items already running when the first one failed finish.
	assert.LessOrEqual(t, ran.Load(), int32(2*seedLoopWorkers), "the items after the failure do not start")
}
