package device

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

type heartbeatWrite struct {
	ctx    context.Context
	id     int64
	seenAt time.Time
	at     time.Time
}

// Record through the real port, including attempts that fail. A bounded fake
// outage lets the unfixed zero-delay retry chain fail without spinning forever.
type retryHeartbeatDirectory struct {
	*mockDeviceDirectory
	record   func(heartbeatWrite) error
	snapshot func() []heartbeatWrite
}

// Construct an immutable port double. The recorder owns synchronized history;
// runtime calls cannot install dependencies or expose a mutable setter surface.
func newHeartbeatRetryDirectory(record func(heartbeatWrite, int) error) *retryHeartbeatDirectory {
	var mu sync.Mutex
	var writes []heartbeatWrite
	return &retryHeartbeatDirectory{
		record: func(write heartbeatWrite) error {
			mu.Lock()
			defer mu.Unlock()
			writes = append(writes, write)
			if record != nil {
				return record(write, len(writes))
			}
			return nil
		},
		snapshot: func() []heartbeatWrite {
			mu.Lock()
			defer mu.Unlock()
			return append([]heartbeatWrite(nil), writes...)
		},
	}
}

func (d *retryHeartbeatDirectory) RecordLastSeen(ctx context.Context, id int64, seenAt time.Time) error {
	write := heartbeatWrite{ctx: ctx, id: id, seenAt: seenAt, at: time.Now()}
	return d.record(write)
}

func stopHeartbeatTimers(d *LastSeenDebouncer) {
	d.cache.Range(func(_, value any) bool {
		state := value.(*lastSeenDebounceState)
		state.mu.Lock()
		defer state.mu.Unlock()
		if state.flushTimer != nil {
			state.flushTimer.Stop()
		}
		return true
	})
}

func TestHeartbeatRetryWaitsAfterDeferredFailure(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		debouncer := NewLastSeenDebouncer()
		defer stopHeartbeatTimers(debouncer)
		directory := newHeartbeatRetryDirectory(func(_ heartbeatWrite, call int) error {
			if call == 2 {
				return errors.New("database is closed")
			}
			return nil
		})
		type schoolRuntimeKey struct{}
		runtime := &struct{ school int64 }{school: 7101}
		ctx, cancel := context.WithCancel(context.WithValue(context.Background(), schoolRuntimeKey{}, runtime))
		defer cancel()
		device := &AuthenticatedDevice{ID: 91001, TenantID: runtime.school}
		start := time.Now()
		debouncer.updateDeviceLastSeen(ctx, directory, device)
		time.Sleep(time.Second)
		debouncer.updateDeviceLastSeen(ctx, directory, device)
		cancel()

		time.Sleep(lastSeenDebounceWindow - time.Second)
		synctest.Wait()
		require.Len(t, directory.snapshot(), 2, "a failed deferred heartbeat must not immediately retry")
		time.Sleep(time.Second)
		debouncer.updateDeviceLastSeen(ctx, directory, device)
		newest := time.Now()
		time.Sleep(lastSeenDebounceWindow - time.Second - time.Nanosecond)
		synctest.Wait()
		require.Len(t, directory.snapshot(), 2, "new requests must not bypass the pending retry window")
		time.Sleep(time.Nanosecond)
		synctest.Wait()

		writes := directory.snapshot()
		require.Len(t, writes, 3)
		require.Equal(t, start, writes[0].at)
		require.Equal(t, start.Add(lastSeenDebounceWindow), writes[1].at)
		require.Equal(t, start.Add(time.Second), writes[1].seenAt)
		require.Equal(t, start.Add(2*lastSeenDebounceWindow), writes[2].at)
		require.Equal(t, newest, writes[2].seenAt, "retry must persist the newest heartbeat")
		for _, write := range writes[1:] {
			require.Equal(t, device.ID, write.id)
			require.NoError(t, write.ctx.Err(), "deferred writes must survive request cancellation")
			require.Nil(t, write.ctx.Done())
			require.Same(t, runtime, write.ctx.Value(schoolRuntimeKey{}))
		}
		time.Sleep(2 * lastSeenDebounceWindow)
		synctest.Wait()
		require.Len(t, directory.snapshot(), 3, "successful retry must drain the pending heartbeat")
	})
}

func TestHeartbeatRetryRepeatedFailuresKeepLatestObservation(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		debouncer := NewLastSeenDebouncer()
		defer stopHeartbeatTimers(debouncer)
		start := time.Now()
		directory := newHeartbeatRetryDirectory(func(write heartbeatWrite, call int) error {
			// End an unexpected immediate chain on the original implementation.
			if call > 2 && write.at.Before(start.Add(time.Duration(call-1)*lastSeenDebounceWindow)) {
				return nil
			}
			if call >= 2 && call <= 4 {
				return errors.New("database remains unavailable")
			}
			return nil
		})
		device := &AuthenticatedDevice{ID: 91002, TenantID: 7102}
		debouncer.updateDeviceLastSeen(context.Background(), directory, device)
		time.Sleep(time.Second)
		debouncer.updateDeviceLastSeen(context.Background(), directory, device)
		time.Sleep(lastSeenDebounceWindow - time.Second)
		synctest.Wait()
		require.Len(t, directory.snapshot(), 2)

		var newest time.Time
		for expectedCalls := 3; expectedCalls <= 5; expectedCalls++ {
			time.Sleep(lastSeenDebounceWindow / 2)
			debouncer.updateDeviceLastSeen(context.Background(), directory, device)
			newest = time.Now()
			time.Sleep(lastSeenDebounceWindow/2 - time.Nanosecond)
			synctest.Wait()
			require.Len(t, directory.snapshot(), expectedCalls-1, "outage retries must stay bounded despite fresh heartbeats")
			time.Sleep(time.Nanosecond)
			synctest.Wait()
			writes := directory.snapshot()
			require.Len(t, writes, expectedCalls)
			require.Equal(t, start.Add(time.Duration(expectedCalls-1)*lastSeenDebounceWindow), writes[expectedCalls-1].at)
			require.Equal(t, newest, writes[expectedCalls-1].seenAt)
		}
		time.Sleep(2 * lastSeenDebounceWindow)
		synctest.Wait()
		require.Len(t, directory.snapshot(), 5, "recovery must stop retries without losing the latest heartbeat")
		debouncer.updateDeviceLastSeen(context.Background(), directory, device)
		writes := directory.snapshot()
		require.Len(t, writes, 6, "an overdue fresh heartbeat must still write immediately after recovery")
		require.Equal(t, time.Now(), writes[5].at)
		require.Equal(t, time.Now(), writes[5].seenAt)
	})
}

func TestHeartbeatRetryIsolatesDevicePrimaryKeysAndSchoolContexts(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		debouncer := NewLastSeenDebouncer()
		defer stopHeartbeatTimers(debouncer)
		start := time.Now()
		type schoolRuntimeKey struct{}
		first := &AuthenticatedDevice{ID: 91003, TenantID: 7103, DeviceID: "shared-kiosk-name"}
		second := &AuthenticatedDevice{ID: 91004, TenantID: 7104, DeviceID: first.DeviceID}
		attempts := make(map[int64]int)
		directory := newHeartbeatRetryDirectory(func(write heartbeatWrite, _ int) error {
			attempts[write.id]++
			if write.id == first.ID && attempts[write.id] == 2 {
				return errors.New("first school's database unavailable")
			}
			return nil
		})
		for _, device := range []*AuthenticatedDevice{first, second} {
			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), schoolRuntimeKey{}, device.TenantID))
			debouncer.updateDeviceLastSeen(ctx, directory, device)
			cancel()
		}
		time.Sleep(time.Second)
		for _, device := range []*AuthenticatedDevice{first, second} {
			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), schoolRuntimeKey{}, device.TenantID))
			debouncer.updateDeviceLastSeen(ctx, directory, device)
			cancel()
		}
		time.Sleep(lastSeenDebounceWindow - time.Second)
		synctest.Wait()
		require.Len(t, directory.snapshot(), 4, "one school's failed flush must not retry immediately or delay the other school")
		time.Sleep(lastSeenDebounceWindow)
		synctest.Wait()
		writes := directory.snapshot()
		require.Len(t, writes, 5)
		for _, device := range []*AuthenticatedDevice{first, second} {
			var deviceWrites []heartbeatWrite
			for _, write := range writes {
				if write.id == device.ID {
					deviceWrites = append(deviceWrites, write)
				}
			}
			expected := 2
			if device.ID == first.ID {
				expected = 3
			}
			require.Len(t, deviceWrites, expected, "device_id strings must not share debounce or retry state")
			for i, write := range deviceWrites {
				require.Equal(t, device.TenantID, write.ctx.Value(schoolRuntimeKey{}))
				require.Equal(t, start.Add(time.Duration(i)*lastSeenDebounceWindow), write.at)
				if i > 0 {
					require.Equal(t, start.Add(time.Second), write.seenAt)
					require.NoError(t, write.ctx.Err())
					require.Nil(t, write.ctx.Done())
				}
			}
		}
		time.Sleep(2 * lastSeenDebounceWindow)
		synctest.Wait()
		require.Len(t, directory.snapshot(), 5)
	})
}

func TestHeartbeatRetryWaitsFromFailureCompletionAndSerializesWrites(t *testing.T) {
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
			if call == 2 {
				close(started)
				<-release
				return errors.New("delayed database failure")
			}
			return nil
		})
		device := &AuthenticatedDevice{ID: 91005, TenantID: 7105}
		start := time.Now()
		debouncer.updateDeviceLastSeen(context.Background(), directory, device)
		time.Sleep(time.Second)
		debouncer.updateDeviceLastSeen(context.Background(), directory, device)
		<-started
		time.Sleep(lastSeenDebounceWindow / 4)
		// A different request principal for the same PK avoids sharing a mutable
		// LastSeen pointer while the directory write is still in flight.
		newRequestDevice := *device
		debouncer.updateDeviceLastSeen(context.Background(), directory, &newRequestDevice)
		newest := time.Now()
		close(release)
		synctest.Wait()
		require.Len(t, directory.snapshot(), 2, "new observations during a failed write must not start a second writer")
		time.Sleep(lastSeenDebounceWindow - time.Nanosecond)
		synctest.Wait()
		require.Len(t, directory.snapshot(), 2, "the retry window starts when the failed write completes")
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		writes := directory.snapshot()
		require.Len(t, writes, 3)
		require.Equal(t, start.Add(2*lastSeenDebounceWindow+lastSeenDebounceWindow/4), writes[2].at)
		require.Equal(t, newest, writes[2].seenAt)
		time.Sleep(2 * lastSeenDebounceWindow)
		synctest.Wait()
		require.Len(t, directory.snapshot(), 3)
	})
}
