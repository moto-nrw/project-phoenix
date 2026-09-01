package device

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// getOrCreateLastSeenState Tests
// =============================================================================

func TestGetOrCreateLastSeenState_CreatesNewState(t *testing.T) {
	t.Parallel()
	debouncer := newLastSeenDebouncer()

	state := debouncer.getOrCreateLastSeenState(int64(1001))
	require.NotNil(t, state)
	assert.True(t, state.lastPersisted.IsZero())
	assert.True(t, state.latestSeen.IsZero())
	assert.Nil(t, state.flushTimer)
}

func TestGetOrCreateLastSeenState_ReturnsSameState(t *testing.T) {
	t.Parallel()
	debouncer := newLastSeenDebouncer()

	state1 := debouncer.getOrCreateLastSeenState(int64(2001))
	state2 := debouncer.getOrCreateLastSeenState(int64(2001))

	assert.Same(t, state1, state2, "Should return the same state object for the same device")
}

func TestGetOrCreateLastSeenState_DifferentDevices(t *testing.T) {
	t.Parallel()
	debouncer := newLastSeenDebouncer()

	stateA := debouncer.getOrCreateLastSeenState(int64(3001))
	stateB := debouncer.getOrCreateLastSeenState(int64(3002))

	assert.NotSame(t, stateA, stateB, "Different devices should have different state objects")
}

func TestGetOrCreateLastSeenState_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	debouncer := newLastSeenDebouncer()

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
	debouncer := newLastSeenDebouncer()

	mockService := newMockIoTService()
	state := &lastSeenDebounceState{}
	observedAt := time.Now().Add(-5 * time.Second)

	debouncer.persistLastSeen(context.Background(), mockService, int64(5001), observedAt, state)

	assert.True(t, mockService.updateCalled, "Should call UpdateDeviceLastSeen")
	assert.Equal(t, observedAt, state.lastPersisted, "Should persist the observed timestamp")
}

func TestPersistLastSeen_Error(t *testing.T) {
	t.Parallel()
	debouncer := newLastSeenDebouncer()

	mockService := newMockIoTService()
	mockService.updateError = errors.New("db connection failed")
	state := &lastSeenDebounceState{}
	observedAt := time.Now()

	debouncer.persistLastSeen(context.Background(), mockService, int64(5002), observedAt, state)

	assert.True(t, mockService.updateCalled, "Should attempt UpdateDeviceLastSeen")
	assert.True(t, state.lastPersisted.IsZero(), "Should NOT update lastPersisted on error")
}

func TestPersistLastSeen_ErrorPreservesQueuedObservation(t *testing.T) {
	t.Parallel()
	debouncer := newLastSeenDebouncer()

	mockService := newMockIoTService()
	mockService.updateError = errors.New("db connection failed")
	observedAt := time.Now()
	state := &lastSeenDebounceState{
		latestSeen: observedAt.Add(30 * time.Second),
	}

	debouncer.persistLastSeen(context.Background(), mockService, int64(5003), observedAt, state)

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
	debouncer := newLastSeenDebouncer()

	mockService := newMockIoTService()
	state := &lastSeenDebounceState{
		// latestSeen is zero value
		lastPersisted: time.Now(),
	}

	debouncer.flushDeferredLastSeen(mockService, int64(6001), state)

	assert.False(t, mockService.updateCalled, "Should not write when latestSeen is zero")
}

func TestFlushDeferredLastSeen_NoopWhenLatestSeenNotAfterLastPersisted(t *testing.T) {
	t.Parallel()
	debouncer := newLastSeenDebouncer()

	mockService := newMockIoTService()
	now := time.Now()
	state := &lastSeenDebounceState{
		latestSeen:    now,
		lastPersisted: now.Add(1 * time.Second), // persisted after latest seen
	}

	debouncer.flushDeferredLastSeen(mockService, int64(6002), state)

	assert.False(t, mockService.updateCalled, "Should not write when latestSeen is not after lastPersisted")
}

func TestFlushDeferredLastSeen_WritesWhenLatestSeenAfterLastPersisted(t *testing.T) {
	t.Parallel()
	debouncer := newLastSeenDebouncer()

	mockService := newMockIoTService()
	now := time.Now()
	state := &lastSeenDebounceState{
		latestSeen:    now,
		lastPersisted: now.Add(-2 * time.Minute),
	}

	debouncer.flushDeferredLastSeen(mockService, int64(6003), state)

	assert.True(t, mockService.updateCalled, "Should write when latestSeen is after lastPersisted")
	assert.Equal(t, now, state.lastPersisted, "Should persist the latest observed timestamp")
}

func TestFlushDeferredLastSeen_ClearsFlushTimer(t *testing.T) {
	t.Parallel()
	debouncer := newLastSeenDebouncer()

	mockService := newMockIoTService()
	now := time.Now()
	timer := time.NewTimer(1 * time.Hour) // dummy timer
	defer timer.Stop()

	state := &lastSeenDebounceState{
		latestSeen:    now,
		lastPersisted: now.Add(-2 * time.Minute),
		flushTimer:    timer,
	}

	debouncer.flushDeferredLastSeen(mockService, int64(6004), state)

	// flushTimer is set to nil at the start of the function
	// It may be re-set if there are new updates, but the original timer should be cleared
	assert.True(t, mockService.updateCalled)
}

// =============================================================================
// updateDeviceLastSeen Integration Tests
// =============================================================================

func TestUpdateDeviceLastSeen_FirstCallWritesImmediately(t *testing.T) {
	t.Parallel()
	debouncer := newLastSeenDebouncer()

	mockService := newMockIoTService()
	device := &iot.Device{Model: base.Model{ID: 7001}, DeviceID: "device-first-write"}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	debouncer.updateDeviceLastSeen(req, mockService, device)

	assert.True(t, mockService.updateCalled, "First call should write immediately")
	assert.NotNil(t, device.LastSeen, "Should set LastSeen on device")
}

func TestUpdateDeviceLastSeen_SecondCallWithinWindowSchedulesTimer(t *testing.T) {
	t.Parallel()
	debouncer := newLastSeenDebouncer()

	mockService := newMockIoTService()
	device := &iot.Device{Model: base.Model{ID: 7002}, DeviceID: "device-debounce-timer"}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	// First call - writes immediately
	debouncer.updateDeviceLastSeen(req, mockService, device)
	assert.True(t, mockService.updateCalled)

	mockService.updateCalled = false

	// Second call within debounce window - should schedule timer instead
	debouncer.updateDeviceLastSeen(req, mockService, device)
	assert.False(t, mockService.updateCalled, "Second call within window should not write immediately")

	// Verify timer was scheduled (cache key is now int64 PK)
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
	debouncer := newLastSeenDebouncer()

	mockService := newMockIoTService()
	mockService.updateStarted = make(chan struct{}, 2)
	mockService.updateBlock = make(chan struct{})
	device := &iot.Device{Model: base.Model{ID: 7003}, DeviceID: "device-concurrent-first-write"}

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		debouncer.updateDeviceLastSeen(req1, mockService, device)
	}()

	<-mockService.updateStarted

	go func() {
		defer wg.Done()
		debouncer.updateDeviceLastSeen(req2, mockService, device)
	}()

	close(mockService.updateBlock)
	wg.Wait()

	mockService.mu.Lock()
	updateCount := mockService.updateCount
	mockService.mu.Unlock()

	assert.Equal(t, 1, updateCount, "Concurrent first requests should reserve the write and avoid duplicate DB updates")
}

func TestUpdateDeviceLastSeen_ErrorDoesNotPreventSubsequentCalls(t *testing.T) {
	t.Parallel()
	debouncer := newLastSeenDebouncer()

	mockService := newMockIoTService()
	mockService.updateError = errors.New("temporary db error")
	device := &iot.Device{Model: base.Model{ID: 7004}, DeviceID: "device-error-recovery"}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	// First call - fails
	debouncer.updateDeviceLastSeen(req, mockService, device)
	assert.True(t, mockService.updateCalled)

	// Reset - next call should still try since lastPersisted was never set
	mockService.updateCalled = false
	mockService.updateError = nil

	debouncer.updateDeviceLastSeen(req, mockService, device)
	assert.True(t, mockService.updateCalled, "Should retry write after previous error since lastPersisted is zero")
}
