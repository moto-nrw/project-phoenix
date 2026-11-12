package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// careNoticeLetterCopies stores the original of the school's first
// Jahresbescheid, its send with the mail on Delivery's outbox (address and
// PDF of the family) and the push that announced it in the parents inbox
// (#6347). The mail and the push name the notice only through the send, so
// a guard that looks for the visitor's name finds neither. It returns the
// letter, the send, the outbox mail and the push.
func careNoticeLetterCopies(t *testing.T, db *bun.DB, schoolID int64) (letter, mailing, mail, push int64) {
	t.Helper()
	ctx := t.Context()
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_notice_letters (organization_id, run_id, notice_id, filename, pdf)
		SELECT organization_id, run_id, id, 'gebuehrenbescheid.pdf', '\x255044'::bytea FROM parent_billing.care_notices
		WHERE school_id = ? ORDER BY id LIMIT 1
		RETURNING id`, schoolID).Scan(ctx, &letter))
	require.NoError(t, db.NewRaw(`INSERT INTO platform.email_outbox (organization_id, kind, recipient, payload, status, related_entity_type, related_entity_id)
		SELECT organization_id, 'parent_billing_letter', jsonb_build_object('address', 'eltern@example.org'),
			jsonb_build_object('document_kind', 'care_fee_notice'), 'sent', 'parent_billing_care_notice_run', run_id
		FROM parent_billing.care_notice_letters WHERE id = ?
		RETURNING id`, letter).Scan(ctx, &mail))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_notice_letter_mailings (organization_id, run_id, letter_id, delivery_id, recipient_email)
		SELECT organization_id, run_id, id, ?, 'eltern@example.org' FROM parent_billing.care_notice_letters WHERE id = ?
		RETURNING id`, mail, letter).Scan(ctx, &mailing))
	require.NoError(t, db.NewRaw(`INSERT INTO platform.push_outbox (organization_id, kind, related_entity_type, related_entity_id, recipient, payload)
		SELECT organization_id, 'parent_billing_document', 'parent_billing_document', id,
			jsonb_build_object('email', 'eltern@example.org'), jsonb_build_object('document_id', id)
		FROM parent_billing.care_notice_letters WHERE id = ?
		RETURNING id`, letter).Scan(ctx, &push))
	return letter, mailing, mail, push
}

// The originals of a retired demo school's Jahresbescheide go with the
// visitor (1.15.883): the original and its send by cascade with the notice,
// the mail with the family's address and PDF and the inbox push before it. A school of the same Träger that is still open keeps
// all of them.
func TestDemoSchoolQueueErasesTheNoticeLettersOfTheRetiredSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	openID := schoolOfOrganization(t, db, schoolID, false)
	accounts := leaveVisitorTraces(t, db, schoolID)
	leaveVisitorTraces(t, db, openID)
	careNoticeRows(t, db, schoolID)
	careNoticeRows(t, db, openID)
	letter, mailing, mail, push := careNoticeLetterCopies(t, db, schoolID)
	openLetter, openMailing, openMail, openPush := careNoticeLetterCopies(t, db, openID)

	retireVisitorOrder(t, db, schoolID, accounts)

	assert.Zero(t, remainingRows(t, db, "platform.email_outbox", []int64{mail}), "no mail with the family's notice")
	assert.Zero(t, remainingRows(t, db, "platform.push_outbox", []int64{push}), "no push naming the notice")
	assert.Zero(t, remainingRows(t, db, "parent_billing.care_notice_letters", []int64{letter}), "no original of the retired school")
	assert.Zero(t, remainingRows(t, db, "parent_billing.care_notice_letter_mailings", []int64{mailing}), "no send of the retired school")
	assert.Equal(t, 1, remainingRows(t, db, "parent_billing.care_notice_letters", []int64{openLetter}), "the open school keeps its original")
	assert.Equal(t, 1, remainingRows(t, db, "parent_billing.care_notice_letter_mailings", []int64{openMailing}))
	assert.Equal(t, 1, remainingRows(t, db, "platform.email_outbox", []int64{openMail}), "the open school keeps its mail")
	assert.Equal(t, 1, remainingRows(t, db, "platform.push_outbox", []int64{openPush}), "the open school keeps its push")
}
