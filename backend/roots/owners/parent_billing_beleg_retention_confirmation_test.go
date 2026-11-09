package owners

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// fourthReviewSpellings cite § 147 AO, § 257 HGB or § 14b UStG in spellings
// the citation heuristic does not catch (#4720, fourth review): a Cyrillic
// "АО", abbreviations and a number in words. With them a 2026 invoice and a
// debit were deleted on 2028-01-01. The heuristic accepts them; only the
// structured confirmation of the Träger lets them keep a short period.
var fourthReviewSpellings = []string{
	"АО", "Abgabenordn.", "Abg.-Ord.", "Handelsges.", "§ einhundertsiebenundvierzig Abgabenordn.",
}

// TestInstructionTableRequiresTheTraegersConfirmation: "Vorschriften nicht
// anwendbar" is the only basis below the statutory periods, and the
// database takes it only with the written justification, the instructing
// person, the reference to the written instruction and the structured
// confirmation that § 147 AO, § 257 HGB and § 14b UStG do not apply, with
// the operator who recorded it and when. The text of the basis is no gate:
// the spellings the heuristic misses still need the confirmation.
func TestInstructionTableRequiresTheTraegersConfirmation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Bestätigung", Slug: "beleg-bestaetigung-" + time.Now().Format("150405.000000"),
	}).ID
	operator := testpkg.CreateTestOperator(t, db).ID
	// insert writes a row and rolls it back, so every case finds the type
	// without an active instruction.
	insert := func(columns string, values ...any) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.beleg_retention_instructions (organization_id, beleg_type, trigger_event,
					period, basis, legal_basis, basis_justification, instructed_by, instructed_on, instruction_reference`+columns, values...)
			if err == nil {
				return errRollbackSpelling
			}
			return err
		})
	}
	unconfirmed := func(belegType, legalBasis string) error {
		return insert(`) VALUES (?, ?, 'issued', 1, 'not_applicable', ?, 'Träger ist nicht buchführungspflichtig',
			'Frau Beispiel, Kämmerei', '2026-11-09', 'Mail vom 09.11.2026')`, organization, belegType, legalBasis)
	}
	confirmed := func(period int, basis, legalBasis string, justification *string, operatorID *int64, at *time.Time) error {
		return insert(`, statutes_not_applicable_confirmed, confirmed_by_operator_id, confirmed_at)
			VALUES (?, 'care_invoice', 'issued', ?, ?, ?, ?, 'Frau Beispiel, Kämmerei', '2026-11-09', 'Mail vom 09.11.2026', TRUE, ?, ?)`,
			organization, period, basis, legalBasis, justification, operatorID, at)
	}
	refused := func(err error, name string) {
		t.Helper()
		var pgErr interface{ Field(byte) string }
		require.ErrorAs(t, err, &pgErr, name)
		assert.Equal(t, "23514", pgErr.Field('C'), name)
	}

	for _, belegType := range []string{"care_invoice", "sepa_debit", "holiday_document"} {
		refused(unconfirmed(belegType, "Satzung des Trägers § 12"), belegType+" without the confirmation")
		for _, spelling := range fourthReviewSpellings {
			refused(unconfirmed(belegType, spelling), belegType+" without the confirmation: "+spelling)
		}
	}

	reason, now := "Träger ist nicht buchführungspflichtig", time.Now()
	for _, spelling := range append([]string{"Satzung des Trägers § 12"}, fourthReviewSpellings...) {
		assert.ErrorIs(t, confirmed(1, "not_applicable", spelling, &reason, &operator, &now), errRollbackSpelling,
			"the heuristic accepts %q, the confirmation carries it", spelling)
	}
	refused(confirmed(1, "not_applicable", "Satzung", &reason, nil, &now), "a confirmation without the operator")
	refused(confirmed(1, "not_applicable", "Satzung", &reason, &operator, nil), "a confirmation without its time")
	refused(confirmed(8, "booking_voucher", "§ 147 AO", nil, &operator, &now), "a confirmation on a statutory basis")
	refused(insert(`, statutes_not_applicable_confirmed, confirmed_by_operator_id, confirmed_at)
		VALUES (?, 'care_invoice', 'issued', 1, 'not_applicable', 'Satzung', ?, 'Frau Beispiel', '2026-11-09', '  ', TRUE, ?, ?)`,
		organization, reason, operator, now), "a confirmation without the reference to the written instruction")
}
