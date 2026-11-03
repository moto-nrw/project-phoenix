package device

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHeartbeatOverdueSuccessfulWriteFlushesImmediately(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		debouncer := NewLastSeenDebouncer()
		defer stopHeartbeatTimers(debouncer)
		started := make(chan struct{})
		release := make(chan struct{})
		defer func() {
			select {
			case <-release:
			default:
				close(release)
			}
		}()
		directory := newHeartbeatRetryDirectory(func(_ heartbeatWrite, call int) error {
			if call == 1 {
				close(started)
				<-release
			}
			return nil
		})
		device := &AuthenticatedDevice{ID: 91006, TenantID: 7106}
		start := time.Now()
		go debouncer.updateDeviceLastSeen(context.Background(), directory, device)
		<-started
		time.Sleep(time.Second)
		newRequestDevice := *device
		debouncer.updateDeviceLastSeen(context.Background(), directory, &newRequestDevice)
		newest := time.Now()
		time.Sleep(lastSeenDebounceWindow)
		close(release)
		synctest.Wait()

		writes := directory.snapshot()
		require.Len(t, writes, 2, "an overdue flush without a failure must not acquire a retry delay")
		require.Equal(t, start, writes[0].at)
		require.Equal(t, start.Add(time.Second+lastSeenDebounceWindow), writes[1].at)
		require.Equal(t, newest, writes[1].seenAt)
		time.Sleep(2 * lastSeenDebounceWindow)
		synctest.Wait()
		require.Len(t, directory.snapshot(), 2)
	})
}
