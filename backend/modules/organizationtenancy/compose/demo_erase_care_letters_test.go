package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// careLetterCopies writes the letter of the school's monthly invoice, its
// send with the mail on Delivery's outbox (address and PDF of the family) and
// the push that announced it in the parents inbox (#4684, ADR 0165). The
// guard of #6017 counts tables, not the kinds of rows in Delivery's outbox,
// so this test names the new kinds itself. It returns the outbox mail and
// the push.
func careLetterCopies(t *testing.T, db *bun.DB, schoolID int64) (mail, push int64) {
	t.Helper()
	require.NoError(t, db.NewRaw(`WITH invoice AS (
			SELECT id, organization_id, run_id FROM parent_billing.care_invoices WHERE school_id = ?0),
		run_letter AS (
			INSERT INTO parent_billing.care_run_letters (organization_id, run_id, payment_due_on)
			SELECT organization_id, run_id, DATE '2026-09-15' FROM invoice)
		INSERT INTO parent_billing.care_letters (organization_id, run_id, invoice_id, kind, filename, pdf)
		SELECT organization_id, run_id, id, 'invoice', 'rechnung.pdf', '\x255044'::bytea FROM invoice
		RETURNING id`, schoolID).Scan(t.Context(), &push))
	require.NoError(t, db.NewRaw(`WITH letter AS (
			SELECT letter.id, letter.organization_id, letter.run_id FROM parent_billing.care_letters AS letter
			JOIN parent_billing.care_invoices AS invoice ON invoice.id = letter.invoice_id WHERE invoice.school_id = ?0),
		outbox AS (
			INSERT INTO platform.email_outbox (organization_id, kind, recipient, payload, status, related_entity_type, related_entity_id)
			SELECT organization_id, 'parent_billing_letter', jsonb_build_object('address', 'eltern@example.org', 'name', '`+visitorFull+`'),
				jsonb_build_object('guardian_name', '`+visitorFull+`', 'document_kind', 'care_invoice'), 'pending',
				'parent_billing_care_run', run_id FROM letter
			RETURNING id, organization_id),
		mailing AS (
			INSERT INTO parent_billing.care_letter_mailings (organization_id, run_id, letter_id, delivery_id, recipient_email)
			SELECT letter.organization_id, letter.run_id, letter.id, outbox.id, 'eltern@example.org'
			FROM letter JOIN outbox ON outbox.organization_id = letter.organization_id)
		SELECT id FROM outbox`, schoolID).Scan(t.Context(), &mail))
	require.NoError(t, db.NewRaw(`INSERT INTO platform.push_outbox (organization_id, kind, related_entity_type, related_entity_id, recipient, payload)
		SELECT organization_id, 'parent_billing_document', 'parent_billing_document', id,
			jsonb_build_object('email', 'eltern@example.org'), jsonb_build_object('title', 'Rechnung für `+visitorFull+`')
		FROM parent_billing.care_letters WHERE id = ?
		RETURNING id`, push).Scan(t.Context(), &push))
	return mail, push
}

// The letters of a retired demo school's monthly invoices go with the
// visitor (1.15.809): the letter and its send by cascade with the invoice,
// the mail with the family's address and PDF and the inbox push before it.
// A school of the same Träger that is still open keeps all of them.
func TestDemoSchoolQueueErasesTheMonthlyLettersOfTheRetiredSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	openID := schoolOfOrganization(t, db, schoolID, false)
	accounts := leaveVisitorTraces(t, db, schoolID)
	leaveVisitorTraces(t, db, openID)
	mail, push := careLetterCopies(t, db, schoolID)
	openMail, openPush := careLetterCopies(t, db, openID)
	var letters, mailings []int64
	require.NoError(t, db.NewRaw(`SELECT letter.id FROM parent_billing.care_letters AS letter
		JOIN parent_billing.care_invoices AS invoice ON invoice.id = letter.invoice_id WHERE invoice.school_id = ?`, schoolID).
		Scan(t.Context(), &letters))
	require.NoError(t, db.NewRaw(`SELECT id FROM parent_billing.care_letter_mailings WHERE letter_id IN (?)`, bun.List(letters)).
		Scan(t.Context(), &mailings))
	require.Len(t, mailings, 1, "the fixture leaves a send")

	retireVisitorOrder(t, db, schoolID, accounts)

	assert.Zero(t, remainingRows(t, db, "platform.email_outbox", []int64{mail}), "no mail with the family's letter")
	assert.Zero(t, remainingRows(t, db, "platform.push_outbox", []int64{push}), "no push naming the letter")
	assert.Zero(t, remainingRows(t, db, "parent_billing.care_letters", letters), "no letter of the retired school")
	assert.Zero(t, remainingRows(t, db, "parent_billing.care_letter_mailings", mailings), "no send of the retired school")
	assert.Equal(t, 1, remainingRows(t, db, "platform.email_outbox", []int64{openMail}), "the open school keeps its mail")
	assert.Equal(t, 1, remainingRows(t, db, "platform.push_outbox", []int64{openPush}), "the open school keeps its push")
}
