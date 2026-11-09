package owners

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// belegFixture is one Träger with a holiday invoice of 2026 and its
// cancellation of 2027, a monthly invoice of 2026 collected by SEPA debit
// on a mandate of an ended contract.
type belegFixture struct {
	organization, holidayRun, invoice, cancellation, careInvoice, debit, mandate, contract int64
}

func newBelegFixture(t *testing.T, db *bun.DB, name string) belegFixture {
	t.Helper()
	ctx := context.Background()
	f := belegFixture{}
	f.organization = testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: name, Slug: fmt.Sprintf("beleg-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	school := testpkg.CreateTestSchoolInOrganization(t, db, f.organization, name+" OGS")
	f.contract = testpkg.CreateTestCareContract(t, db, f.organization, school, testpkg.UniqueTestTenantID(t), "versendet", calendar.NewDate(2026, time.August, 1))
	_, err := db.ExecContext(ctx, `UPDATE care_contract.contracts SET concluded_at = '2026-08-01 10:00+02', conclusion_method = 'papier',
		status = 'gekuendigt', ends_on = '2027-07-31' WHERE id = ?`, f.contract)
	require.NoError(t, err)

	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.runs (organization_id, service, holiday_period_id, period_name, service_from,
			service_until, status, released_at, issued_on, document_count, total_cents)
		VALUES (?, 'holiday', ?, 'Sommerferien', '2026-07-06', '2026-07-10', 'released', NOW(), '2026-06-01', 2, 0) RETURNING id`,
		f.organization, testpkg.UniqueTestTenantID(t)).Scan(ctx, &f.holidayRun))
	document := func(number, issued, kind string, reference *int64, total int) int64 {
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year,
				number_sequence, issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name,
				recipient_email, service_from, service_until, total_cents, kind, reference_document_id, reason)
			VALUES (?, ?, 'holiday', ?, 0, ?, ?, 1, 2, 'Eva', 'Muster', 'eva@example.org', '2026-07-06', '2026-07-10', ?, ?, ?, ?) RETURNING id`,
			f.organization, f.holidayRun, number, testpkg.UniqueTestTenantID(t), issued, total, kind, reference,
			map[bool]string{true: "Storno", false: ""}[reference != nil]).Scan(ctx, &id))
		return id
	}
	f.invoice = document("FE-1", "2026-06-01", "invoice", nil, 4500)
	f.cancellation = document("FE-2", "2027-02-01", "cancellation", &f.invoice, -4500)

	var careRun int64
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_runs (organization_id, billing_month, status, released_at, issued_on,
			document_count, total_cents, month_rule) VALUES (?, DATE '2026-10-01', 'released', NOW(), DATE '2026-10-30', 1, 3700, 'version_on_first')
		RETURNING id`, f.organization).Scan(ctx, &careRun))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_invoices (organization_id, run_id, number, number_year, number_sequence,
			issued_on, billing_month, care_contract_id, school_id, school_name, child_first_name, child_last_name, recipient_first_name,
			recipient_last_name, recipient_email, month_rule, total_cents)
		VALUES (?, ?, 'MB-1', 2026, 1, DATE '2026-10-30', DATE '2026-10-01', ?, ?, 'OGS', 'Mia', 'Muster', 'Eva', 'Muster', '',
			'version_on_first', 3700) RETURNING id`, f.organization, careRun, f.contract, school).Scan(ctx, &f.careInvoice))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.sepa_mandates (organization_id, family_email, care_contract_id, reference,
			signed_on, kind, sequence, creditor_id, account_holder_sealed, iban_sealed, iban_masked, source)
		VALUES (?, 'eva@example.org', ?, ?, '2026-09-01', 'recurring', 'first', 'DE98ZZZ09999999999', '\x01'::bytea, '\x02'::bytea, 'DE••', 'office')
		RETURNING id`, f.organization, f.contract, fmt.Sprintf("M-%d", testpkg.UniqueTestTenantID(t))).Scan(ctx, &f.mandate))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.sepa_debits (organization_id, mandate_id, care_contract_id, run_kind, run_id,
			care_invoice_id, invoice_number, amount_cents, due_on, sequence_type, creditor_id, mandate_reference, mandate_signed_on,
			account_holder_sealed, iban_sealed, status, message_id, submitted_at, submitted_amount_cents, collect_on)
		VALUES (?, ?, ?, 'care', ?, ?, 'MB-1', 3700, '2026-11-13', 'FRST', 'DE98ZZZ09999999999', 'M', '2026-09-01',
			'\x01'::bytea, '\x02'::bytea, 'submitted', 'MSG-1', '2026-11-10 09:00+01', 3700, '2026-11-13') RETURNING id`,
		f.organization, f.mandate, f.contract, careRun, f.careInvoice).Scan(ctx, &f.debit))
	return f
}

func instruct(t *testing.T, retention parentbilling.BelegRetention, ctx context.Context, organization int64) {
	t.Helper()
	for _, instruction := range []parentbilling.NewRetentionInstruction{
		{BelegType: parentbilling.BelegHolidayDocument, TriggerEvent: parentbilling.TriggerIssued, Period: 8, Basis: parentbilling.BasisBookingVoucher},
		{BelegType: parentbilling.BelegCareInvoice, TriggerEvent: parentbilling.TriggerIssued, Period: 8, Basis: parentbilling.BasisBookingVoucher},
		{BelegType: parentbilling.BelegSEPADebit, TriggerEvent: parentbilling.TriggerIssued, Period: 8, Basis: parentbilling.BasisBookingVoucher},
		{BelegType: parentbilling.BelegSEPAMandate, TriggerEvent: parentbilling.TriggerLastCollection, Period: 14, Basis: parentbilling.BasisCollectionAgreement},
		{BelegType: parentbilling.BelegCareContract, TriggerEvent: parentbilling.TriggerContractEnded, Period: 6, Basis: parentbilling.BasisBusinessLetter},
	} {
		instruction.LegalBasis, instruction.InstructedBy = "§ 147 AO", "Kämmerei des Trägers"
		instruction.InstructedOn, instruction.InstructionReference = calendar.NewDate(2026, time.November, 9), "AVV-Nachtrag 2026-11"
		_, err := retention.RecordInstruction(ctx, organization, 0, instruction)
		require.NoError(t, err, instruction.BelegType)
	}
}

// TestBelegRetentionDeletesOnlyInstructedBelegeAfterTheirPeriod runs the
// hourly job across the turn of 2034/2035 with a test clock (#4720):
// without an instruction nothing goes and the Beleg waits on the
// clarification list; with one, a Beleg of 2026 goes on 1 January 2035,
// unless a later cancellation still keeps it. Every deletion is logged
// without content, and the records of deleted Belege go with them.
func TestBelegRetentionDeletesOnlyInstructedBelegeAfterTheirPeriod(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	instructed := newBelegFixture(t, db, "Träger mit Weisung")
	open := newBelegFixture(t, db, "Träger ohne Weisung")
	today := calendar.NewDate(2026, time.November, 9)
	retention := NewBelegRetention(func() calendar.Date { return today })
	admin := func(fn func(context.Context) error) {
		t.Helper()
		require.NoError(t, tenant.WithinAdmin(tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, db)), fn))
	}
	run := func(day calendar.Date) int {
		t.Helper()
		today = day
		deleted := 0
		admin(func(adminCtx context.Context) error {
			var err error
			deleted, err = retention.ApplyBelegRetention(adminCtx)
			return err
		})
		return deleted
	}
	exists := func(table string, id int64) bool {
		var found bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM `+table+` WHERE id = ?)`, id).Scan(ctx, &found))
		return found
	}
	overview := func(organization int64) parentbilling.BelegRetentionOverview {
		var result parentbilling.BelegRetentionOverview
		admin(func(adminCtx context.Context) error {
			var err error
			result, err = retention.Overview(adminCtx, organization)
			return err
		})
		return result
	}

	assert.Zero(t, run(today), "production has no instruction: the job deletes nothing")
	listed := overview(instructed.organization)
	require.NotEmpty(t, listed.Clarifications, "every Beleg without instruction is on the clarification list")
	for _, entry := range listed.Clarifications {
		assert.Equal(t, today, entry.ListedOn)
		assert.Equal(t, today.AddDays(30), entry.ReviewOn, entry.BelegType)
		assert.Equal(t, parentbilling.ClarificationOwner, entry.Responsible)
	}

	admin(func(adminCtx context.Context) error {
		instruct(t, retention, adminCtx, instructed.organization)
		return nil
	})
	assert.Zero(t, run(calendar.NewDate(2034, time.December, 31)), "on the last day of 2034 every Beleg of 2026 is still kept")
	for table, id := range map[string]int64{"parent_billing.documents": instructed.invoice, "parent_billing.care_invoices": instructed.careInvoice,
		"parent_billing.sepa_debits": instructed.debit, "parent_billing.sepa_mandates": instructed.mandate, "care_contract.contracts": instructed.contract} {
		assert.True(t, exists(table, id), table)
	}

	deleted := run(calendar.NewDate(2035, time.January, 1))
	assert.False(t, exists("parent_billing.sepa_debits", instructed.debit), "the debit of 2026 goes")
	assert.False(t, exists("parent_billing.care_invoices", instructed.careInvoice), "the invoice of 2026 goes after its debit")
	assert.False(t, exists("parent_billing.sepa_mandates", instructed.mandate), "the expired mandate goes once its debits are gone")
	assert.False(t, exists("care_contract.contracts", instructed.contract), "the contract ended 2027 goes after six years")
	assert.True(t, exists("parent_billing.documents", instructed.invoice), "the cancellation of 2027 still keeps the invoice it refers to")
	assert.True(t, exists("parent_billing.documents", instructed.cancellation))
	assert.GreaterOrEqual(t, deleted, 4)
	for table, id := range map[string]int64{"parent_billing.documents": open.invoice, "parent_billing.care_invoices": open.careInvoice,
		"parent_billing.sepa_debits": open.debit, "parent_billing.sepa_mandates": open.mandate, "care_contract.contracts": open.contract} {
		assert.True(t, exists(table, id), "without instruction the other Träger keeps %s", table)
	}
	after := overview(instructed.organization)
	logged := map[string]int{}
	for _, entry := range after.Deletions {
		assert.Equal(t, calendar.NewDate(2035, time.January, 1), entry.DeletedOn)
		logged[entry.BelegType] += entry.Count
	}
	assert.Equal(t, map[string]int{parentbilling.BelegSEPADebit: 1, parentbilling.BelegCareInvoice: 1,
		parentbilling.BelegSEPAMandate: 1, parentbilling.BelegCareContract: 1}, logged, "the log counts per type, no content")
	var records int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.beleg_retentions WHERE organization_id = ? AND beleg_type <> 'holiday_document'`,
		instructed.organization).Scan(ctx, &records))
	assert.Zero(t, records, "the records of deleted Belege go with them")
	overdue := overview(open.organization)
	require.NotEmpty(t, overdue.Clarifications)
	assert.Positive(t, overdue.Clarifications[0].Overdue, "an unclarified Beleg past its review shows as escalation")

	run(calendar.NewDate(2036, time.January, 1))
	assert.False(t, exists("parent_billing.documents", instructed.cancellation), "the cancellation of 2027 goes in 2036")
	assert.False(t, exists("parent_billing.documents", instructed.invoice), "and with it the invoice")
	assert.False(t, exists("parent_billing.runs", instructed.holidayRun), "the emptied run goes with its letters")
	assert.True(t, exists("parent_billing.documents", open.invoice))
}

// TestBelegRetentionSuspensionHoldsTheDeletion: a suspended type stays
// although its period ended, and goes after the suspension is lifted.
func TestBelegRetentionSuspensionHoldsTheDeletion(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	f := newBelegFixture(t, db, "Träger mit Aussetzung")
	today := calendar.NewDate(2026, time.November, 9)
	retention := NewBelegRetention(func() calendar.Date { return today })
	admin := func(fn func(context.Context) error) {
		t.Helper()
		require.NoError(t, tenant.WithinAdmin(tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, db)), fn))
	}
	var hold parentbilling.RetentionSuspension
	admin(func(adminCtx context.Context) error {
		instruct(t, retention, adminCtx, f.organization)
		var err error
		hold, err = retention.RecordSuspension(adminCtx, f.organization, 0, parentbilling.NewRetentionSuspension{
			BelegType: parentbilling.BelegSEPADebit, Reason: "Betriebsprüfung 2026", Scope: "alle Lastschriften",
			Responsible: "Kämmerei", ReviewOn: calendar.NewDate(2027, time.March, 1),
		})
		return err
	})
	today = calendar.NewDate(2035, time.January, 2)
	admin(func(adminCtx context.Context) error { _, err := retention.ApplyBelegRetention(adminCtx); return err })
	var debits, invoices int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.sepa_debits WHERE id = ?`, f.debit).Scan(ctx, &debits))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.care_invoices WHERE id = ?`, f.careInvoice).Scan(ctx, &invoices))
	assert.Equal(t, 1, debits, "the suspended debit stays")
	assert.Equal(t, 1, invoices, "and keeps the invoice it would take along")

	admin(func(adminCtx context.Context) error {
		return retention.LiftSuspension(adminCtx, f.organization, hold.ID, 0)
	})
	today = calendar.NewDate(2035, time.January, 3)
	admin(func(adminCtx context.Context) error { _, err := retention.ApplyBelegRetention(adminCtx); return err })
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.sepa_debits WHERE id = ?`, f.debit).Scan(ctx, &debits))
	assert.Zero(t, debits, "after the lift the debit goes")
}

// TestBelegRetentionRefusesTheTraegerTransaction: the request role neither
// reads nor writes the retention, and a Träger transaction cannot run it.
func TestBelegRetentionRefusesTheTraegerTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Rolle", Slug: fmt.Sprintf("beleg-rolle-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	for _, table := range []string{"beleg_retention_instructions", "beleg_retention_suspensions", "beleg_retentions", "beleg_deletions"} {
		err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_org_id', ?, true)`, fmt.Sprint(organization)); err != nil {
				return err
			}
			var rows int
			return tx.NewRaw(`SELECT count(*) FROM parent_billing.`+table).Scan(ctx, &rows)
		})
		assert.ErrorContains(t, err, "permission denied", table)
	}
	unit := tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, db))
	err := tenant.WithinTraeger(unit, organization, func(traegerCtx context.Context) error {
		_, err := NewBelegRetention(nil).Overview(traegerCtx, organization)
		return err
	})
	assert.ErrorContains(t, err, "administrative transaction")
}
