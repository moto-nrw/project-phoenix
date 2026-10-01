package email

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sentryRecordingTransport chan *sentry.Event

func (t sentryRecordingTransport) Configure(sentry.ClientOptions)        {}
func (t sentryRecordingTransport) Flush(time.Duration) bool              { return true }
func (t sentryRecordingTransport) FlushWithContext(context.Context) bool { return true }
func (t sentryRecordingTransport) Close()                                {}
func (t sentryRecordingTransport) SendEvent(event *sentry.Event)         { t <- event }

// dispatchWithRecordingSentry dispatches one message with up to attempts
// attempts, waits for callbacks delivery results and returns the recording
// transport. The delivery takes its Sentry client from the hub on the
// dispatch context.
func dispatchWithRecordingSentry(t *testing.T, mailer *mockMailer, attempts, callbacks int) (sentryRecordingTransport, *callbackTracker) {
	t.Helper()
	transport := make(sentryRecordingTransport, 4)
	client, err := sentry.NewClient(sentry.ClientOptions{Transport: transport})
	require.NoError(t, err)
	ctx := sentry.SetHubOnContext(context.Background(), sentry.NewHub(client, sentry.NewScope()))

	dispatcher := NewDispatcher(mailer, slog.New(slog.DiscardHandler))
	dispatcher.SetDefaults(attempts, []time.Duration{time.Millisecond})
	tracker := newCallbackTracker()
	dispatcher.Dispatch(ctx, DeliveryRequest{
		Message:  Message{Subject: "Einladung"},
		Metadata: DeliveryMetadata{Type: "invitation"},
		Callback: tracker.callback,
	})
	require.True(t, tracker.waitForResults(callbacks, time.Second))
	return transport, tracker
}

// Issue #3640: an email that fails for good after all retries is exactly one
// Sentry event with the failed attempts as breadcrumbs.
func TestDispatcher_Dispatch_FinalFailureSendsOneSentryEvent(t *testing.T) {
	t.Parallel()

	mailer := newMockMailer()
	mailer.sendError = errors.New("421 service not available")
	mailer.alwaysFail = true

	transport, _ := dispatchWithRecordingSentry(t, mailer, 3, 3)

	var event *sentry.Event
	select {
	case event = <-transport:
	case <-time.After(time.Second):
		t.Fatal("final delivery failure was not reported")
	}
	assert.Equal(t, "email-delivery", event.Tags["job"])
	assert.NotContains(t, event.Tags, "school_id")
	require.NotEmpty(t, event.Exception)
	assert.Equal(t, "invitation email delivery failed after 3 attempt(s): 421 service not available",
		event.Exception[len(event.Exception)-1].Value)
	require.Len(t, event.Breadcrumbs, 3)
	for i, crumb := range event.Breadcrumbs {
		assert.Equal(t, "email send attempt failed", crumb.Message)
		assert.Equal(t, i+1, crumb.Data["attempt"])
		assert.Equal(t, "invitation", crumb.Data["email_type"])
	}
	select {
	case extra := <-transport:
		t.Fatalf("a second event was sent: %v", extra.Exception)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestDispatcher_Dispatch_RetriedSuccessSendsNoSentryEvent(t *testing.T) {
	t.Parallel()

	mailer := newMockMailer()
	mailer.sendError = errors.New("421 service not available")
	mailer.setFailCount(1)

	transport, tracker := dispatchWithRecordingSentry(t, mailer, 3, 2)

	results := tracker.getResults()
	require.Len(t, results, 2)
	assert.Equal(t, DeliveryStatusSent, results[1].Status)
	select {
	case event := <-transport:
		t.Fatalf("a delivery made good by a retry was reported: %v", event.Exception)
	case <-time.After(50 * time.Millisecond):
	}
}
