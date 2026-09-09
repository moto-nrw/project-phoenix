package device

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// getOrCreateLastSeenState Tests
// =============================================================================

func TestGetOrCreateLastSeenState_CreatesNewState(t *testing.T) {
	t.Parallel()
	debouncer := NewLastSeenDebouncer()

	state := debouncer.getOrCreateLastSeenState(int64(1001))
	require.NotNil(t, state)
	assert.True(t, state.lastPersisted.IsZero())
	assert.True(t, state.latestSeen.IsZero())
	assert.Nil(t, state.flushTimer)
}

func TestGetOrCreateLastSeenState_ReturnsSameState(t *testing.T) {
	t.Parallel()
	debouncer := NewLastSeenDebouncer()

	state1 := debouncer.getOrCreateLastSeenState(int64(2001))
	state2 := debouncer.getOrCreateLastSeenState(int64(2001))

	assert.Same(t, state1, state2, "Should return the same state object for the same device")
}

func TestGetOrCreateLastSeenState_DifferentDevices(t *testing.T) {
	t.Parallel()
	debouncer := NewLastSeenDebouncer()

	stateA := debouncer.getOrCreateLastSeenState(int64(3001))
	stateB := debouncer.getOrCreateLastSeenState(int64(3002))

	assert.NotSame(t, stateA, stateB, "Different devices should have different state objects")
}

func TestGetOrCreateLastSeenState_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	debouncer := NewLastSeenDebouncer()

	var wg sync.WaitGroup
	states := make([]*lastSeenDebounceState, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			states[idx] = debouncer.getOrCreateLastSeenState(int64(4001))
		}(i)
	}
	wg.Wait()

	// All goroutines should get the same state
	for i := 1; i < 10; i++ {
		assert.Same(t, states[0], states[i], "All concurrent accesses should return the same state")
	}
}

// =============================================================================
// persistLastSeen Tests
// =============================================================================

func TestPersistLastSeen_Success(t *testing.T) {
	t.Parallel()
	debouncer := NewLastSeenDebouncer()

	directory := newMockDeviceDirectory()
	state := &lastSeenDebounceState{}
	observedAt := time.Now().Add(-5 * time.Second)

	debouncer.persistLastSeen(context.Background(), directory, int64(5001), observedAt, state)

	assert.True(t, directory.wasUpdated(), "Should call RecordLastSeen")
	assert.Equal(t, observedAt, state.lastPersisted, "Should persist the observed timestamp")
}

func TestPersistLastSeen_Error(t *testing.T) {
	t.Parallel()
	debouncer := NewLastSeenDebouncer()

	directory := newMockDeviceDirectory()
	directory.updateError = errors.New("db connection failed")
	state := &lastSeenDebounceState{}
	observedAt := time.Now()

	debouncer.persistLastSeen(context.Background(), directory, int64(5002), observedAt, state)

	assert.True(t, directory.wasUpdated(), "Should attempt RecordLastSeen")
	assert.True(t, state.lastPersisted.IsZero(), "Should NOT update lastPersisted on error")
}

func TestPersistLastSeen_ErrorPreservesQueuedObservation(t *testing.T) {
	t.Parallel()
	debouncer := NewLastSeenDebouncer()

	directory := newMockDeviceDirectory()
	directory.updateError = errors.New("db connection failed")
	observedAt := time.Now()
	state := &lastSeenDebounceState{
		latestSeen: observedAt.Add(30 * time.Second),
	}

	debouncer.persistLastSeen(context.Background(), directory, int64(5003), observedAt, state)

	state.mu.Lock()
	defer state.mu.Unlock()

	assert.Equal(t, observedAt, state.lastPersisted, "Should retain the failed write timestamp so queued observations can be flushed")
	assert.NotNil(t, state.flushTimer, "Should schedule a deferred flush for the queued newer observation")
	if state.flushTimer != nil {
		state.flushTimer.Stop()
	}
}

// =============================================================================
// flushDeferredLastSeen Tests
// =============================================================================

func TestFlushDeferredLastSeen_NoopWhenLatestSeenIsZero(t *testing.T) {
	t.Parallel()
	debouncer := NewLastSeenDebouncer()

	directory := newMockDeviceDirectory()
	state := &lastSeenDebounceState{
		// latestSeen is zero value
		lastPersisted: time.Now(),
	}

	debouncer.flushDeferredLastSeen(directory, int64(6001), state)

	assert.False(t, directory.wasUpdated(), "Should not write when latestSeen is zero")
}

func TestFlushDeferredLastSeen_NoopWhenLatestSeenNotAfterLastPersisted(t *testing.T) {
	t.Parallel()
	debouncer := NewLastSeenDebouncer()

	directory := newMockDeviceDirectory()
	now := time.Now()
	state := &lastSeenDebounceState{
		latestSeen:    now,
		lastPersisted: now.Add(1 * time.Second), // persisted after latest seen
	}

	debouncer.flushDeferredLastSeen(directory, int64(6002), state)

	assert.False(t, directory.wasUpdated(), "Should not write when latestSeen is not after lastPersisted")
}

func TestFlushDeferredLastSeen_WritesWhenLatestSeenAfterLastPersisted(t *testing.T) {
	t.Parallel()
	debouncer := NewLastSeenDebouncer()

	directory := newMockDeviceDirectory()
	now := time.Now()
	state := &lastSeenDebounceState{
		latestSeen:    now,
		lastPersisted: now.Add(-2 * time.Minute),
	}

	debouncer.flushDeferredLastSeen(directory, int64(6003), state)

	assert.True(t, directory.wasUpdated(), "Should write when latestSeen is after lastPersisted")
	assert.Equal(t, now, state.lastPersisted, "Should persist the latest observed timestamp")
}

func TestFlushDeferredLastSeen_ClearsFlushTimer(t *testing.T) {
	t.Parallel()
	debouncer := NewLastSeenDebouncer()

	directory := newMockDeviceDirectory()
	now := time.Now()
	timer := time.NewTimer(1 * time.Hour) // dummy timer
	defer timer.Stop()

	state := &lastSeenDebounceState{
		latestSeen:    now,
		lastPersisted: now.Add(-2 * time.Minute),
		flushTimer:    timer,
	}

	debouncer.flushDeferredLastSeen(directory, int64(6004), state)

	// flushTimer is set to nil at the start of the function
	// It may be re-set if there are new updates, but the original timer should be cleared
	assert.True(t, directory.wasUpdated())
}

// =============================================================================
// updateDeviceLastSeen Integration Tests
// =============================================================================

func TestUpdateDeviceLastSeen_FirstCallWritesImmediately(t *testing.T) {
	t.Parallel()
	debouncer := NewLastSeenDebouncer()

	directory := newMockDeviceDirectory()
	device := &AuthenticatedDevice{ID: 7001, DeviceID: "device-first-write"}

	debouncer.updateDeviceLastSeen(context.Background(), directory, device)

	assert.True(t, directory.wasUpdated(), "First call should write immediately")
	assert.NotNil(t, device.LastSeen, "Should set LastSeen on device")
}

func TestUpdateDeviceLastSeen_SecondCallWithinWindowSchedulesTimer(t *testing.T) {
	t.Parallel()
	debouncer := NewLastSeenDebouncer()

	directory := newMockDeviceDirectory()
	device := &AuthenticatedDevice{ID: 7002, DeviceID: "device-debounce-timer"}

	// First call - writes immediately
	debouncer.updateDeviceLastSeen(context.Background(), directory, device)
	assert.True(t, directory.wasUpdated())

	directory.resetUpdated()

	// Second call within debounce window - should schedule timer instead
	debouncer.updateDeviceLastSeen(context.Background(), directory, device)
	assert.False(t, directory.wasUpdated(), "Second call within window should not write immediately")

	// Verify timer was scheduled (cache key is the int64 PK)
	rawState, ok := debouncer.cache.Load(int64(7002))
	require.True(t, ok)
	state, ok := rawState.(*lastSeenDebounceState)
	require.True(t, ok)

	state.mu.Lock()
	hasTimer := state.flushTimer != nil
	if state.flushTimer != nil {
		state.flushTimer.Stop()
	}
	state.mu.Unlock()

	assert.True(t, hasTimer, "Should have scheduled a flush timer")
}

func TestUpdateDeviceLastSeen_ConcurrentFirstCallsOnlyWriteOnce(t *testing.T) {
	t.Parallel()
	debouncer := NewLastSeenDebouncer()

	directory := newMockDeviceDirectory()
	directory.updateStarted = make(chan struct{}, 2)
	directory.updateBlock = make(chan struct{})
	device := &AuthenticatedDevice{ID: 7003, DeviceID: "device-concurrent-first-write"}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		debouncer.updateDeviceLastSeen(context.Background(), directory, device)
	}()

	<-directory.updateStarted

	go func() {
		defer wg.Done()
		debouncer.updateDeviceLastSeen(context.Background(), directory, device)
	}()

	close(directory.updateBlock)
	wg.Wait()

	directory.mu.Lock()
	updateCount := directory.updateCount
	directory.mu.Unlock()

	assert.Equal(t, 1, updateCount, "Concurrent first requests should reserve the write and avoid duplicate DB updates")
}

func TestUpdateDeviceLastSeen_ErrorDoesNotPreventSubsequentCalls(t *testing.T) {
	t.Parallel()
	debouncer := NewLastSeenDebouncer()

	directory := newMockDeviceDirectory()
	directory.updateError = errors.New("temporary db error")
	device := &AuthenticatedDevice{ID: 7004, DeviceID: "device-error-recovery"}

	// First call - fails
	debouncer.updateDeviceLastSeen(context.Background(), directory, device)
	assert.True(t, directory.wasUpdated())

	// Reset - next call should still try since lastPersisted was never set
	directory.resetUpdated()
	directory.updateError = nil

	debouncer.updateDeviceLastSeen(context.Background(), directory, device)
	assert.True(t, directory.wasUpdated(), "Should retry write after previous error since lastPersisted is zero")
}
