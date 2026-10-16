package compose

import (
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The visitor name orderDemoSchoolWithin queues every order with.
const (
	visitorFirst = "Anna Lena"
	visitorLast  = "von Berg"
	visitorFull  = visitorFirst + " " + visitorLast
)

// visitorTraceCounts are the places the seed of a demo school copies the
// visitor's name to (#4048), as SQL counting the rows of the school ?0 that
// still hold it. The signed declarations of the parent are a seventh place;
// they need a whole declaration of Communication, so the seed-smoke job
// covers them with a real seed.
var visitorTraceCounts = map[string]string{
	"users.persons": `SELECT count(*) FROM users.persons
		WHERE tenant_id = ?0 AND first_name = '` + visitorFirst + `' AND last_name = '` + visitorLast + `'`,
	"users.guardian_profiles": `SELECT count(*) FROM users.guardian_profiles
		WHERE tenant_id = ?0 AND first_name = '` + visitorFirst + `' AND last_name = '` + visitorLast + `'`,
	"enrollment.requests": `SELECT count(*) FROM enrollment.requests
		WHERE tenant_id = ?0 AND guardian_first_name = '` + visitorFirst + `' AND guardian_last_name = '` + visitorLast + `'`,
	"audit.guardian_changes": `SELECT count(*) FROM audit.guardian_changes
		WHERE tenant_id = ?0 AND actor_name_snapshot = '` + visitorFull + `'`,
	"users.parent_messages": `SELECT count(*) FROM users.parent_messages
		WHERE tenant_id = ?0 AND sender_name = '` + visitorFull + `'`,
	"platform.email_outbox": `SELECT count(*) FROM platform.email_outbox
		WHERE tenant_id = ?0 AND payload->>'last_name' = '` + visitorLast + `'`,
}

// leaveVisitorTraces writes one row per place of visitorTraceCounts into the
// school, the way its seed leaves them: the visitor is a caregiver and the
// parent of a child, wrote a message and changed the child's guardians.
func leaveVisitorTraces(t *testing.T, db *bun.DB, schoolID int64) {
	t.Helper()
	ctx := t.Context()
	testpkg.CreateTestPersonForTenant(t, db, schoolID, visitorFirst, visitorLast)
	student := testpkg.CreateTestStudentForTenant(t, db, schoolID, "Mia", "Schneider", "2a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, schoolID, visitorFirst, visitorLast, fmt.Sprintf("eltern-%d@example.org", schoolID))
	parent := testpkg.CreateTestAccount(t, db, fmt.Sprintf("eltern-konto-%d@example.org", schoolID))
	for _, statement := range []string{
		`WITH phase AS (
			INSERT INTO enrollment.phases (tenant_id, name, service_start_date, service_end_date)
			VALUES (?0, 'Anmeldung', DATE '2026-08-01', DATE '2027-07-31') RETURNING id)
		INSERT INTO enrollment.requests (tenant_id, phase_id, guardian_first_name, guardian_last_name, guardian_email, status_token)
		SELECT ?0, id, '` + visitorFirst + `', '` + visitorLast + `', 'eltern@example.org', 'token-' || ?0 FROM phase`,
		`INSERT INTO audit.guardian_changes (tenant_id, student_id, guardian_profile_id, change_type, field_name, actor_name_snapshot)
			VALUES (?0, ?1, ?2, 'updated', 'phone', '` + visitorFull + `')`,
		`WITH thread AS (
			INSERT INTO users.parent_message_threads (tenant_id, student_id, guardian_account_id) VALUES (?0, ?1, ?3) RETURNING id)
		INSERT INTO users.parent_messages (tenant_id, thread_id, student_id, sender_account_id, sender_kind, sender_name, body)
		SELECT ?0, id, ?1, ?3, 'guardian', '` + visitorFull + `', 'Hallo' FROM thread`,
		`INSERT INTO platform.email_outbox (tenant_id, kind, recipient, payload, status)
			VALUES (?0, 'parent_message', '{}'::jsonb,
				jsonb_build_object('first_name', '` + visitorFirst + `', 'last_name', '` + visitorLast + `'), 'sent')`,
	} {
		_, err := db.NewRaw(statement, schoolID, student.ID, guardian.ID, parent.ID).Exec(ctx)
		require.NoError(t, err, statement)
	}
}

// visitorTraces counts, per place, the rows of the school that still hold
// the visitor's name.
func visitorTraces(t *testing.T, db *bun.DB, schoolID int64) map[string]int {
	t.Helper()
	traces := make(map[string]int, len(visitorTraceCounts))
	for table, query := range visitorTraceCounts {
		var count int
		require.NoError(t, db.NewRaw(query, schoolID).Scan(t.Context(), &count), table)
		traces[table] = count
	}
	return traces
}

func everyTrace(count int) map[string]int {
	traces := make(map[string]int, len(visitorTraceCounts))
	for table := range visitorTraceCounts {
		traces[table] = count
	}
	return traces
}

// schoolOfOrganization adds a school to the organization of another one: a
// demo school next to the standing one under the Demo-Träger.
func schoolOfOrganization(t *testing.T, db *bun.DB, siblingID int64, deleted bool) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.schools (organization_id, name, slug, subdomain, deleted_at)
		SELECT organization_id, 'Neben ' || id, 'neben-' || id || '-' || nextval('platform.schools_id_seq'), 'n' || currval('platform.schools_id_seq'),
			CASE WHEN ? THEN NOW() END
		FROM platform.schools WHERE id = ? RETURNING id`, deleted, siblingID).Scan(t.Context(), &id))
	return id
}

func demoOrderNames(t *testing.T, db *bun.DB) []string {
	t.Helper()
	var names []string
	require.NoError(t, db.NewRaw(`SELECT name FROM platform.demo_school_states ORDER BY name`).Scan(t.Context(), &names))
	return names
}

// A retired or failed order loses the visitor once no demo access names it
// (#4048): its state row with name, account ids and credentials goes, and
// the visitor's name leaves every hidden school of the Demo-Träger,
// including a school an earlier attempt abandoned. Schools still open and
// schools of other organizations keep their rows, and an order still
// waiting keeps its name.
func TestDemoSchoolQueueErasesTheVisitorsOfRetiredAndFailedOrders(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	retiredID, _ := testpkg.CreateTestTenant(t, db)
	abandonedID := schoolOfOrganization(t, db, retiredID, true)
	openID := schoolOfOrganization(t, db, retiredID, false)
	foreignID, _ := testpkg.CreateTestTenant(t, db)
	_, err := db.NewRaw(`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, foreignID).Exec(t.Context())
	require.NoError(t, err)
	for _, schoolID := range []int64{retiredID, abandonedID, openID, foreignID} {
		leaveVisitorTraces(t, db, schoolID)
	}
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	ctx := t.Context()

	retired := orderDemoSchool(t, db, "OGS Nord")
	_, err = queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	require.NoError(t, schools.RememberDemoSchool(ctx, retired, organizationtenancy.DemoSchoolState{SchoolID: retiredID, SeedJSON: []byte(`{}`)}))
	require.NoError(t, queue.FinishDemoSchoolOrder(ctx, retired, 11, 12))
	failed := orderDemoSchool(t, db, "OGS Ost")
	_, err = queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	closed, err := queue.FailDemoSchoolOrder(ctx, failed, 1)
	require.NoError(t, err)
	require.True(t, closed)
	waiting := orderDemoSchool(t, db, "OGS Süd")
	// The visitor of the failed order still has an access that shows the failure.
	_, err = db.NewRaw(`INSERT INTO auth.demo_accesses (email, school_name, token_hash, expires_at, school_slug, first_name, last_name)
		VALUES ('besuch@example.org', 'OGS Ost', 'hash-' || ?0, NOW() + INTERVAL '14 days', ?0, ?1, ?2)`,
		failed, visitorFirst, visitorLast).Exec(ctx)
	require.NoError(t, err)

	// An open school and a failure its visitor can still look at keep the
	// visitor.
	hidden, erased, err := queue.RetireDemoSchools(ctx, nil)
	require.NoError(t, err)
	assert.Zero(t, hidden)
	assert.Zero(t, erased)
	assert.ElementsMatch(t, []string{retired, failed, waiting}, demoOrderNames(t, db))
	assert.Equal(t, everyTrace(1), visitorTraces(t, db, retiredID), "the school is still open")
	assert.Equal(t, everyTrace(1), visitorTraces(t, db, abandonedID), "no order is out of use yet")

	// The expiry deletes the access and hides the school of the other order.
	_, err = db.NewRaw(`DELETE FROM auth.demo_accesses WHERE school_slug = ?`, failed).Exec(ctx)
	require.NoError(t, err)
	hidden, erased, err = queue.RetireDemoSchools(ctx, []string{retired})
	require.NoError(t, err)
	assert.Equal(t, 1, hidden)
	assert.Equal(t, 2, erased, "the school hidden right now and the failed order")
	assert.Equal(t, []string{waiting}, demoOrderNames(t, db))

	assert.Equal(t, everyTrace(0), visitorTraces(t, db, retiredID), "the retired school")
	assert.Equal(t, everyTrace(0), visitorTraces(t, db, abandonedID), "the school an earlier attempt abandoned")
	assert.Equal(t, everyTrace(1), visitorTraces(t, db, openID), "a school still open")
	assert.Equal(t, everyTrace(1), visitorTraces(t, db, foreignID), "a hidden school outside the Demo-Träger")

	// The people stay, under a placeholder; the mails go.
	var placeholders int
	require.NoError(t, db.NewRaw(`SELECT
			(SELECT count(*) FROM users.persons WHERE tenant_id = ?0 AND first_name = 'Demo' AND last_name = 'Besuch')
			+ (SELECT count(*) FROM users.guardian_profiles WHERE tenant_id = ?0 AND first_name = 'Demo' AND last_name = 'Besuch')
			+ (SELECT count(*) FROM enrollment.requests WHERE tenant_id = ?0 AND guardian_first_name = 'Demo' AND guardian_last_name = 'Besuch')
			+ (SELECT count(*) FROM audit.guardian_changes WHERE tenant_id = ?0 AND actor_name_snapshot = 'Demo Besuch')
			+ (SELECT count(*) FROM users.parent_messages WHERE tenant_id = ?0 AND sender_name = 'Demo Besuch')`,
		retiredID).Scan(ctx, &placeholders))
	assert.Equal(t, 5, placeholders)

	_, erased, err = queue.RetireDemoSchools(ctx, nil)
	require.NoError(t, err)
	assert.Zero(t, erased, "a second run finds nothing")
}
