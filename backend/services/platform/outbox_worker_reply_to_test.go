package platform

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

// stubMailIdentity is a behaviourally divergent double (it records the tenant
// it was asked about and can inject an error), so it stays local per Rule 13.
type stubMailIdentity struct {
	identity     email.ReplyToIdentity
	err          error
	askedTenants []int64
}

func (s *stubMailIdentity) ResolveReplyTo(
	_ context.Context,
	tenantID int64,
) (email.ReplyToIdentity, error) {
	s.askedTenants = append(s.askedTenants, tenantID)
	return s.identity, s.err
}

func replyToRenderer(msg *email.Message) *TemplateRegistry {
	registry := NewTemplateRegistry()
	registry.Register("welcome", RendererFunc(func(_ context.Context, _ *platformModels.EmailOutbox) (*email.Message, error) {
		return msg, nil
	}))
	return registry
}

// Every outbox kind is tenant-bound, so the worker — not each renderer — is
// where the reply address is stamped. This pins that the sent message actually
// carries it, and for the row's own tenant (#1936).
func TestOutboxWorker_RunOnce_StampsTenantReplyTo(t *testing.T) {
	t.Parallel()

	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()

	expectAdminTx(mock)
	expectAdminTx(mock)

	row := makeRow(1001, 99, "welcome", 0)
	repo := &stubOutboxRepo{due: []*platformModels.EmailOutbox{row}}
	mailer := &stubMailer{}
	identity := &stubMailIdentity{identity: email.ReplyToIdentity{
		Name:    "OGS Am Berg",
		Address: "ogs@schule.example",
	}}

	w := newMockOutboxWorker(t, OutboxWorkerConfig{
		Repo: repo, Registry: replyToRenderer(&email.Message{
			Subject: "Welcome", To: email.Email{Address: "p@example.test"},
		}), Mailer: mailer, DB: db, MaxAttempts: 3,
	})
	w.SetMailIdentityResolver(identity)

	n, err := w.RunOnce(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	require.Len(t, mailer.sent, 1)
	assert.Equal(t, "ogs@schule.example", mailer.sent[0].ReplyTo.Address)
	assert.Equal(t, "OGS Am Berg", mailer.sent[0].ReplyTo.Name)
	assert.Equal(t, []int64{99}, identity.askedTenants)
	require.NoError(t, mock.ExpectationsWereMet())
}

// A renderer that set its own ReplyTo owns the decision; the worker only fills
// the gap.
func TestOutboxWorker_RunOnce_RendererReplyToWins(t *testing.T) {
	t.Parallel()

	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()

	expectAdminTx(mock)
	expectAdminTx(mock)

	row := makeRow(1001, 99, "welcome", 0)
	repo := &stubOutboxRepo{due: []*platformModels.EmailOutbox{row}}
	mailer := &stubMailer{}

	w := newMockOutboxWorker(t, OutboxWorkerConfig{
		Repo: repo, Registry: replyToRenderer(&email.Message{
			Subject: "Welcome",
			To:      email.Email{Address: "p@example.test"},
			ReplyTo: email.NewEmail("Renderer", "renderer@example.test"),
		}), Mailer: mailer, DB: db, MaxAttempts: 3,
	})
	w.SetMailIdentityResolver(&stubMailIdentity{identity: email.ReplyToIdentity{
		Address: "ogs@schule.example",
	}})

	_, err := w.RunOnce(context.Background(), 10)
	require.NoError(t, err)

	require.Len(t, mailer.sent, 1)
	assert.Equal(t, "renderer@example.test", mailer.sent[0].ReplyTo.Address)
	require.NoError(t, mock.ExpectationsWereMet())
}

// A resolver failure must not cost the mail: it is sent without the header,
// and the row is still marked sent rather than retried.
func TestOutboxWorker_RunOnce_ReplyToLookupFails_StillSends(t *testing.T) {
	t.Parallel()

	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()

	expectAdminTx(mock)
	expectAdminTx(mock)

	row := makeRow(1001, 99, "welcome", 0)
	repo := &stubOutboxRepo{due: []*platformModels.EmailOutbox{row}}
	mailer := &stubMailer{}

	w := newMockOutboxWorker(t, OutboxWorkerConfig{
		Repo: repo, Registry: replyToRenderer(&email.Message{
			Subject: "Welcome", To: email.Email{Address: "p@example.test"},
		}), Mailer: mailer, DB: db, MaxAttempts: 3,
	})
	w.SetMailIdentityResolver(&stubMailIdentity{err: errors.New("lookup failed")})

	_, err := w.RunOnce(context.Background(), 10)
	require.NoError(t, err)

	require.Len(t, mailer.sent, 1)
	assert.Empty(t, mailer.sent[0].ReplyTo.Address)
	assert.Equal(t, []int64{1001}, repo.sent)
	assert.Empty(t, repo.retried)
	require.NoError(t, mock.ExpectationsWereMet())
}

// No resolver wired (tests, CLI paths) must behave exactly as before.
func TestOutboxWorker_RunOnce_NoResolver_SendsWithoutReplyTo(t *testing.T) {
	t.Parallel()

	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()

	expectAdminTx(mock)
	expectAdminTx(mock)

	row := makeRow(1001, 99, "welcome", 0)
	repo := &stubOutboxRepo{due: []*platformModels.EmailOutbox{row}}
	mailer := &stubMailer{}

	w := newMockOutboxWorker(t, OutboxWorkerConfig{
		Repo: repo, Registry: replyToRenderer(&email.Message{
			Subject: "Welcome", To: email.Email{Address: "p@example.test"},
		}), Mailer: mailer, DB: db, MaxAttempts: 3,
	})

	_, err := w.RunOnce(context.Background(), 10)
	require.NoError(t, err)

	require.Len(t, mailer.sent, 1)
	assert.Empty(t, mailer.sent[0].ReplyTo.Address)
	require.NoError(t, mock.ExpectationsWereMet())
}
