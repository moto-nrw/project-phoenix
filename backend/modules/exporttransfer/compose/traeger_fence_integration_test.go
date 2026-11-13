package compose

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/exporttransfer"
	"github.com/moto-ogs/project-phoenix/modules/exporttransfer/internal/adapters/sftp"
	"github.com/moto-ogs/project-phoenix/modules/exporttransfer/internal/adapters/sftp/sftptest"
	"github.com/moto-ogs/project-phoenix/modules/exporttransfer/internal/domain"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The one-shot fence, the row level security and the locks of the Träger
// scope (ADR 0218 sections 3 and 5), each on its own: every test turns red
// when its protection alone is removed.

func requestTx(t *testing.T, ctx context.Context) bun.Tx {
	t.Helper()
	transaction, ok := tenant.TransactionFromContext(ctx)
	require.True(t, ok)
	tx, ok := transaction.(bun.Tx)
	require.True(t, ok)
	return tx
}

func (f traegerFixture) intent(t *testing.T) int64 {
	t.Helper()
	recipient := f.readyInput()
	recipient.Name = fmt.Sprintf("Zaun %d", testpkg.UniqueTestTenantID(t))
	recipientID := f.add(t, f.org, recipient)
	var id int64
	require.NoError(t, tenant.WithinTraeger(f.ctx, f.org, func(ctx context.Context) error {
		var err error
		id, err = traegerStore().Begin(ctx, domain.TraegerJournalEntry{OrganizationID: f.org, RecipientID: recipientID, RecipientName: recipient.Name,
			Kind:   exporttransfer.KindParentBillingFinance,
			Format: "sollstellung", Filename: "zaun.csv", TargetHost: "kaemmerei.example.org", TargetPort: 22, TargetDirectory: "/eingang",
			Reason: domain.ReasonInternal})
		return err
	}))
	return id
}

func (f traegerFixture) claim(org, id int64) (string, error) {
	var token string
	err := tenant.WithinTraeger(f.ctx, org, func(ctx context.Context) error {
		var err error
		token, err = traegerStore().Claim(ctx, org, id)
		return err
	})
	return token, err
}

func (f traegerFixture) complete(org, id int64, token string) error {
	return tenant.WithinTraeger(f.ctx, org, func(ctx context.Context) error {
		return traegerStore().Complete(ctx, org, id, token, true, "")
	})
}

func (f traegerFixture) expire(t *testing.T, id int64) {
	t.Helper()
	_, err := f.db.NewRaw(`UPDATE audit.traeger_export_transfers SET dispatch_expires_at = clock_timestamp() - INTERVAL '1 second'
		WHERE organization_id = ? AND id = ?`, f.org, id).Exec(f.ctx)
	require.NoError(t, err)
}

// An intent is claimed once, completed once, only with its token and only by
// its own Träger.
func TestTraegerFenceClaimsAndCompletesOnce(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	id := f.intent(t)
	token, err := f.claim(f.org, id)
	require.NoError(t, err)
	_, err = f.claim(f.org, id)
	require.Error(t, err, "a second claim never replaces the token")
	require.Error(t, f.complete(f.org, id, "falsches-token"), "only the token of the claim completes")
	err = tenant.WithinTraeger(f.ctx, f.other, func(ctx context.Context) error {
		var done bool
		return requestTx(t, ctx).NewRaw(`SELECT audit.complete_traeger_export_transfer(?, ?, ?, true, '')`, f.org, id, token).Scan(ctx, &done)
	})
	require.Error(t, err, "another Träger cannot complete it, even with the token")
	require.NoError(t, f.complete(f.org, id, token))
	require.Error(t, f.complete(f.org, id, token), "a completed dispatch completes once")
}

// An expired intent cannot be claimed, an expired dispatch not completed:
// recovery owns them.
func TestTraegerFenceRefusesExpiredIntentsAndDispatches(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	id := f.intent(t)
	f.expire(t, id)
	_, err := f.claim(f.org, id)
	require.Error(t, err, "an expired intent cannot be claimed")
	id = f.intent(t)
	token, err := f.claim(f.org, id)
	require.NoError(t, err)
	f.expire(t, id)
	require.Error(t, f.complete(f.org, id, token), "an expired dispatch cannot be completed")
}

// The journal policy hides another Träger's rows from an unfiltered query of
// the request role, in a Träger and in a school transaction.
func TestTraegerJournalPolicyHidesForeignAndSchoolRows(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	f.save(t, f.other)
	var settled []exporttransfer.Outcome
	_, err := f.transfers.Transfer(f.ctx, financeRequest(f.other, "fremd.csv", &settled))
	require.NoError(t, err)
	count := func(ctx context.Context) error {
		var visible int
		require.NoError(t, requestTx(t, ctx).NewRaw(`SELECT count(*) FROM audit.traeger_export_transfers`).Scan(ctx, &visible))
		assert.Zero(t, visible)
		return nil
	}
	require.NoError(t, tenant.WithinTraeger(f.ctx, f.org, count))
	testpkg.OwnTenant(t)
	require.NoError(t, tenant.WithinCurrentTenant(testpkg.Ctx(t), count))
}

// Journal and producer settle in one transaction: the producer sees the
// completed journal, nothing outside does before the commit, and a failed
// settlement rolls the completion back.
func TestTraegerSettlementSharesTheJournalTransaction(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	f.save(t, f.org)
	var settled []exporttransfer.Outcome
	request := financeRequest(f.org, "gemeinsam.csv", &settled)
	request.Settle = func(ctx context.Context, _ exporttransfer.Outcome) error {
		var state string
		require.NoError(t, requestTx(t, ctx).NewRaw(`SELECT dispatch_state FROM audit.traeger_export_transfers WHERE organization_id = ?`,
			f.org).Scan(ctx, &state))
		assert.Equal(t, "terminal", state, "the settlement sees the completed journal")
		assert.Equal(t, "dispatching", journalRows(t, f.ctx, f.db, f.org)[0].DispatchState, "nothing outside sees it before the commit")
		return errors.New("settlement refused")
	}
	_, err := f.transfers.Transfer(f.ctx, request)
	require.ErrorIs(t, err, exporttransfer.ErrUnknownSettlement)
	assert.Equal(t, "dispatching", journalRows(t, f.ctx, f.db, f.org)[0].DispatchState, "a failed settlement rolls the journal back")
}

// blockingRewrapper holds the rekey between reading and writing the sealed
// password.
type blockingRewrapper struct{ started, release chan struct{} }

func (w blockingRewrapper) Rewrap(sealed, _ []byte) ([]byte, error) {
	close(w.started)
	<-w.release
	return append([]byte(nil), sealed...), nil
}

// The rekey locks the recipient before it opens the password: a concurrent
// save waits instead of being overwritten with the old password.
func TestTraegerRekeyLocksTheRecipientWhileItOpensThePassword(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	f.save(t, f.org)
	rewrapper := blockingRewrapper{started: make(chan struct{}), release: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		done <- tenant.WithinTraeger(f.ctx, f.org, func(ctx context.Context) error {
			_, err := NewRecipientRekey(rewrapper).Batch(ctx, f.org, 0, 1)
			return err
		})
	}()
	<-rewrapper.started
	err := tenant.WithinTraeger(f.ctx, f.org, func(ctx context.Context) error {
		tx := requestTx(t, ctx)
		if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '200ms'`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE config.traeger_export_recipients SET password_sealed = password_sealed WHERE organization_id = ?`, f.org)
		return err
	})
	close(rewrapper.release)
	require.NoError(t, <-done)
	require.Error(t, err, "a password write waits while the rekey holds the recipient")
}

// A counterpart that answers with the password it received never gets it
// into the log; journal and protocol keep the stable reason.
func TestTraegerHandoverLogsNoPasswordAnEchoingCounterpartReturns(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	f.server = sftptest.StartEchoing(t)
	var logs bytes.Buffer
	transfers, err := NewTraegerTransfers(contextCipher{}, slog.New(slog.NewTextHandler(&logs, nil)),
		sftp.WithAddressPolicy(sftptest.AllowLoopbackPolicy{}))
	require.NoError(t, err)
	recipient, err := transfers.CreateRecipient(f.ctx, f.org, 0, f.readyInput())
	require.NoError(t, err)
	_, err = transfers.AssignFormats(f.ctx, f.org, 0, []exporttransfer.TraegerAssignment{{Format: "sollstellung", RecipientID: &recipient.ID}})
	require.NoError(t, err)
	var settled []exporttransfer.Outcome
	outcome, err := transfers.Transfer(f.ctx, financeRequest(f.org, "echo.csv", &settled))
	require.NoError(t, err)
	assert.False(t, outcome.Transferred)
	assert.Equal(t, domain.ReasonUpload, outcome.Reason)
	require.Contains(t, logs.String(), "Träger export transfer failed")
	assert.NotContains(t, logs.String(), f.server.Password, "the counterpart's text stays out of the log")
	assert.NotContains(t, fmt.Sprint(journalRows(t, f.ctx, f.db, f.org)), f.server.Password)
}

// A counterpart that does not answer the rename leaves the handover open:
// no completion, no settlement, the journal waits for recovery.
func TestTraegerHandoverWithoutAnAnsweredRenameStaysOpen(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	f.server = sftptest.StartDroppingRename(t)
	f.dir = "/"
	f.save(t, f.org)
	var settled []exporttransfer.Outcome
	_, err := f.transfers.Transfer(f.ctx, financeRequest(f.org, "offen.csv", &settled))
	require.ErrorIs(t, err, exporttransfer.ErrUnknownSettlement)
	assert.Empty(t, settled, "an open outcome is never settled as failed")
	rows := journalRows(t, f.ctx, f.db, f.org)
	require.Len(t, rows, 1)
	assert.Equal(t, "dispatching", rows[0].DispatchState)
}
