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

// Each holiday position keeps the KOST1 of its host school or its fallback
// with the reason (#6624): the release writes them per position, the export
// reads them per position, and the protocol keeps the positions a file
// booked on the fallback. No role changes the reason afterwards.
func TestHolidayPositionsKeepTheirKost1AndFallback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := newCareScope(t, db, "Träger Gastgeber-KOST1")
	store := NewStore()
	rollback := errors.New("roll the release back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, scope.organization)
		require.NoError(t, err)
		run, _ := seedHolidayExportRun(t, ctx, tx, scope.organization, "Gastgeber", 8000, false)

		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
		require.NoError(t, err)
		issuedOn := calendar.NewDate(2029, time.September, 21)
		first, second := calendar.NewDate(2029, time.October, 1), calendar.NewDate(2029, time.October, 8)
		firstEnd, secondEnd := first.AddDays(4), second.AddDays(4)
		isp := parentbilling.PositionAccounts{RevenueAccount: "4420", CostCenter: "ISP", Frozen: true}
		fer := parentbilling.PositionAccounts{RevenueAccount: "4420", CostCenter: "FER", Frozen: true}
		sequence := testpkg.UniqueTestTenantID(t)
		number := fmt.Sprintf("FE-G-%d", sequence)
		document, err := store.IssueDocument(ctx, scope.organization, parentbilling.BillingRun{ID: run, Service: parentbilling.ServiceHoliday,
			HolidayPeriodID: 1}, 2029, sequence, parentbilling.BillingDocument{
			Number: number, IssuedOn: &issuedOn, RegistrationID: sequence, RecipientFirstName: "Eva", RecipientLastName: "Beispiel",
			ServiceFrom: first, ServiceUntil: secondEnd, TotalCents: 8000,
			Children: []parentbilling.BillingChild{{ChildID: 1, FirstName: "Mia", LastName: "Beispiel", Positions: []parentbilling.BillingPosition{
				{Kind: parentbilling.LineBase, AmountCents: 5000, WeekStartsOn: &first, WeekEndsOn: &firstEnd, Accounts: isp},
				{Kind: parentbilling.LineBase, AmountCents: 3000, WeekStartsOn: &second, WeekEndsOn: &secondEnd, Accounts: fer,
					CostCenterFallback: parentbilling.CostCenterFallbackNoSchoolCostCenter}}}},
		})
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = origin`)
		require.NoError(t, err)

		// The issued invoice reads back with each position's values; a
		// Storno built of it copies them.
		issued, err := store.RunDocuments(ctx, scope.organization, run)
		require.NoError(t, err)
		var positions []parentbilling.BillingPosition
		for _, document := range issued {
			if document.Number == number {
				positions = document.Children[0].Positions
			}
		}
		require.Len(t, positions, 2)
		assert.Equal(t, isp, positions[0].Accounts)
		assert.Empty(t, positions[0].CostCenterFallback)
		assert.Equal(t, fer, positions[1].Accounts)
		assert.Equal(t, parentbilling.CostCenterFallbackNoSchoolCostCenter, positions[1].CostCenterFallback)

		content, err := store.ExportRunContent(ctx, scope.organization, parentbilling.DebitRunHoliday, run)
		require.NoError(t, err)
		var parts []parentbilling.ExportBookingPart
		for _, invoice := range content.Invoices {
			if invoice.DocumentID == document {
				parts = invoice.Parts
			}
		}
		require.Len(t, parts, 2)
		assert.Equal(t, "ISP", parts[0].Accounts.CostCenter)
		assert.Equal(t, "FER", parts[1].Accounts.CostCenter)
		assert.Equal(t, parentbilling.CostCenterFallbackNoSchoolCostCenter, parts[1].CostCenterFallback)
		require.NotNil(t, parts[1].WeekStartsOn)
		assert.Equal(t, second, *parts[1].WeekStartsOn)

		// The protocol keeps the positions on the fallback.
		fallback := parentbilling.CostCenterFallback{Number: number, WeekStartsOn: &second, WeekEndsOn: &secondEnd,
			Reason: parentbilling.CostCenterFallbackNoSchoolCostCenter, CostCenter: "FER"}
		fileNumber, err := store.ReserveExportFileNumber(ctx, scope.organization)
		require.NoError(t, err)
		entry, err := store.InsertExportFile(ctx, scope.organization, 0, parentbilling.ExportFileRecord{Format: parentbilling.ExportFormatDatev,
			RunKind: parentbilling.DebitRunHoliday, RunID: run, FileNumber: fileNumber, FileName: "EXTF.csv", Content: []byte("datei"),
			TotalCents: 8000, DocumentIDs: []int64{document}, CostCenterFallbacks: []parentbilling.CostCenterFallback{fallback}})
		require.NoError(t, err)
		assert.Equal(t, []parentbilling.CostCenterFallback{fallback}, entry.CostCenterFallbacks)
		page, err := store.ExportFiles(ctx, scope.organization, parentbilling.DebitRunHoliday, run, 0, 10)
		require.NoError(t, err)
		require.Len(t, page.Files, 1)
		assert.Equal(t, []parentbilling.CostCenterFallback{fallback}, page.Files[0].CostCenterFallbacks)

		// No role changes the reason of an issued position.
		_, err = tx.ExecContext(ctx, `SAVEPOINT change_fallback`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `UPDATE parent_billing.document_positions SET cost_center_fallback = NULL, cost_center = 'BIR'
			WHERE organization_id = ? AND document_id = ?`, scope.organization, document)
		require.Error(t, err, "the trigger keeps the issued position")
		_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT change_fallback`)
		require.NoError(t, err)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
