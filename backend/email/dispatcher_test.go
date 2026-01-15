package email

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
// Mock Mailer
// =============================================================================

type mockMailer struct {
	mu           sync.Mutex
	messages     []Message
	sendError    error
	sendAttempts int
	failCount    int  // Number of times to fail before succeeding
	alwaysFail   bool // If true, always fail when sendError is set
}

func newMockMailer() *mockMailer {
	return &mockMailer{}
}

func (m *mockMailer) Send(msg Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sendAttempts++

	// When failCount is set, fail that many times then succeed
	if m.failCount > 0 {
		m.failCount--
		if m.sendError != nil {
			return m.sendError
		}
		return errors.New("temporary failure")
	}

	// If failCount reached 0 (was set before), we should succeed now
	// If failCount was never set (still 0) and sendError is set, always fail
	// We track this by checking if sendError is set but we haven't failed yet
	// This is handled by the alwaysFail flag
	if m.alwaysFail && m.sendError != nil {
		return m.sendError
	}

	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockMailer) getSentMessages() []Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]Message, len(m.messages))
	copy(result, m.messages)
	return result
}

func (m *mockMailer) setFailCount(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failCount = count
}

// =============================================================================
// Callback Tracking
// =============================================================================

type callbackTracker struct {
	mu      sync.Mutex
	results []DeliveryResult
	ch      chan struct{}
}

func newCallbackTracker() *callbackTracker {
	return &callbackTracker{
		ch: make(chan struct{}, 10),
	}
}

func (ct *callbackTracker) callback(_ context.Context, result DeliveryResult) {
	ct.mu.Lock()
	ct.results = append(ct.results, result)
	ct.mu.Unlock()

	select {
	case ct.ch <- struct{}{}:
	default:
	}
}

func (ct *callbackTracker) waitForResults(count int, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		ct.mu.Lock()
		currentCount := len(ct.results)
		ct.mu.Unlock()

		if currentCount >= count {
			return true
		}

		select {
		case <-ct.ch:
		case <-timer.C:
			return false
		}
	}
}

func (ct *callbackTracker) getResults() []DeliveryResult {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	result := make([]DeliveryResult, len(ct.results))
	copy(result, ct.results)
	return result
}

// =============================================================================
// NewDispatcher Tests
// =============================================================================

func TestNewDispatcher(t *testing.T) {
	mailer := newMockMailer()
	dispatcher := NewDispatcher(mailer)

	require.NotNil(t, dispatcher)
	assert.Equal(t, 3, dispatcher.defaultRetry)
	assert.Equal(t, 3, len(dispatcher.defaultBackoff))
	assert.Equal(t, time.Minute, dispatcher.defaultBackoff[0])
	assert.Equal(t, 5*time.Minute, dispatcher.defaultBackoff[1])
	assert.Equal(t, 15*time.Minute, dispatcher.defaultBackoff[2])
}

// =============================================================================
// SetDefaults Tests
// =============================================================================

func TestDispatcher_SetDefaults(t *testing.T) {
	mailer := newMockMailer()
	dispatcher := NewDispatcher(mailer)

	// Custom settings
	customBackoff := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond}
	dispatcher.SetDefaults(5, customBackoff)

	assert.Equal(t, 5, dispatcher.defaultRetry)
	assert.Equal(t, customBackoff, dispatcher.defaultBackoff)
}

func TestDispatcher_SetDefaults_NilDispatcher(t *testing.T) {
	var dispatcher *Dispatcher
	// Should not panic
	dispatcher.SetDefaults(5, []time.Duration{time.Second})
}

func TestDispatcher_SetDefaults_ZeroMaxAttempts(t *testing.T) {
	mailer := newMockMailer()
	dispatcher := NewDispatcher(mailer)
	originalRetry := dispatcher.defaultRetry

	// Zero should not change the value
	dispatcher.SetDefaults(0, nil)
	assert.Equal(t, originalRetry, dispatcher.defaultRetry)
}

func TestDispatcher_SetDefaults_EmptyBackoff(t *testing.T) {
	mailer := newMockMailer()
	dispatcher := NewDispatcher(mailer)
	originalBackoff := dispatcher.defaultBackoff

	// Empty slice should not change backoff
	dispatcher.SetDefaults(5, []time.Duration{})
	assert.Equal(t, originalBackoff, dispatcher.defaultBackoff)
}

// =============================================================================
// Dispatch Tests - Successful Delivery
// =============================================================================

func TestDispatcher_Dispatch_Success(t *testing.T) {
	mailer := newMockMailer()
	dispatcher := NewDispatcher(mailer)
	// Use short backoff for testing
	dispatcher.SetDefaults(3, []time.Duration{1 * time.Millisecond, 2 * time.Millisecond})

	tracker := newCallbackTracker()

	msg := Message{
		From:    Email{Name: "Test", Address: "test@example.com"},
		To:      Email{Name: "Recipient", Address: "recipient@example.com"},
		Subject: "Test Email",
	}

	req := DeliveryRequest{
		Message: msg,
		Metadata: DeliveryMetadata{
			Type:        "test",
			ReferenceID: 123,
			Recipient:   "recipient@example.com",
		},
		Callback: tracker.callback,
	}

	dispatcher.Dispatch(context.Background(), req)

	// Wait for callback
	require.True(t, tracker.waitForResults(1, 500*time.Millisecond))

	results := tracker.getResults()
	require.Len(t, results, 1)
	assert.Equal(t, DeliveryStatusSent, results[0].Status)
	assert.Equal(t, 1, results[0].Attempt)
	assert.True(t, results[0].Final)
	assert.Nil(t, results[0].Err)

	// Verify message was sent
	messages := mailer.getSentMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "Test Email", messages[0].Subject)
}

func TestDispatcher_Dispatch_NilMailer(t *testing.T) {
	dispatcher := NewDispatcher(nil)

	tracker := newCallbackTracker()

	req := DeliveryRequest{
		Message:  Message{Subject: "Test"},
		Callback: tracker.callback,
	}

	// Should not panic or call callback
	dispatcher.Dispatch(context.Background(), req)

	// Wait briefly to ensure nothing happens
	time.Sleep(50 * time.Millisecond)
	results := tracker.getResults()
	assert.Empty(t, results)
}

func TestDispatcher_Dispatch_NoCallback(t *testing.T) {
	mailer := newMockMailer()
	dispatcher := NewDispatcher(mailer)

	msg := Message{
		From:    Email{Name: "Test", Address: "test@example.com"},
		To:      Email{Name: "Recipient", Address: "recipient@example.com"},
		Subject: "Test Email",
	}

	req := DeliveryRequest{
		Message:  msg,
		Callback: nil, // No callback
	}

	dispatcher.Dispatch(context.Background(), req)

	// Wait for async delivery
	time.Sleep(50 * time.Millisecond)

	// Verify message was sent
	messages := mailer.getSentMessages()
	require.Len(t, messages, 1)
}

// =============================================================================
// Dispatch Tests - Retry Behavior
// =============================================================================

func TestDispatcher_Dispatch_RetryOnFailure(t *testing.T) {
	mailer := newMockMailer()
	mailer.sendError = errors.New("SMTP error")
	mailer.setFailCount(2) // Fail twice, then succeed

	dispatcher := NewDispatcher(mailer)
	dispatcher.SetDefaults(3, []time.Duration{1 * time.Millisecond, 2 * time.Millisecond})

	tracker := newCallbackTracker()

	req := DeliveryRequest{
		Message: Message{Subject: "Test"},
		Metadata: DeliveryMetadata{
			Type:      "test",
			Recipient: "test@example.com",
		},
		Callback: tracker.callback,
	}

	dispatcher.Dispatch(context.Background(), req)

	// Wait for all callbacks (2 failures + 1 success = 3)
	require.True(t, tracker.waitForResults(3, 500*time.Millisecond))

	results := tracker.getResults()
	require.Len(t, results, 3)

	// First two should be failures
	assert.Equal(t, DeliveryStatusFailed, results[0].Status)
	assert.Equal(t, 1, results[0].Attempt)
	assert.False(t, results[0].Final)
	assert.NotNil(t, results[0].Err)

	assert.Equal(t, DeliveryStatusFailed, results[1].Status)
	assert.Equal(t, 2, results[1].Attempt)
	assert.False(t, results[1].Final)

	// Third should be success
	assert.Equal(t, DeliveryStatusSent, results[2].Status)
	assert.Equal(t, 3, results[2].Attempt)
	assert.True(t, results[2].Final)
	assert.Nil(t, results[2].Err)
}

func TestDispatcher_Dispatch_AllRetriesFail(t *testing.T) {
	mailer := newMockMailer()
	mailer.sendError = errors.New("permanent SMTP error")
	mailer.alwaysFail = true // Always fail, not just for failCount times

	dispatcher := NewDispatcher(mailer)
	dispatcher.SetDefaults(3, []time.Duration{1 * time.Millisecond, 2 * time.Millisecond})

	tracker := newCallbackTracker()

	req := DeliveryRequest{
		Message: Message{Subject: "Test"},
		Metadata: DeliveryMetadata{
			Type:      "test",
			Recipient: "test@example.com",
		},
		Callback: tracker.callback,
	}

	dispatcher.Dispatch(context.Background(), req)

	// Wait for all callbacks
	require.True(t, tracker.waitForResults(3, 500*time.Millisecond))

	results := tracker.getResults()
	require.Len(t, results, 3)

	// All should be failures
	for i, result := range results {
		assert.Equal(t, DeliveryStatusFailed, result.Status)
		assert.Equal(t, i+1, result.Attempt)
		assert.NotNil(t, result.Err)
	}

	// Last one should be final
	assert.False(t, results[0].Final)
	assert.False(t, results[1].Final)
	assert.True(t, results[2].Final)
}

// =============================================================================
// Dispatch Tests - Custom Configuration
// =============================================================================

func TestDispatcher_Dispatch_CustomMaxAttempts(t *testing.T) {
	mailer := newMockMailer()
	mailer.sendError = errors.New("error")
	mailer.alwaysFail = true // Always fail for this test

	dispatcher := NewDispatcher(mailer)
	dispatcher.SetDefaults(3, []time.Duration{1 * time.Millisecond})

	tracker := newCallbackTracker()

	req := DeliveryRequest{
		Message:     Message{Subject: "Test"},
		MaxAttempts: 2, // Custom: only 2 attempts
		Callback:    tracker.callback,
	}

	dispatcher.Dispatch(context.Background(), req)

	require.True(t, tracker.waitForResults(2, 500*time.Millisecond))

	results := tracker.getResults()
	assert.Len(t, results, 2)
	assert.True(t, results[1].Final)
}

func TestDispatcher_Dispatch_CustomBackoff(t *testing.T) {
	mailer := newMockMailer()
	mailer.setFailCount(1)

	dispatcher := NewDispatcher(mailer)
	dispatcher.SetDefaults(3, []time.Duration{1 * time.Millisecond})

	tracker := newCallbackTracker()

	// Custom backoff policy
	customBackoff := []time.Duration{10 * time.Millisecond}

	req := DeliveryRequest{
		Message:       Message{Subject: "Test"},
		BackoffPolicy: customBackoff,
		Callback:      tracker.callback,
	}

	start := time.Now()
	dispatcher.Dispatch(context.Background(), req)

	require.True(t, tracker.waitForResults(2, 500*time.Millisecond))
	elapsed := time.Since(start)

	// Should have waited at least the custom backoff time
	assert.True(t, elapsed >= 10*time.Millisecond)

	results := tracker.getResults()
	assert.Len(t, results, 2)
}

// =============================================================================
// DeliveryMetadata Tests
// =============================================================================

func TestDispatcher_Dispatch_MetadataPassedToCallback(t *testing.T) {
	mailer := newMockMailer()
	dispatcher := NewDispatcher(mailer)

	tracker := newCallbackTracker()

	metadata := DeliveryMetadata{
		Type:        "invitation",
		ReferenceID: 12345,
		Token:       "secret-token",
		Recipient:   "user@example.com",
	}

	req := DeliveryRequest{
		Message:  Message{Subject: "Test"},
		Metadata: metadata,
		Callback: tracker.callback,
	}

	dispatcher.Dispatch(context.Background(), req)

	require.True(t, tracker.waitForResults(1, 500*time.Millisecond))

	results := tracker.getResults()
	require.Len(t, results, 1)

	assert.Equal(t, "invitation", results[0].Metadata.Type)
	assert.Equal(t, int64(12345), results[0].Metadata.ReferenceID)
	assert.Equal(t, "secret-token", results[0].Metadata.Token)
	assert.Equal(t, "user@example.com", results[0].Metadata.Recipient)
}

// =============================================================================
// Message Copy Tests (Race Condition Prevention)
// =============================================================================

func TestDispatcher_Dispatch_MessageCopied(t *testing.T) {
	mailer := newMockMailer()
	dispatcher := NewDispatcher(mailer)

	tracker := newCallbackTracker()

	msg := Message{
		From:    Email{Name: "Original", Address: "original@example.com"},
		To:      Email{Name: "Recipient", Address: "recipient@example.com"},
		Subject: "Original Subject",
	}

	req := DeliveryRequest{
		Message:  msg,
		Callback: tracker.callback,
	}

	dispatcher.Dispatch(context.Background(), req)

	// Modify the original message after dispatch
	msg.Subject = "Modified Subject"

	require.True(t, tracker.waitForResults(1, 500*time.Millisecond))

	// The sent message should have the original subject
	messages := mailer.getSentMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "Original Subject", messages[0].Subject)
}

// =============================================================================
// backoffDuration Tests
// =============================================================================

func TestBackoffDuration(t *testing.T) {
	backoff := []time.Duration{time.Second, 2 * time.Second, 5 * time.Second}

	testCases := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, time.Second},
		{2, 2 * time.Second},
		{3, 5 * time.Second},
		{4, 5 * time.Second}, // Beyond array, use last
		{10, 5 * time.Second},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			result := backoffDuration(backoff, tc.attempt)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBackoffDuration_ZeroAttempt(t *testing.T) {
	backoff := []time.Duration{time.Second, 2 * time.Second}
	result := backoffDuration(backoff, 0)
	assert.Equal(t, time.Second, result)
}

func TestBackoffDuration_NegativeAttempt(t *testing.T) {
	backoff := []time.Duration{time.Second, 2 * time.Second}
	result := backoffDuration(backoff, -1)
	assert.Equal(t, time.Second, result)
}

// =============================================================================
// DeliveryStatus Tests
// =============================================================================

func TestDeliveryStatus_Values(t *testing.T) {
	assert.Equal(t, DeliveryStatus("pending"), DeliveryStatusPending)
	assert.Equal(t, DeliveryStatus("sent"), DeliveryStatusSent)
	assert.Equal(t, DeliveryStatus("failed"), DeliveryStatusFailed)
}

// =============================================================================
// DeliveryResult Tests
// =============================================================================

func TestDeliveryResult_SentAt(t *testing.T) {
	mailer := newMockMailer()
	dispatcher := NewDispatcher(mailer)

	tracker := newCallbackTracker()

	before := time.Now()

	req := DeliveryRequest{
		Message:  Message{Subject: "Test"},
		Callback: tracker.callback,
	}

	dispatcher.Dispatch(context.Background(), req)

	require.True(t, tracker.waitForResults(1, 500*time.Millisecond))

	after := time.Now()

	results := tracker.getResults()
	require.Len(t, results, 1)

	// SentAt should be between before and after
	assert.True(t, results[0].SentAt.After(before) || results[0].SentAt.Equal(before))
	assert.True(t, results[0].SentAt.Before(after) || results[0].SentAt.Equal(after))
}

// =============================================================================
// Concurrent Dispatch Tests
// =============================================================================

func TestDispatcher_Dispatch_Concurrent(t *testing.T) {
	mailer := newMockMailer()
	dispatcher := NewDispatcher(mailer)

	tracker := newCallbackTracker()

	const numMessages = 10

	var wg sync.WaitGroup
	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := DeliveryRequest{
				Message: Message{Subject: "Test " + string(rune('A'+idx))},
				Metadata: DeliveryMetadata{
					ReferenceID: int64(idx),
				},
				Callback: tracker.callback,
			}
			dispatcher.Dispatch(context.Background(), req)
		}(i)
	}

	wg.Wait()

	// Wait for all callbacks
	require.True(t, tracker.waitForResults(numMessages, 2*time.Second))

	results := tracker.getResults()
	assert.Len(t, results, numMessages)

	// All should be successful
	for _, result := range results {
		assert.Equal(t, DeliveryStatusSent, result.Status)
	}

	// All messages should be sent
	messages := mailer.getSentMessages()
	assert.Len(t, messages, numMessages)
}

// =============================================================================
// Context Propagation Tests
// =============================================================================

func TestDispatcher_Dispatch_ContextPassedToCallback(t *testing.T) {
	mailer := newMockMailer()
	dispatcher := NewDispatcher(mailer)

	type contextKey string
	const testKey contextKey = "test-key"

	var receivedCtx context.Context
	var mu sync.Mutex

	callback := func(ctx context.Context, _ DeliveryResult) {
		mu.Lock()
		receivedCtx = ctx
		mu.Unlock()
	}

	ctx := context.WithValue(context.Background(), testKey, "test-value")

	req := DeliveryRequest{
		Message:  Message{Subject: "Test"},
		Callback: callback,
	}

	dispatcher.Dispatch(ctx, req)

	// Wait for callback
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	require.NotNil(t, receivedCtx)
	assert.Equal(t, "test-value", receivedCtx.Value(testKey))
}
