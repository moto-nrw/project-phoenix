package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The revenue accounts and cost centers (#4821) below row level security
// (brief rule 25): the test runs without the request role, so only the
// Träger filter of each statement keeps the other Träger's rows out. The
// other Träger holds accounts, cost centers, a price of the same Angebot
// and a position on this Träger's invoice, as only a broken write could
// leave them.
func TestBookingAccountsKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newCareScope(t, db, "Träger Konten A")
	other := newCareScope(t, db, "Träger Konten B")
	store := NewStore()
	rollback := errors.New("roll the accounts back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		own = seedCareScope(t, ctx, tx, own, "Träger Konten A")
		other = seedCareScope(t, ctx, tx, other, "Träger Konten B")

		require.NoError(t, store.SetRevenueAccount(ctx, own.organization, 0, parentbilling.BookingScopeOffering, 101, "4410"))
		require.NoError(t, store.InsertCostCenter(ctx, own.organization, 0, parentbilling.CostCenter{
			Scope: parentbilling.BookingScopeSchool, ScopeID: own.school, ValidFrom: calendar.NewDate(2029, time.September, 1), Code: "ISP"}))
		var foreignCenter int64
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.revenue_accounts (organization_id, scope, scope_id, account)
			VALUES (?, 'offering', 101, '4999'), (?, 'care', 0, '4998')`, other.organization, other.organization)
		require.NoError(t, err)
		require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.cost_centers (organization_id, scope, scope_id, valid_from, code)
			VALUES (?, 'school', ?, DATE '2029-01-01', 'FREMD') RETURNING id`, other.organization, own.school).Scan(ctx, &foreignCenter))

		assignments, err := store.BookingAssignments(ctx, own.organization)
		require.NoError(t, err)
		require.Len(t, assignments.RevenueAccounts, 1, "only this Träger's revenue accounts")
		assert.Equal(t, "4410", assignments.RevenueAccounts[0].Account)
		require.Len(t, assignments.CostCenters, 1, "only this Träger's cost centers")
		assert.Equal(t, "ISP", assignments.CostCenters[0].Code)

		offerings, err := store.BookingOfferings(ctx, own.organization)
		require.NoError(t, err)
		assert.Equal(t, []parentbilling.BookingOffering{{SchoolID: own.school, CareOfferingID: 101,
			ModuleNames: []string{"Betreuung bis 14:00 Uhr"}}}, offerings, "the Angebote of this Träger's prices")

		assert.ErrorIs(t, store.DeleteCostCenter(ctx, own.organization, foreignCenter), parentbilling.ErrCostCenterNotFound)
		var kept int
		require.NoError(t, tx.NewRaw(`SELECT COUNT(*) FROM parent_billing.cost_centers WHERE id = ?`, foreignCenter).Scan(ctx, &kept))
		assert.Equal(t, 1, kept, "the other Träger's cost center stays")
		assert.NoError(t, store.DeleteRevenueAccount(ctx, own.organization, parentbilling.BookingScopeOffering, 101))
		require.NoError(t, tx.NewRaw(`SELECT COUNT(*) FROM parent_billing.revenue_accounts WHERE organization_id = ?`, other.organization).Scan(ctx, &kept))
		assert.Equal(t, 2, kept, "removing this Träger's account leaves the other's")

		// A position and a Mensa month of the other Träger on this Träger's
		// invoice; the keys would refuse them, replica mode lets them in.
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_invoice_positions (organization_id, invoice_id, position,
			care_offering_id, module_name, contract_version_number, service_from, service_until, weekdays, amount_cents, net_cents,
			revenue_account, accounts_frozen)
			SELECT ?, invoice.id, 9, 101, 'Fremd', 1, DATE '2029-09-01', DATE '2029-09-30',
				'[{"weekday":"mon","monthly_cents":999,"amount_cents":999}]'::jsonb, 999, 999, '4999', true
			FROM parent_billing.care_invoices AS invoice WHERE invoice.organization_id = ? AND invoice.run_id = ?`,
			other.organization, own.organization, own.released)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_meal_positions (organization_id, invoice_id, position, school_id,
			student_id, meal_month, mode, quantity, unit_cents, amount_cents, net_cents, revenue_account, accounts_frozen)
			SELECT ?, invoice.id, 9, ?, 1, DATE '2029-09-01', 'flat', 1, 999, 999, 999, '4999', true
			FROM parent_billing.care_invoices AS invoice WHERE invoice.organization_id = ? AND invoice.run_id = ?`,
			other.organization, own.school, own.organization, own.released)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = origin`)
		require.NoError(t, err)

		content, err := store.ExportRunContent(ctx, own.organization, parentbilling.DebitRunCare, own.released)
		require.NoError(t, err)
		require.Len(t, content.Invoices, 1)
		assert.Equal(t, []parentbilling.ExportBookingPart{{Kind: parentbilling.BookingPartCare, SchoolID: own.school, CareOfferingID: 101,
			Day: calendar.NewDate(2029, time.September, 1), AmountCents: 3200}}, content.Invoices[0].Parts,
			"the position of this Träger, issued before #4821, without the other Träger's")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// Invariant 1: an issued position keeps the accounts its release copied.
// A later change of the Träger's accounts reaches no issued position and no
// file of it, and no role can change the position.
func TestIssuedPositionsKeepWhereTheyBook(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := newCareScope(t, db, "Träger Festschreibung")
	store := NewStore()
	rollback := errors.New("roll the release back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, scope.organization)
		require.NoError(t, err)
		run, _ := seedHolidayExportRun(t, ctx, tx, scope.organization, "Festschreibung", 4500, false)

		// The release copies what it resolved; the period and registration
		// of the run play no part, replica mode skips their keys.
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
		require.NoError(t, err)
		issuedOn := calendar.NewDate(2029, time.September, 21)
		sequence := testpkg.UniqueTestTenantID(t)
		document, err := store.IssueDocument(ctx, scope.organization, parentbilling.BillingRun{ID: run, Service: parentbilling.ServiceHoliday,
			HolidayPeriodID: 1}, 2029, sequence, parentbilling.BillingDocument{
			Number: fmt.Sprintf("FE-K-%d", sequence), IssuedOn: &issuedOn, RegistrationID: sequence, RecipientFirstName: "Eva",
			RecipientLastName: "Beispiel", ServiceFrom: calendar.NewDate(2029, time.October, 1), ServiceUntil: calendar.NewDate(2029, time.October, 5),
			TotalCents: 4500,
			Accounts:   parentbilling.PositionAccounts{RevenueAccount: "4420", CostCenter: "FER", Frozen: true},
			Children: []parentbilling.BillingChild{{ChildID: 1, FirstName: "Mia", LastName: "Beispiel", Positions: []parentbilling.BillingPosition{
				{Kind: parentbilling.LineBase, AmountCents: 5000}, {Kind: parentbilling.LineSibling, AmountCents: -500}}}},
		})
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = origin`)
		require.NoError(t, err)

		// The Träger changes its accounts after the release.
		require.NoError(t, store.SetRevenueAccount(ctx, scope.organization, 0, parentbilling.BookingScopeHoliday, 0, "4999"))
		require.NoError(t, store.InsertCostCenter(ctx, scope.organization, 0, parentbilling.CostCenter{
			Scope: parentbilling.BookingScopeHoliday, ValidFrom: calendar.NewDate(2029, time.January, 1), Code: "NEU"}))

		content, err := store.ExportRunContent(ctx, scope.organization, parentbilling.DebitRunHoliday, run)
		require.NoError(t, err)
		var parts []parentbilling.ExportBookingPart
		for _, invoice := range content.Invoices {
			if invoice.DocumentID == document {
				parts = invoice.Parts
			}
		}
		require.Len(t, parts, 2)
		for _, part := range parts {
			assert.Equal(t, parentbilling.PositionAccounts{RevenueAccount: "4420", CostCenter: "FER", Frozen: true}, part.Accounts,
				"the position books where its release put it")
		}
		documents, err := store.ReferencingDocuments(ctx, scope.organization, nil)
		require.NoError(t, err)
		assert.Empty(t, documents)

		// No role changes an issued position, also not below RLS.
		_, err = tx.ExecContext(ctx, `SAVEPOINT change_position`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `UPDATE parent_billing.document_positions SET revenue_account = '4999', cost_center = 'NEU'
			WHERE organization_id = ? AND document_id = ?`, scope.organization, document)
		require.Error(t, err, "the trigger keeps the issued position")
		assert.Contains(t, err.Error(), "never changes")
		_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT change_position`)
		require.NoError(t, err)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
