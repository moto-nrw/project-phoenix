package compose

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// pendingContractSend reproduces the send transaction's persistence protocol:
// lock the contract before enqueueing its PDF, mailing and send event.
func pendingContractSend(t *testing.T, order demoOrder) (bun.Tx, int64, int64) {
	t.Helper()
	ctx := t.Context()
	tx, err := order.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.NewRaw(`SELECT id FROM care_contract.contracts WHERE school_id = ? FOR UPDATE`, order.schoolID).Exec(ctx)
	require.NoError(t, err)
	var mailID int64
	require.NoError(t, tx.NewRaw(`INSERT INTO platform.email_outbox
		(organization_id, kind, recipient, payload, related_entity_type, related_entity_id)
		SELECT contract.organization_id, 'care_contract',
			jsonb_build_object('address', party.email, 'name', ?),
			jsonb_build_object('guardian_name', ?, 'pdf', encode(document.pdf, 'base64')),
			'care_contract', contract.id
		FROM care_contract.contracts AS contract
		JOIN care_contract.contract_parties AS party ON party.contract_id = contract.id
		JOIN care_contract.contract_documents AS document ON document.contract_id = contract.id
		WHERE contract.school_id = ? RETURNING id`, visitorFull, visitorFull, order.schoolID).Scan(ctx, &mailID))
	_, err = tx.NewRaw(`INSERT INTO care_contract.contract_mailings
		(organization_id, contract_id, contract_document_id, outbox_id, recipient_email)
		SELECT document.organization_id, document.contract_id, document.id, ?, 'eltern@example.org'
		FROM care_contract.contract_documents AS document
		JOIN care_contract.contracts AS contract ON contract.id = document.contract_id
		WHERE contract.school_id = ?`, mailID, order.schoolID).Exec(ctx)
	require.NoError(t, err)
	var eventID int64
	require.NoError(t, tx.NewRaw(`INSERT INTO care_contract.contract_events
		(organization_id, contract_id, school_id, action)
		SELECT organization_id, id, school_id, 'send' FROM care_contract.contracts
		WHERE school_id = ? RETURNING id`, order.schoolID).Scan(ctx, &eventID))
	return tx, mailID, eventID
}

func TestDemoErasureIncludesAConcurrentContractSend(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	sender, mailID, eventID := pendingContractSend(t, order)
	order.waitForLocks(t)
	type outcome struct {
		erased int
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		_, erased, err := order.queue.RetireDemoSchools(t.Context(), nil)
		done <- outcome{erased: erased, err: err}
	}()
	// Commit only once cleanup is waiting for the sender's contract lock.
	// Before the fix, mail/event cleanup has already missed these rows.
	require.Eventually(t, func() bool {
		return lockWaits(t, order.db, "retire_expired_demo_schools")
	}, 5*time.Second, time.Millisecond)
	require.NoError(t, sender.Commit())
	select {
	case result := <-done:
		require.NoError(t, result.err)
		require.Equal(t, 1, result.erased, "the send commits within the lock timeout")
	case <-time.After(5 * time.Second):
		t.Fatal("erasure did not finish after the send committed")
	}
	assert.Empty(t, remainingMails(t, order.db, mailID), "the concurrent send leaves no visitor name or PDF")
	var events int
	require.NoError(t, order.db.NewRaw(`SELECT count(*) FROM care_contract.contract_events WHERE id = ?`, eventID).Scan(t.Context(), &events))
	assert.Zero(t, events, "the concurrent send leaves no event")
	order.requireFullyErased(t)
}

func TestDemoErasureDefersAContractSendThatKeepsItsLock(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	before := order.snapshot(t)
	contracts := careContractRows(t, order.db, order.schoolID)
	sender, mailID, _ := pendingContractSend(t, order)

	_, erased := retireNow(t, order.db, order.queue, nil)
	assert.Zero(t, erased)
	assert.Equal(t, before, order.snapshot(t), "the deferred order is unchanged")
	assert.Equal(t, contracts, careContractRows(t, order.db, order.schoolID), "contract cleanup rolls back too")
	require.NoError(t, sender.Commit(), "erasure does not abort the send")
	require.Equal(t, []int64{mailID}, remainingMails(t, order.db, mailID))

	order.requireErasedNext(t)
	assert.Empty(t, remainingMails(t, order.db, mailID))
	assert.Zero(t, careContractRows(t, order.db, order.schoolID)["contract_events of the school"])
}
