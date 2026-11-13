package compose

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// visitorTraceTable is the first table a count of visitorTraceCounts reads.
var visitorTraceTable = regexp.MustCompile(`(?i)\bFROM\s+([a-z_]+\.[a-z_]+)`)

// rowsHoldingTheVisitor counts, per table of the schema, the rows whose text
// holds the visitor's last name: every column, JSON included. The database
// holds one school's traces only.
func rowsHoldingTheVisitor(t *testing.T, db *bun.DB) map[string]int {
	t.Helper()
	ctx := t.Context()
	var tables []string
	require.NoError(t, db.NewRaw(`
		SELECT format('%I.%I', n.nspname, c.relname)
		FROM pg_class AS c JOIN pg_namespace AS n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p') AND NOT c.relispartition
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')`).Scan(ctx, &tables))
	require.NotEmpty(t, tables, "schema discovery must not pass vacuously")
	rows := map[string]int{}
	for _, table := range tables {
		var count int
		require.NoError(t, db.NewRaw(fmt.Sprintf(`SELECT count(*) FROM %s AS row WHERE row::text LIKE ?`, table),
			"%"+visitorLast+"%").Scan(ctx, &count), table)
		if count > 0 {
			rows[table] = count
		}
	}
	for table, count := range visitorClosureIdentityTraces(t, db) {
		if count > 0 {
			rows[table] += count
		}
	}

	return rows
}

// visitorTraceCounts is no free list: every row leaveVisitorTraces leaves
// with the visitor's name is counted by one of its entries. Removing an entry,
// or a new place in leaveVisitorTraces without one, fails here, so the
// erasure tests cannot lose a place silently (#6015).
func TestVisitorTraceCountsCountEveryRowHoldingTheVisitor(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	accounts := leaveVisitorTraces(t, db, schoolID)

	counted := map[string]int{}
	for place, query := range visitorTraceCounts {
		match := visitorTraceTable.FindStringSubmatch(query)
		require.NotNil(t, match, "the count of %s reads no table", place)
		var count int
		require.NoError(t, db.NewRaw(query, schoolID).Scan(t.Context(), &count), place)
		require.Equal(t, 1, count, "leaveVisitorTraces leaves one row for %s", place)
		counted[match[1]] += count
	}
	held := rowsHoldingTheVisitor(t, db)
	require.NotEmpty(t, held, "the visitor's name must be found somewhere; the scan would pass vacuously")
	require.Equal(t, held, counted,
		"rows holding the visitor's name per table (left) against the rows visitorTraceCounts counts (right): give every place a count")

	// The erasure leaves the name in no row of the database.
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	slug := orderDemoSchool(t, db, "OGS Nord")
	_, err = queue.ClaimDemoSchoolOrder(t.Context())
	require.NoError(t, err)
	require.NoError(t, schools.RememberDemoSchool(t.Context(), slug, organizationtenancy.DemoSchoolState{SchoolID: schoolID, SeedJSON: []byte(`{}`)}))
	require.NoError(t, queue.FinishDemoSchoolOrder(t.Context(), slug, accounts.caregiver, accounts.parent))
	_, erased := retireAtLeast(t, db, queue, []string{slug}, 0, 1)
	require.Equal(t, 1, erased)
	require.Empty(t, rowsHoldingTheVisitor(t, db), "rows still holding the visitor's name after the erasure")
}

func leaveVisitorClosureAuthorization(t *testing.T, db *bun.DB, schoolID, accountID int64) {
	t.Helper()
	// The trace predates retirement; restore the fixture's exact tombstone.
	var deletedAt *time.Time
	require.NoError(t, db.NewRaw(`SELECT deleted_at FROM platform.schools WHERE id = ?`, schoolID).Scan(t.Context(), &deletedAt))
	if deletedAt != nil {
		_, err := db.NewRaw(`UPDATE platform.schools SET deleted_at = NULL WHERE id = ?`, schoolID).Exec(t.Context())
		require.NoError(t, err)
		defer func() {
			_, err := db.NewRaw(`UPDATE platform.schools SET deleted_at = ? WHERE id = ?`, deletedAt, schoolID).Exec(t.Context())
			require.NoError(t, err)
		}()
	}
	var organizationID *int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, schoolID).Scan(t.Context(), &organizationID))
	if organizationID == nil {
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
			Name: "Träger Freigabe", Slug: fmt.Sprintf("freigabe-%d", schoolID),
		})
		organizationID = &organization.ID
		_, err := db.NewRaw(`UPDATE platform.schools SET organization_id = ? WHERE id = ?`, organizationID, schoolID).Exec(t.Context())
		require.NoError(t, err)
		defer func() {
			_, err := db.NewRaw(`UPDATE platform.schools SET organization_id = NULL WHERE id = ?`, schoolID).Exec(t.Context())
			require.NoError(t, err)
		}()
	}
	_, err := db.NewRaw(`WITH source AS (
		INSERT INTO schedule.closing_days (tenant_id, start_date, end_date, reason)
		VALUES (?0, DATE '2030-07-08', DATE '2030-07-08', 'Fortbildung') RETURNING id
	), proposal AS (
		INSERT INTO calendar.holiday_closure_proposals (tenant_id, organization_id, source_kind, source_id,
			operation_id, reason, current_start, current_end, span_from, span_to, revision, state, confirmed_at)
		SELECT ?0, ?1, 'school_closing_day', id, gen_random_uuid(), 'Fortbildung',
			DATE '2030-07-08', DATE '2030-07-08', DATE '2030-07-08', DATE '2030-07-08', 1, 'confirmed', NOW()
		FROM source RETURNING *
	), event AS (
		INSERT INTO calendar.holiday_closure_events (tenant_id, organization_id, proposal_id, operation_id,
			source_kind, source_id, revision, span_from, span_to, confirmed_channel)
		SELECT tenant_id, organization_id, id, operation_id, source_kind, source_id, revision, span_from, span_to, 'school'
		FROM proposal RETURNING *
	)
	INSERT INTO calendar.holiday_closure_authorizations (tenant_id, event_id, operation_id, revision, account_id, channel)
	SELECT tenant_id, id, operation_id, revision, ?2, 'school' FROM event`, schoolID, organizationID, accountID).Exec(t.Context())
	require.NoError(t, err)
}

// Approval history identifies its actor by foreign key, without copying names.
func visitorClosureIdentityTraces(t *testing.T, db *bun.DB) map[string]int {
	t.Helper()
	var counts struct {
		Authorizations int
		Events         int
		Proposals      int
	}
	require.NoError(t, db.NewRaw(`SELECT count(DISTINCT entry.id) AS authorizations,
		count(DISTINCT event.id) AS events, count(DISTINCT event.proposal_id) AS proposals
		FROM calendar.holiday_closure_authorizations AS entry
		JOIN calendar.holiday_closure_events AS event ON event.tenant_id = entry.tenant_id AND event.id = entry.event_id
		JOIN users.persons AS person ON person.account_id = entry.account_id
		WHERE person.first_name = ? AND person.last_name = ?`, visitorFirst, visitorLast).Scan(t.Context(), &counts))
	return map[string]int{
		"calendar.holiday_closure_authorizations": counts.Authorizations,
		"calendar.holiday_closure_events":         counts.Events,
		"calendar.holiday_closure_proposals":      counts.Proposals,
	}
}
