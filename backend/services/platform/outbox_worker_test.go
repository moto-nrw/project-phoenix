package platform

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubOutboxRepo is a deterministic in-memory test double for
// EmailOutboxRepository. The worker tests don't need a real DB —
// they're checking dispatch decisions, not the SQL.
type stubOutboxRepo struct {
	mu          sync.Mutex
	due         []*platformModels.EmailOutbox
	claimedIDs  []int64
	sent        []int64
	retried     []retryEntry
	failed      []failEntry
	claimErr    error
	markSentErr error
}

type retryEntry struct {
	ID       int64
	Attempts int
	Err      string
	NextAt   time.Time
}

type failEntry struct {
	ID       int64
	Attempts int
	Err      string
}

func (s *stubOutboxRepo) Create(_ context.Context, _ *platformModels.EmailOutbox) error {
	return errors.New("not implemented in stub")
}

func (s *stubOutboxRepo) FindByID(_ context.Context, _ int64) (*platformModels.EmailOutbox, error) {
	return nil, errors.New("not implemented in stub")
}

func (s *stubOutboxRepo) ClaimDuePending(_ context.Context, _ int, _ time.Time) ([]*platformModels.EmailOutbox, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimErr != nil {
		return nil, s.claimErr
	}
	out := s.due
	for _, row := range out {
		s.claimedIDs = append(s.claimedIDs, row.ID)
	}
	s.due = nil
	return out, nil
}

func (s *stubOutboxRepo) MarkSent(_ context.Context, id int64, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.markSentErr != nil {
		return s.markSentErr
	}
	s.sent = append(s.sent, id)
	return nil
}

func (s *stubOutboxRepo) MarkRetry(_ context.Context, id int64, attempts int, errMsg string, nextAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retried = append(s.retried, retryEntry{ID: id, Attempts: attempts, Err: errMsg, NextAt: nextAt})
	return nil
}

func (s *stubOutboxRepo) MarkFailed(_ context.Context, id int64, attempts int, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failed = append(s.failed, failEntry{ID: id, Attempts: attempts, Err: errMsg})
	return nil
}

func (s *stubOutboxRepo) FindByRelatedEntity(_ context.Context, _ string, _ int64) ([]*platformModels.EmailOutbox, error) {
	return nil, errors.New("not implemented in stub")
}

// stubMailer captures dispatched messages.
type stubMailer struct {
	mu      sync.Mutex
	sent    []email.Message
	sendErr error
}

func (m *stubMailer) Send(msg email.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sent = append(m.sent, msg)
	return nil
}

func (m *stubMailer) SendAsync(msg email.Message) {
	_ = m.Send(msg)
}

// TestTemplateRegistry_LookupRoundTrip — register + lookup + kinds.
func TestTemplateRegistry_LookupRoundTrip(t *testing.T) {
	reg := NewTemplateRegistry()
	reg.Register("test_kind", RendererFunc(func(_ context.Context, _ *platformModels.EmailOutbox) (*email.Message, error) {
		return &email.Message{Subject: "ok"}, nil
	}))

	rdr, err := reg.Lookup("test_kind")
	require.NoError(t, err)
	require.NotNil(t, rdr)

	_, err = reg.Lookup("missing_kind")
	require.Error(t, err)

	kinds := reg.Kinds()
	assert.Equal(t, []string{"test_kind"}, kinds)
}

// TestPickBackoff_ClampsAtCeiling — backoff schedule must not panic on
// large attempts counts.
func TestPickBackoff_ClampsAtCeiling(t *testing.T) {
	last := defaultBackoff[len(defaultBackoff)-1]
	assert.Equal(t, last, pickBackoff(99))
	// First retry uses index 0.
	assert.Equal(t, defaultBackoff[0], pickBackoff(0))
	assert.Equal(t, defaultBackoff[0], pickBackoff(-1))
}

// TestOutboxService_Enqueue_PopulatesPayloadAndStatus — Enqueue defaults
// status to pending and clones nil payload to empty map.
func TestOutboxService_Enqueue_RejectsMissingKind(t *testing.T) {
	svc := NewOutboxService(&stubOutboxRepo{})
	_, err := svc.Enqueue(context.Background(), EnqueueRequest{Kind: ""})
	require.Error(t, err)
}
