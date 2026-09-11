package email

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type synchronousTestMailer struct {
	mu       sync.Mutex
	attempts int
	errors   []error
	started  chan struct{}
	release  chan struct{}
}

type panickingSynchronousMailer struct{}

func (panickingSynchronousMailer) Send(Message) error {
	panic("transport invariant violated")
}

func (m *synchronousTestMailer) Send(message Message) error {
	return m.SendContext(context.Background(), message)
}

func (m *synchronousTestMailer) SendContext(ctx context.Context, _ Message) error {
	m.mu.Lock()
	m.attempts++
	attempt := m.attempts
	m.mu.Unlock()

	if m.started != nil {
		select {
		case m.started <- struct{}{}:
		default:
		}
	}
	if m.release != nil {
		select {
		case <-m.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if attempt <= len(m.errors) {
		return m.errors[attempt-1]
	}
	return nil
}

func (m *synchronousTestMailer) Attempts() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.attempts
}

func synchronousTestRequest() DeliveryRequest {
	return DeliveryRequest{
		Message: Message{
			To:       Email{Address: "recipient@example.invalid"},
			Template: "mfa-email-code.html",
		},
		Metadata: DeliveryMetadata{
			Type:      "mfa_challenge",
			Recipient: "recipient@example.invalid",
		},
		MaxAttempts:   3,
		BackoffPolicy: []time.Duration{time.Millisecond, time.Millisecond},
	}
}

func TestDispatcherDeliverWaitsForTransportAcceptance(t *testing.T) {
	t.Parallel()

	mailer := &synchronousTestMailer{started: make(chan struct{}, 1), release: make(chan struct{})}
	dispatcher := NewDispatcher(mailer, slog.Default())
	returned := make(chan error, 1)

	go func() {
		returned <- dispatcher.Deliver(context.Background(), synchronousTestRequest())
	}()

	select {
	case <-mailer.started:
	case <-time.After(time.Second):
		t.Fatal("transport was not called")
	}
	select {
	case err := <-returned:
		t.Fatalf("Deliver returned before transport acceptance: %v", err)
	default:
	}

	close(mailer.release)
	require.NoError(t, <-returned)
}

func TestDispatcherDeliverRetriesAndReturnsTransportFailure(t *testing.T) {
	t.Parallel()

	transient := errors.New("smtp temporarily unavailable")
	mailer := &synchronousTestMailer{errors: []error{transient, transient}}
	dispatcher := NewDispatcher(mailer, slog.Default())

	require.NoError(t, dispatcher.Deliver(context.Background(), synchronousTestRequest()))
	assert.Equal(t, 3, mailer.Attempts())

	permanent := errors.New("smtp rejected message")
	mailer = &synchronousTestMailer{errors: []error{permanent, permanent, permanent}}
	dispatcher = NewDispatcher(mailer, slog.Default())

	err := dispatcher.Deliver(context.Background(), synchronousTestRequest())
	require.ErrorIs(t, err, permanent)
	assert.ErrorContains(t, err, "after 3 attempt(s)")
	assert.Equal(t, 3, mailer.Attempts())
}

func TestDispatcherDeliverHonorsCancellationDuringTransportAndBackoff(t *testing.T) {
	t.Parallel()

	t.Run("in flight", func(t *testing.T) {
		t.Parallel()
		mailer := &synchronousTestMailer{started: make(chan struct{}, 1), release: make(chan struct{})}
		dispatcher := NewDispatcher(mailer, slog.Default())
		ctx, cancel := context.WithCancel(context.Background())
		returned := make(chan error, 1)
		go func() { returned <- dispatcher.Deliver(ctx, synchronousTestRequest()) }()
		<-mailer.started
		cancel()

		require.ErrorIs(t, <-returned, context.Canceled)
		assert.Equal(t, 1, mailer.Attempts())
	})

	t.Run("retry backoff", func(t *testing.T) {
		t.Parallel()
		transient := errors.New("smtp unavailable")
		mailer := &synchronousTestMailer{errors: []error{transient}}
		dispatcher := NewDispatcher(mailer, slog.Default())
		req := synchronousTestRequest()
		req.BackoffPolicy = []time.Duration{time.Hour}
		ctx, cancel := context.WithCancel(context.Background())
		returned := make(chan error, 1)
		go func() { returned <- dispatcher.Deliver(ctx, req) }()
		require.Eventually(t, func() bool { return mailer.Attempts() == 1 }, time.Second, time.Millisecond)
		cancel()

		require.ErrorIs(t, <-returned, context.Canceled)
		assert.Equal(t, 1, mailer.Attempts())
	})
}

func TestDispatcherDeliverReturnsDeadlineExceededForTransportTimeout(t *testing.T) {
	t.Parallel()

	mailer := &synchronousTestMailer{release: make(chan struct{})}
	dispatcher := NewDispatcher(mailer, slog.Default())
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	err := dispatcher.Deliver(ctx, synchronousTestRequest())

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, 1, mailer.Attempts())
}

func TestDispatcherDeliverReportsStableObserverLabelsWithoutRecipient(t *testing.T) {
	t.Parallel()

	type observation struct {
		transport string
		template  string
		caller    string
		err       error
	}
	observed := make(chan observation, 1)
	observer := func(transport, template, caller string, _ time.Duration, err error) {
		observed <- observation{transport: transport, template: template, caller: caller, err: err}
	}
	dispatcher := NewDispatcher(&synchronousTestMailer{}, slog.Default(), observer)

	require.NoError(t, dispatcher.Deliver(context.Background(), synchronousTestRequest()))
	got := <-observed
	assert.Equal(t, "email", got.transport)
	assert.Equal(t, "mfa-email-code.html", got.template)
	assert.Equal(t, "mfa_challenge", got.caller)
	assert.NoError(t, got.err)
	assert.NotContains(t, got.transport+got.template+got.caller, "recipient")
}

func TestDispatcherDeliverFailsWhenTransportIsUnavailable(t *testing.T) {
	t.Parallel()

	err := NewDispatcher(nil, slog.Default()).Deliver(context.Background(), synchronousTestRequest())
	require.ErrorIs(t, err, ErrDeliveryUnavailable)
}

func TestDispatcherDeliverConvertsTransportPanicToFailure(t *testing.T) {
	t.Parallel()

	err := NewDispatcher(panickingSynchronousMailer{}, slog.Default()).Deliver(
		context.Background(), synchronousTestRequest(),
	)

	require.Error(t, err)
	assert.ErrorContains(t, err, "panic in synchronous email delivery")
}
