// Router-level test for the deletion of the holiday registrations three
// months after their period (#4667, spec #4611 user story 121, ADR 0118):
// with a test clock the hourly retention keeps everything on the 92nd day
// after the last week and deletes the registrations, children, guests,
// contacts, bookings, contributions and the trail on the 93rd, logs the
// deletion for the Träger and leaves the issued invoices, their positions
// and letters untouched.
package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type holidayRegistrationDeletionEntry struct {
	HolidayPeriodID int64         `bun:"holiday_period_id"`
	PeriodEndedOn   calendar.Date `bun:"period_ended_on"`
	Registrations   int           `bun:"registrations"`
	Children        int           `bun:"children"`
	AuditEntries    int           `bun:"audit_entries"`
}

// TestHolidayRegistrationsGoNinetyTwoDaysAfterTheirPeriodAndInvoicesStay runs
// on its own database: the retention deletes across every Träger, and the
// test clock lies after the periods of the other tests.
func TestHolidayRegistrationsGoNinetyTwoDaysAfterTheirPeriodAndInvoicesStay(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	f := newReviewFixture(t, "Träger Löschfrist Rechnung")
	ctx := context.Background()
	organization := f.office.organization
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)

	// Both families confirmed and invoiced with a correction and the letters.
	for _, id := range []int64{f.siblings, f.again} {
		require.Equal(t, http.StatusOK, f.act(t, id, "confirm").Code)
	}
	f.list(t, "")
	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	var lia int64
	for _, document := range run.Documents {
		if document.RegistrationID == f.siblings {
			lia = document.Children[0].ChildID
		}
	}
	require.NotZero(t, lia)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/corrections"), map[string]any{
		"registration_id": f.siblings, "child_id": lia, "total_cents": 3000, "reason": "Erste Woche krank, halber Beitrag",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	run = billingRun(t, f, billing, run.ID)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	brandTraeger(t, f)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/letters"), lettersBody("2099-01-15"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	invoices := func() []string {
		snapshot := make([]string, 5)
		require.NoError(t, f.db.NewRaw(`SELECT
				(SELECT jsonb_agg(document ORDER BY document.id)::text FROM parent_billing.documents AS document
					WHERE document.organization_id = ?0),
				(SELECT jsonb_agg(line ORDER BY line.id)::text FROM parent_billing.document_positions AS line
					WHERE line.organization_id = ?0),
				(SELECT jsonb_agg(letter.sha256 ORDER BY letter.document_id)::text FROM parent_billing.document_letters AS letter
					WHERE letter.organization_id = ?0),
				(SELECT jsonb_agg(letters ORDER BY letters.run_id)::text FROM parent_billing.run_letters AS letters
					WHERE letters.organization_id = ?0),
				(SELECT jsonb_agg(event ORDER BY event.id)::text FROM parent_billing.run_events AS event
					WHERE event.organization_id = ?0)`, organization).
			Scan(ctx, &snapshot[0], &snapshot[1], &snapshot[2], &snapshot[3], &snapshot[4]))
		return snapshot
	}
	issued := invoices()
	require.Len(t, issued, 5)
	for _, part := range issued {
		require.NotEmpty(t, part, "the run issued invoices with positions and letters")
	}
	assert.Contains(t, issued[1], `"kind": "correction"`)

	personal := func() map[string]int {
		counts := map[string]int{}
		for _, table := range []string{
			"enrollment.holiday_registrations", "enrollment.holiday_registration_children",
			"enrollment.holiday_registration_contacts", "enrollment.holiday_registration_bookings",
			"enrollment.holiday_registration_audit", "parent_billing.holiday_contributions",
			"parent_billing.holiday_contribution_lines", "parent_billing.run_corrections",
		} {
			var rows int
			require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM `+table+` WHERE organization_id = ?`, organization).Scan(ctx, &rows))
			counts[table] = rows
		}
		return counts
	}
	before := personal()
	for table, rows := range before {
		require.NotZero(t, rows, "%s holds data before the deadline", table)
	}
	deletions := func() []holidayRegistrationDeletionEntry {
		var entries []holidayRegistrationDeletionEntry
		require.NoError(t, f.db.NewRaw(`SELECT holiday_period_id, period_ended_on, registrations, children, audit_entries
			FROM enrollment.holiday_registration_deletions WHERE organization_id = ? ORDER BY id`, organization).Scan(ctx, &entries))
		return entries
	}
	retain := func(today calendar.Date) {
		t.Helper()
		unit := tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, f.db))
		require.NoError(t, tenant.WithinAdmin(unit, func(adminCtx context.Context) error {
			_, err := enrollmentCompose.NewHolidayRegistrationRetention(func() calendar.Date { return today }).
				DeleteExpiredHolidayRegistrations(adminCtx)
			return err
		}))
	}

	// The last week ends on 15.01.2027; the 92nd day after it is 17.04.2027.
	lastDay := calendar.NewDate(2027, time.January, 15)
	retain(lastDay.AddDays(holidayRetentionDays))
	assert.Equal(t, before, personal(), "on the 92nd day after the period everything is still there")
	assert.Empty(t, deletions(), "nothing deleted, nothing logged")

	retain(lastDay.AddDays(holidayRetentionDays + 1))
	for table, rows := range personal() {
		assert.Zero(t, rows, "%s is gone on the 93rd day", table)
	}
	assert.Equal(t, []holidayRegistrationDeletionEntry{{
		HolidayPeriodID: f.period.ID, PeriodEndedOn: lastDay,
		Registrations: 2, Children: 3, AuditEntries: before["enrollment.holiday_registration_audit"],
	}}, deletions(), "the deletion stands in the log of the Träger with its counts")
	assert.Equal(t, issued, invoices(), "the issued invoices, positions, letters and the trail of the run stay unchanged")

	retain(lastDay.AddDays(holidayRetentionDays + 2))
	assert.Len(t, deletions(), 1, "a period with nothing left writes no further entry")

	// The log keeps an entry for three years.
	_, err := f.db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_deletions
			(organization_id, holiday_period_id, period_ended_on, registrations, children, audit_entries, deleted_at)
		VALUES (?0, ?1, '2022-01-14', 1, 1, 0, NOW() - INTERVAL '3 years 1 day'),
			(?0, ?1, '2024-01-12', 1, 1, 0, NOW() - INTERVAL '2 years 11 months')`, organization, f.period.ID)
	require.NoError(t, err)
	retain(lastDay.AddDays(holidayRetentionDays + 3))
	var kept []calendar.Date
	for _, entry := range deletions() {
		kept = append(kept, entry.PeriodEndedOn)
	}
	assert.ElementsMatch(t, []calendar.Date{lastDay, calendar.NewDate(2024, time.January, 12)}, kept,
		"an entry older than three years goes")
}

// holidayRetentionDays is the window of user story 121: three months after
// the last week of the period.
const holidayRetentionDays = 92
