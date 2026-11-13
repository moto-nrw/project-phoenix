// Router-level test for the deletion of the holiday registrations after
// their retention (#4667, #6691, spec #4611 user story 121, ADR 0118): each
// registration keeps the days its privacy text names. With a test clock the
// hourly retention keeps everything on the 92nd day after the last week. On
// the 93rd it deletes the registration that agreed to three months and the
// health information of the one that agreed to a year; that one goes on the
// 366th day with children, guests, contacts, bookings, contributions and the
// trail. Every run logs what went for the Träger and leaves the issued
// invoices, their positions and letters untouched.
package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

type holidayRegistrationDeletionEntry struct {
	HolidayPeriodID int64         `bun:"holiday_period_id"`
	PeriodEndedOn   calendar.Date `bun:"period_ended_on"`
	Registrations   int           `bun:"registrations"`
	Children        int           `bun:"children"`
	AuditEntries    int           `bun:"audit_entries"`
	HealthRecords   int           `bun:"health_records"`
}

// TestHolidayRegistrationsGoAfterTheirRetentionAndInvoicesStay runs
// on its own database: the retention deletes across every Träger, and the
// test clock lies after the periods of the other tests.
func TestHolidayRegistrationsGoAfterTheirRetentionAndInvoicesStay(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	f := newReviewFixture(t, "Träger Löschfrist Rechnung")
	ctx := context.Background()
	organization := f.office.organization
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	_, err := f.db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET guardian_address_street = 'Musterweg 12',
		guardian_address_postal_code = '01234', guardian_address_city = 'Bissendorf' WHERE id = ?`, f.siblings)
	require.NoError(t, err)

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
	// The correction of the run goes with the registration; the invoice
	// keeps its own copy of amount and reason (#4720).
	assert.Contains(t, issued[1], `"note": "Erste Woche krank, halber Beitrag"`, "the invoice keeps the reason of its correction")
	assert.Contains(t, issued[0], `"recipient_address_street": "Musterweg 12"`, "the invoice copied the address (#5625)")

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
		require.NoError(t, f.db.NewRaw(`SELECT holiday_period_id, period_ended_on, registrations, children, audit_entries, health_records
			FROM enrollment.holiday_registration_deletions WHERE organization_id = ? ORDER BY id`, organization).Scan(ctx, &entries))
		return entries
	}
	retain := func(today calendar.Date) {
		t.Helper()
		unit := tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, f.db))
		require.NoError(t, tenant.WithinAdmin(unit, func(adminCtx context.Context) error {
			_, err := owners.NewHolidayRegistrationRetention(func() calendar.Date { return today }).
				DeleteExpiredHolidayRegistrations(adminCtx)
			return err
		}))
	}

	// Both registrations agreed to a year. The second family is turned into
	// one from before #6691: version 3 of the texts with its three months.
	var agreed []int
	require.NoError(t, f.db.NewRaw(`SELECT retention_days FROM enrollment.holiday_registrations
		WHERE organization_id = ? ORDER BY id`, organization).Scan(ctx, &agreed))
	require.Equal(t, []int{holidayDefaultRetentionDays, holidayDefaultRetentionDays}, agreed,
		"a new registration keeps the retention its privacy text names")
	_, err = f.db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET consent_version = 3, retention_days = ?
		WHERE id = ?`, holidayLegacyRetentionDays, f.again)
	require.NoError(t, err)
	count := func(query string, args ...any) int {
		t.Helper()
		var rows int
		require.NoError(t, f.db.NewRaw(query, args...).Scan(ctx, &rows))
		return rows
	}
	trailOf := func(registration int64) int {
		return count(`SELECT COUNT(*) FROM enrollment.holiday_registration_audit WHERE holiday_registration_id = ?`, registration)
	}
	health := func() int {
		return count(`SELECT (SELECT COUNT(*) FROM enrollment.holiday_registrations
				WHERE organization_id = ?0 AND health_data IS NOT NULL)
			+ (SELECT COUNT(*) FROM enrollment.holiday_registration_children
				WHERE organization_id = ?0 AND health_data IS NOT NULL)`, organization)
	}
	againTrail, siblingsTrail := trailOf(f.again), trailOf(f.siblings)
	require.NotZero(t, againTrail)
	require.Equal(t, 1, health(), "Lia's allergy is the health information of the fixture")

	// The last week ends on 15.01.2027; the 92nd day after it is 17.04.2027.
	lastDay := calendar.NewDate(2027, time.January, 15)
	retain(lastDay.AddDays(holidayLegacyRetentionDays))
	assert.Equal(t, before, personal(), "on the 92nd day after the period everything is still there")
	assert.Equal(t, 1, health(), "and so is the health information")
	assert.Empty(t, deletions(), "nothing deleted, nothing logged")

	// On the 93rd day the three months of the old text are over, and so is
	// the retention of the health information, whatever the registration
	// keeps.
	retain(lastDay.AddDays(holidayLegacyRetentionDays + 1))
	assert.Zero(t, count(`SELECT COUNT(*) FROM enrollment.holiday_registrations WHERE id = ?`, f.again),
		"the registration under the three months is gone on the 93rd day")
	assert.Zero(t, trailOf(f.again), "with its trail")
	assert.Equal(t, 1, count(`SELECT COUNT(*) FROM enrollment.holiday_registrations WHERE id = ?`, f.siblings),
		"the registration that agreed to a year stays")
	assert.Equal(t, 2, count(`SELECT COUNT(*) FROM enrollment.holiday_registration_children WHERE holiday_registration_id = ?`, f.siblings))
	assert.Equal(t, siblingsTrail, trailOf(f.siblings), "with its trail")
	assert.Zero(t, health(), "its health information went on the 93rd day")
	afterThreeMonths := personal()
	listReads := afterThreeMonths["enrollment.holiday_registration_audit"] - siblingsTrail
	assert.Positive(t, listReads, "the reads of the list stay while a registration of the period stays")
	first := holidayRegistrationDeletionEntry{
		HolidayPeriodID: f.period.ID, PeriodEndedOn: lastDay,
		Registrations: 1, Children: 1, AuditEntries: againTrail, HealthRecords: 1,
	}
	assert.Equal(t, []holidayRegistrationDeletionEntry{first}, deletions(),
		"the deletion stands in the log of the Träger with its counts")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(f.siblings, "/health"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Erdnüsse", "the office reads no health information any more")

	retain(lastDay.AddDays(holidayDefaultRetentionDays))
	afterHealthRead := personal()
	assert.Equal(t, afterThreeMonths["enrollment.holiday_registrations"], afterHealthRead["enrollment.holiday_registrations"],
		"on the 365th day the registration is still there")
	assert.Len(t, deletions(), 1, "nothing deleted, nothing logged")

	retain(lastDay.AddDays(holidayDefaultRetentionDays + 1))
	for table, rows := range personal() {
		assert.Zero(t, rows, "%s is gone on the 366th day", table)
	}
	assert.Equal(t, []holidayRegistrationDeletionEntry{first, {
		HolidayPeriodID: f.period.ID, PeriodEndedOn: lastDay,
		Registrations: 1, Children: 2, AuditEntries: afterHealthRead["enrollment.holiday_registration_audit"],
	}}, deletions(), "the second deletion stands in the log with its counts")
	assert.Equal(t, issued, invoices(), "the issued invoices, positions, letters and the trail of the run stay unchanged")

	retain(lastDay.AddDays(holidayDefaultRetentionDays + 2))
	assert.Len(t, deletions(), 2, "a period with nothing left writes no further entry")

	// The log keeps an entry for three years.
	_, err = f.db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_deletions
			(organization_id, holiday_period_id, period_ended_on, registrations, children, audit_entries, deleted_at)
		VALUES (?0, ?1, '2022-01-14', 1, 1, 0, NOW() - INTERVAL '3 years 1 day'),
			(?0, ?1, '2024-01-12', 1, 1, 0, NOW() - INTERVAL '2 years 11 months')`, organization, f.period.ID)
	require.NoError(t, err)
	retain(lastDay.AddDays(holidayDefaultRetentionDays + 3))
	var kept []calendar.Date
	for _, entry := range deletions() {
		kept = append(kept, entry.PeriodEndedOn)
	}
	assert.ElementsMatch(t, []calendar.Date{lastDay, lastDay, calendar.NewDate(2024, time.January, 12)}, kept,
		"an entry older than three years goes")
}

// holidayLegacyRetentionDays is the window of user story 121 the texts
// before #6691 promise, and the retention of the health information: three
// months after the last week of the period.
const holidayLegacyRetentionDays = 92
