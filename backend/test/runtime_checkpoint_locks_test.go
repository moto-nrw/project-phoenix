package test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCheckpointLockSamplingSeparatesAccountingFromEvidence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		enabled bool
	}{
		{name: "accounting"},
		{name: "evidence", enabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				calls := 0
				stop := sampleCheckpointLocks(tc.enabled, func(_ context.Context) (int, error) {
					calls++
					return 0, context.DeadlineExceeded
				})
				// Advance the virtual clock beyond one sampling tick, without wall-clock delay.
				time.Sleep(3 * time.Millisecond)
				synctest.Wait()
				result := stop()
				if tc.enabled {
					require.Equal(t, 1, calls)
					require.Equal(t, context.DeadlineExceeded.Error(), result.Error)
				} else {
					require.Zero(t, calls, "query accounting must not run measurement-only probes")
					require.Equal(t, RuntimeCheckpointLockSamples{}, result)
				}
				require.Equal(t, result, stop(), "stopping a sampler is idempotent")
			})
		})
	}
}
