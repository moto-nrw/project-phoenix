package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The originals of the Jahresbescheide, their sends and the inbox keep a
// Träger to its rows also below row level security (#6347): the Träger
// transaction drops its request role, so each assertion holds only because
// the adapter names the Träger in its own statement. Both Träger share the
// address and the child, so a missing filter shows the other Träger's row.

// noticeLetterScope is a Träger with a released notice run, the notice of
// the permitted child with its sent original, and the notice of a sibling
// the reader may not see, sent too.
type noticeLetterScope struct {
	careScope
	run, notice, letter, siblingLetter, unsentLetter int64
}

func seedNoticeLetterScope(t *testing.T, ctx context.Context, tx bun.IDB, scope careScope) noticeLetterScope {
	t.Helper()
	s := noticeLetterScope{careScope: scope}
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_notice_runs (organization_id, billing_year, period_from,
		period_until, school_ids, status, released_at, issued_on, notice_count, total_cents)
		VALUES (?, 2029, DATE '2029-09-01', DATE '2029-12-31', ARRAY[?]::bigint[], 'released', NOW(), DATE '2029-09-24', 3, 9600)
		RETURNING id`, scope.organization, scope.school).Scan(ctx, &s.run))
	notice := func(sequence int, student int64, sent bool) (int64, int64) {
		var noticeID, letterID int64
		require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_notices (organization_id, run_id, number, number_year,
			number_sequence, issued_on, billing_year, kind, care_contract_id, school_id, school_name, child_first_name,
			child_last_name, student_id, recipient_email, family_email, recipient, payment_method, period_from, period_until, lines,
			previous_cents, total_cents, delta_cents, legal_remedy)
			VALUES (?, ?, ? || '-' || ?::text, 2029, ?, DATE '2029-09-24', 2029, 'initial', ?, ?, 'Schule', 'Mia', 'Beispiel', ?, ?, ?,
			'{}'::jsonb, 'transfer', DATE '2029-09-01', DATE '2029-12-31', '[]'::jsonb, 0, 3200, 3200, 'Widerspruch binnen eines Monats.')
			RETURNING id`, scope.organization, s.run, scope.number, sequence, sequence, sharedLetterContract+int64(sequence),
			scope.school, student, sharedLetterEmail, sharedLetterEmail).Scan(ctx, &noticeID))
		require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_notice_letters (organization_id, run_id, notice_id, filename, pdf)
			VALUES (?, ?, ?, 'gebuehrenbescheid.pdf', '\x255044462d'::bytea) RETURNING id`,
			scope.organization, s.run, noticeID).Scan(ctx, &letterID))
		if sent {
			_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.care_notice_letter_mailings (organization_id, run_id, letter_id,
				delivery_id, recipient_email) VALUES (?, ?, ?, ?, ?)`, scope.organization, s.run, letterID, letterID, sharedLetterEmail)
			require.NoError(t, err)
		}
		return noticeID, letterID
	}
	s.notice, s.letter = notice(1, permittedStudent, true)
	_, s.siblingLetter = notice(2, otherStudent, true)
	_, s.unsentLetter = notice(3, permittedStudent, false)
	return s
}

func TestCareNoticeLetterFiltersKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ownScope := newCareScope(t, db, "Träger Bescheidpost A")
	otherScope := newCareScope(t, db, "Träger Bescheidpost B")
	store := NewStore()
	rollback := errors.New("roll the notice letters back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, ownScope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, ownScope.organization)
		require.NoError(t, err)
		var bypasses bool
		require.NoError(t, tx.NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(ctx, &bypasses))
		require.True(t, bypasses, "the test runs below row level security")
		own := seedNoticeLetterScope(t, ctx, tx, ownScope)
		other := seedNoticeLetterScope(t, ctx, tx, otherScope)

		records, err := store.CareNoticeLetterRecords(ctx, own.organization, other.run, true)
		require.NoError(t, err)
		assert.Empty(t, records, "CareNoticeLetterRecords of another Träger's run")
		_, err = store.CareNoticeLetterRecord(ctx, own.organization, other.run, other.notice)
		assert.ErrorIs(t, err, parentbilling.ErrBillingLetterNotFound, "CareNoticeLetterRecord of another Träger's notice")
		_, err = store.CareNoticeLetterRecord(ctx, own.organization, own.run, other.notice)
		assert.ErrorIs(t, err, parentbilling.ErrBillingLetterNotFound, "CareNoticeLetterRecord with another Träger's notice in the own run")
		record, err := store.CareNoticeLetterRecord(ctx, own.organization, own.run, own.notice)
		require.NoError(t, err)
		assert.Equal(t, own.letter, record.ID)
		sends, err := store.CareNoticeLetterMailings(ctx, own.organization, other.run)
		require.NoError(t, err)
		assert.Empty(t, sends, "CareNoticeLetterMailings of another Träger's run")
		_, err = tx.ExecContext(ctx, "SAVEPOINT second_original")
		require.NoError(t, err)
		_, err = store.InsertCareNoticeLetter(ctx, own.organization, own.run, own.notice, parentbilling.BillingLetterFile{
			Filename: "zweites.pdf", Data: []byte("%PDF-2")})
		assert.ErrorIs(t, err, parentbilling.ErrBillingLettersExist, "a notice keeps its first original")
		_, err = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT second_original")
		require.NoError(t, err)

		children := []parentbilling.ParentDocumentChild{{SchoolID: own.school, StudentID: permittedStudent}}
		inbox, err := store.RecipientDocuments(ctx, own.organization, sharedLetterEmail, children)
		require.NoError(t, err)
		require.Len(t, inbox, 1, "only the own sent notice of the permitted child")
		assert.Equal(t, own.letter, inbox[0].DocumentID)
		assert.True(t, inbox[0].NoticeRun)
		assert.Equal(t, parentbilling.ParentDocumentKindCareFeeNotice, inbox[0].Kind)
		assert.Equal(t, "Betreuung 2029", inbox[0].PeriodName)
		for name, letter := range map[string]int64{
			"another Träger's notice":         other.letter,
			"a sibling without the right":     own.siblingLetter,
			"an original that was never sent": own.unsentLetter,
		} {
			_, _, err := store.RecipientCareNoticeLetter(ctx, own.organization, letter, sharedLetterEmail, children)
			assert.ErrorIs(t, err, parentbilling.ErrParentDocumentNotFound, name)
		}
		_, _, err = store.RecipientCareNoticeLetter(ctx, own.organization, own.letter, "andere@example.org", children)
		assert.ErrorIs(t, err, parentbilling.ErrParentDocumentNotFound, "another address")
		elsewhere := []parentbilling.ParentDocumentChild{{SchoolID: own.school + 1, StudentID: permittedStudent}}
		none, err := store.RecipientDocuments(ctx, own.organization, sharedLetterEmail, elsewhere)
		require.NoError(t, err)
		assert.Empty(t, none, "the right at another school opens nothing here")
		runID, file, err := store.RecipientCareNoticeLetter(ctx, own.organization, own.letter, sharedLetterEmail, children)
		require.NoError(t, err)
		assert.Equal(t, own.run, runID)
		assert.Equal(t, "gebuehrenbescheid.pdf", file.Filename)

		valid, err := store.ValidDocumentPushes(ctx, own.organization, []parentbilling.DocumentPushCandidate{
			{Key: 1, DocumentID: own.letter, Email: sharedLetterEmail, SchoolID: own.school, StudentIDs: []int64{permittedStudent}},
			{Key: 2, DocumentID: other.letter, Email: sharedLetterEmail, SchoolID: other.school, StudentIDs: []int64{permittedStudent}},
			{Key: 3, DocumentID: own.siblingLetter, Email: sharedLetterEmail, SchoolID: own.school, StudentIDs: []int64{permittedStudent}},
			{Key: 4, DocumentID: own.unsentLetter, Email: sharedLetterEmail, SchoolID: own.school, StudentIDs: []int64{permittedStudent}},
		})
		require.NoError(t, err)
		assert.Equal(t, map[int64]bool{1: true}, valid, "only the own sent notice of the permitted child pushes")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// An original and a send never change, for any role (1.15.883).
func TestCareNoticeLettersNeverChange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := newCareScope(t, db, "Träger Bescheidpost C")
	rollback := errors.New("roll back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, scope.organization)
		require.NoError(t, err)
		own := seedNoticeLetterScope(t, ctx, tx, scope)
		for _, statement := range []string{
			`UPDATE parent_billing.care_notice_letters SET filename = 'neu.pdf' WHERE id = ?`,
			`UPDATE parent_billing.care_notice_letters SET pdf = '\x2550'::bytea WHERE id = ?`,
			`UPDATE parent_billing.care_notice_letter_mailings SET recipient_email = 'neu@example.org' WHERE letter_id = ?`,
		} {
			_, err := tx.ExecContext(ctx, "SAVEPOINT change")
			require.NoError(t, err)
			_, err = tx.ExecContext(ctx, statement, own.letter)
			assert.ErrorContains(t, err, "never changes", statement)
			_, err = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT change")
			require.NoError(t, err)
		}
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
