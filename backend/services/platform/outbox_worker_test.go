package platform

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// stubOutboxRepo is a deterministic in-memory test double for
// EmailOutboxRepository. The worker tests don't need a real DB —
// they're checking dispatch decisions, not the SQL.
type stubOutboxRepo struct {
	mu           sync.Mutex
	due          []*platformModels.EmailOutbox
	claimedIDs   []int64
	sent         []int64
	retried      []retryEntry
	failed       []failEntry
	claimErr     error
	markSentErr  error
	cancelledIDs map[int64]bool // LockSending returns false for these
	locked       []int64
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

func (s *stubOutboxRepo) LockSending(_ context.Context, id int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.locked = append(s.locked, id)
	return !s.cancelledIDs[id], nil
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

func (s *stubOutboxRepo) CancelPendingByRelatedEntity(_ context.Context, _ string, _ int64, _ string) (int64, error) {
	return 0, errors.New("not implemented in stub")
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

// -----------------------------------------------------------------------------
// Behavioral tests for the OutboxWorker poll-loop state machine.
//
// The worker wraps repo calls in tenant.WithAdminTx, so even a stub
// repo needs a sqlmock-backed bun.DB to satisfy the BEGIN / SET LOCAL
// ROLE phoenix_admin / COMMIT cycle. The repo work itself happens in
// the stub — these tests are checking dispatch decisions
// (sent/retry/failed/skip), not the SQL of the repo methods.
// -----------------------------------------------------------------------------

// stubMailer captures sends and optionally returns an error.
type stubMailer struct {
	sent []email.Message
	err  error
}

func (m *stubMailer) Send(msg email.Message) error {
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, msg)
	return nil
}

func newAdminTxDB(t *testing.T) (*bun.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	cleanup := func() {
		// Allow the underlying driver to close without an unexpected-close
		// assertion failure; sqlmock treats Close like any other operation.
		mock.ExpectClose()
		_ = db.Close()
	}
	return db, mock, cleanup
}

// expectAdminTx programs a single BEGIN/SET LOCAL ROLE phoenix_admin/COMMIT
// cycle on the mock. The repo call inside the callback is handled by the
// stub repo, not by SQL.
func expectAdminTx(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
}

func makeRow(id, tenantID int64, kind string, attempts int) *platformModels.EmailOutbox {
	row := &platformModels.EmailOutbox{
		Kind:     kind,
		Status:   platformModels.EmailOutboxStatusPending,
		Attempts: attempts,
	}
	row.ID = id
	row.SetTenantID(tenantID)
	return row
}

// TestOutboxWorker_RunOnce_NoRows — empty pickup returns (0, nil) and
// never starts a send cycle.
func TestOutboxWorker_RunOnce_NoRows(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()

	// Phase 1 (claim) runs under WithAdminTx even when the result is empty.
	expectAdminTx(mock)

	registry := NewTemplateRegistry()
	registry.Register("test", RendererFunc(func(_ context.Context, _ *platformModels.EmailOutbox) (*email.Message, error) {
		t.Fatal("renderer must not run when there are no rows")
		return nil, nil
	}))

	repo := &stubOutboxRepo{}
	mailer := &stubMailer{}
	w := NewOutboxWorker(OutboxWorkerConfig{
		Repo: repo, Registry: registry, Mailer: mailer, DB: db, MaxAttempts: 3,
	})

	n, err := w.RunOnce(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Empty(t, mailer.sent)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestOutboxWorker_RunOnce_HappyPath_SendsAndMarksSent — single
// pending row → render → send → MarkSent.
func TestOutboxWorker_RunOnce_HappyPath_SendsAndMarksSent(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()

	expectAdminTx(mock) // Phase 1: ClaimDuePending
	expectAdminTx(mock) // Phase 2: send tx (LockSending + send + MarkSent)

	registry := NewTemplateRegistry()
	registry.Register("welcome", RendererFunc(func(_ context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		assert.Equal(t, int64(1001), row.ID)
		return &email.Message{Subject: "Welcome", To: email.Email{Address: "p@example.test"}}, nil
	}))

	row := makeRow(1001, 99, "welcome", 0)
	repo := &stubOutboxRepo{due: []*platformModels.EmailOutbox{row}}
	mailer := &stubMailer{}

	w := NewOutboxWorker(OutboxWorkerConfig{
		Repo: repo, Registry: registry, Mailer: mailer, DB: db, MaxAttempts: 3,
	})

	n, err := w.RunOnce(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, []int64{1001}, repo.sent)
	assert.Empty(t, repo.retried)
	assert.Empty(t, repo.failed)
	require.Len(t, mailer.sent, 1)
	assert.Equal(t, "Welcome", mailer.sent[0].Subject)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestOutboxWorker_RunOnce_RowCancelledAfterClaim_SkipsSend — when the
// claimed row disappears before the send (a feature deleted its outbox
// rows, e.g. enrollment deletion), the worker must NOT send the already
// rendered message and must not record sent/retry/failed for it.
func TestOutboxWorker_RunOnce_RowCancelledAfterClaim_SkipsSend(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()

	expectAdminTx(mock) // ClaimDuePending
	expectAdminTx(mock) // send tx (LockSending finds the row gone)

	registry := NewTemplateRegistry()
	registry.Register("welcome", RendererFunc(func(_ context.Context, _ *platformModels.EmailOutbox) (*email.Message, error) {
		return &email.Message{Subject: "stale data"}, nil
	}))

	row := makeRow(6006, 99, "welcome", 0)
	repo := &stubOutboxRepo{
		due:          []*platformModels.EmailOutbox{row},
		cancelledIDs: map[int64]bool{6006: true},
	}
	mailer := &stubMailer{}
	w := NewOutboxWorker(OutboxWorkerConfig{
		Repo: repo, Registry: registry, Mailer: mailer, DB: db, MaxAttempts: 3,
	})

	n, err := w.RunOnce(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, []int64{6006}, repo.locked, "the worker must re-check the row before sending")
	assert.Empty(t, mailer.sent, "a cancelled row must never be sent")
	assert.Empty(t, repo.sent)
	assert.Empty(t, repo.retried)
	assert.Empty(t, repo.failed)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestOutboxWorker_RunOnce_RenderError_SchedulesRetry — renderer
// failure on a row well under MaxAttempts goes to MarkRetry, not
// MarkFailed.
func TestOutboxWorker_RunOnce_RenderError_SchedulesRetry(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()

	expectAdminTx(mock) // ClaimDuePending
	expectAdminTx(mock) // MarkRetry

	registry := NewTemplateRegistry()
	registry.Register("welcome", RendererFunc(func(_ context.Context, _ *platformModels.EmailOutbox) (*email.Message, error) {
		return nil, errors.New("render boom")
	}))

	row := makeRow(2002, 99, "welcome", 0)
	repo := &stubOutboxRepo{due: []*platformModels.EmailOutbox{row}}
	mailer := &stubMailer{}

	w := NewOutboxWorker(OutboxWorkerConfig{
		Repo: repo, Registry: registry, Mailer: mailer, DB: db, MaxAttempts: 3,
	})

	n, err := w.RunOnce(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.Len(t, repo.retried, 1)
	assert.Equal(t, int64(2002), repo.retried[0].ID)
	assert.Equal(t, 1, repo.retried[0].Attempts, "attempts increments to 1 after the first failure")
	assert.Contains(t, repo.retried[0].Err, "render boom")
	assert.True(t, repo.retried[0].NextAt.After(time.Now()), "next_retry_at is scheduled in the future")
	assert.Empty(t, repo.sent)
	assert.Empty(t, repo.failed)
	assert.Empty(t, mailer.sent)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestOutboxWorker_RunOnce_SendError_SchedulesRetry — mailer send
// failure on a row under MaxAttempts also retries.
func TestOutboxWorker_RunOnce_SendError_SchedulesRetry(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()

	expectAdminTx(mock) // ClaimDuePending
	expectAdminTx(mock) // send tx (LockSending + failed send)
	expectAdminTx(mock) // MarkRetry

	registry := NewTemplateRegistry()
	registry.Register("welcome", RendererFunc(func(_ context.Context, _ *platformModels.EmailOutbox) (*email.Message, error) {
		return &email.Message{Subject: "Hi"}, nil
	}))

	row := makeRow(3003, 99, "welcome", 1) // already had one failure
	repo := &stubOutboxRepo{due: []*platformModels.EmailOutbox{row}}
	mailer := &stubMailer{err: errors.New("smtp timeout")}

	w := NewOutboxWorker(OutboxWorkerConfig{
		Repo: repo, Registry: registry, Mailer: mailer, DB: db, MaxAttempts: 5,
	})

	n, err := w.RunOnce(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.Len(t, repo.retried, 1)
	assert.Equal(t, 2, repo.retried[0].Attempts, "attempts increments to 2 after the second failure")
	assert.Contains(t, repo.retried[0].Err, "smtp timeout")
	assert.Empty(t, repo.failed)
}

// TestOutboxWorker_RunOnce_ExhaustedAttempts_MarksFailed — when the
// next attempt would equal MaxAttempts, the row goes to terminal
// 'failed' instead of being rescheduled.
func TestOutboxWorker_RunOnce_ExhaustedAttempts_MarksFailed(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()

	expectAdminTx(mock) // ClaimDuePending
	expectAdminTx(mock) // send tx (LockSending + failed send)
	expectAdminTx(mock) // MarkFailed

	registry := NewTemplateRegistry()
	registry.Register("welcome", RendererFunc(func(_ context.Context, _ *platformModels.EmailOutbox) (*email.Message, error) {
		return &email.Message{Subject: "Hi"}, nil
	}))

	// MaxAttempts=3; row already has Attempts=2 → next attempt = 3 → terminal.
	row := makeRow(4004, 99, "welcome", 2)
	repo := &stubOutboxRepo{due: []*platformModels.EmailOutbox{row}}
	mailer := &stubMailer{err: errors.New("still broken")}

	w := NewOutboxWorker(OutboxWorkerConfig{
		Repo: repo, Registry: registry, Mailer: mailer, DB: db, MaxAttempts: 3,
	})

	n, err := w.RunOnce(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.Len(t, repo.failed, 1)
	assert.Equal(t, int64(4004), repo.failed[0].ID)
	assert.Equal(t, 3, repo.failed[0].Attempts, "attempts is logged at the terminal value")
	assert.Empty(t, repo.retried)
	assert.Empty(t, repo.sent)
}

// TestOutboxWorker_RunOnce_UnknownKind_MarksFailedImmediately —
// unknown renderer is terminal regardless of attempts. No backoff.
func TestOutboxWorker_RunOnce_UnknownKind_MarksFailedImmediately(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()

	expectAdminTx(mock) // ClaimDuePending
	expectAdminTx(mock) // MarkFailed

	registry := NewTemplateRegistry()
	// no renderers registered

	row := makeRow(5005, 99, "no_such_kind", 0)
	repo := &stubOutboxRepo{due: []*platformModels.EmailOutbox{row}}
	mailer := &stubMailer{}

	w := NewOutboxWorker(OutboxWorkerConfig{
		Repo: repo, Registry: registry, Mailer: mailer, DB: db, MaxAttempts: 10,
	})

	n, err := w.RunOnce(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.Len(t, repo.failed, 1)
	assert.Contains(t, repo.failed[0].Err, "no renderer registered")
	assert.Empty(t, repo.retried, "unknown-kind never retries")
	assert.Empty(t, mailer.sent)
}

// TestOutboxWorker_RunOnce_ClaimError_Bubbles — a repo-level claim
// error surfaces to the caller and short-circuits the batch.
func TestOutboxWorker_RunOnce_ClaimError_Bubbles(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()

	// The WithAdminTx wrapper still issues BEGIN/SET ROLE before the
	// callback returns its claim error; the wrapper then rolls back.
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	repo := &stubOutboxRepo{claimErr: errors.New("db down")}
	mailer := &stubMailer{}
	w := NewOutboxWorker(OutboxWorkerConfig{
		Repo: repo, Registry: NewTemplateRegistry(), Mailer: mailer, DB: db, MaxAttempts: 3,
	})

	n, err := w.RunOnce(context.Background(), 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "claim due pending")
	assert.Equal(t, 0, n)
	assert.Empty(t, mailer.sent)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestOutboxWorker_RunOnce_MultipleRows_OneBadDoesNotStallBatch — one
// row's failure doesn't stop the rest. The worker processes them
// sequentially and records the right outcome per row.
func TestOutboxWorker_RunOnce_MultipleRows_OneBadDoesNotStallBatch(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()

	expectAdminTx(mock) // ClaimDuePending
	expectAdminTx(mock) // row 1 MarkSent
	expectAdminTx(mock) // row 2 MarkFailed (unknown kind)
	expectAdminTx(mock) // row 3 MarkSent

	registry := NewTemplateRegistry()
	registry.Register("welcome", RendererFunc(func(_ context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		return &email.Message{Subject: "row " + string(rune('0'+row.ID))}, nil
	}))

	good1 := makeRow(1, 99, "welcome", 0)
	bad := makeRow(2, 99, "unknown", 0)
	good2 := makeRow(3, 99, "welcome", 0)

	repo := &stubOutboxRepo{due: []*platformModels.EmailOutbox{good1, bad, good2}}
	mailer := &stubMailer{}
	w := NewOutboxWorker(OutboxWorkerConfig{
		Repo: repo, Registry: registry, Mailer: mailer, DB: db, MaxAttempts: 3,
	})

	n, err := w.RunOnce(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, []int64{1, 3}, repo.sent)
	require.Len(t, repo.failed, 1)
	assert.Equal(t, int64(2), repo.failed[0].ID)
	assert.Len(t, mailer.sent, 2)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestOutboxWorker_RunOnce_MissingDependencies_Errors — bootstrap
// guard. Misconfiguration surfaces before any rows are touched.
func TestOutboxWorker_RunOnce_MissingDependencies_Errors(t *testing.T) {
	db, _, cleanup := newAdminTxDB(t)
	defer cleanup()

	cases := []struct {
		name string
		cfg  OutboxWorkerConfig
		want string
	}{
		{"missing repo", OutboxWorkerConfig{Registry: NewTemplateRegistry(), Mailer: &stubMailer{}, DB: db}, "missing repo"},
		{"missing registry", OutboxWorkerConfig{Repo: &stubOutboxRepo{}, Mailer: &stubMailer{}, DB: db}, "missing registry"},
		{"missing mailer", OutboxWorkerConfig{Repo: &stubOutboxRepo{}, Registry: NewTemplateRegistry(), DB: db}, "missing mailer"},
		{"missing db", OutboxWorkerConfig{Repo: &stubOutboxRepo{}, Registry: NewTemplateRegistry(), Mailer: &stubMailer{}}, "missing db"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := NewOutboxWorker(tc.cfg)
			_, err := w.RunOnce(context.Background(), 10)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
