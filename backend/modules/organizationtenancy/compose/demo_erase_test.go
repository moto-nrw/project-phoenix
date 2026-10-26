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

type visitorAccounts struct {
	caregiver int64
	parent    int64
}

// leaveVisitorTraces writes one row per place of visitorTraceCounts into the
// school, the way its seed leaves them: the visitor is a caregiver and the
// parent of a child, wrote a message and changed the child's guardians.
func leaveVisitorTraces(t *testing.T, db *bun.DB, schoolID int64) visitorAccounts {
	t.Helper()
	ctx := t.Context()
	caregiver := testpkg.CreateTestAccount(t, db, fmt.Sprintf("betreuer-%d@example.org", schoolID))
	person := testpkg.CreateTestPersonForTenant(t, db, schoolID, visitorFirst, visitorLast)
	student := testpkg.CreateTestStudentForTenant(t, db, schoolID, "Mia", "Schneider", "2a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, schoolID, visitorFirst, visitorLast, fmt.Sprintf("eltern-%d@example.org", schoolID))
	parent := testpkg.CreateTestAccount(t, db, fmt.Sprintf("eltern-konto-%d@example.org", schoolID))
	_, err := db.NewRaw(`UPDATE users.persons SET account_id = ? WHERE id = ?`, caregiver.ID, person.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE users.guardian_profiles SET account_id = ?, has_account = true WHERE id = ?`, parent.ID, guardian.ID).Exec(ctx)
	require.NoError(t, err)
	testpkg.CreateTestStudentGuardianLinkForTenant(t, db, schoolID, student.ID, guardian.ID, "primary_guardian")
	for _, statement := range []string{
		`WITH phase AS (
			INSERT INTO enrollment.phases (tenant_id, name, service_start_date, service_end_date)
			VALUES (?0, 'Anmeldung', DATE '2026-08-01', DATE '2027-07-31') RETURNING id)
		INSERT INTO enrollment.requests (tenant_id, phase_id, guardian_first_name, guardian_last_name, guardian_email, guardian_account_id, status_token)
		SELECT ?0, id, '` + visitorFirst + `', '` + visitorLast + `', 'eltern@example.org', ?3, 'token-' || ?0 FROM phase`,
		`INSERT INTO audit.guardian_changes (tenant_id, student_id, guardian_profile_id, change_type, field_name, actor_name_snapshot)
			VALUES (?0, ?1, ?2, 'updated', 'phone', '` + visitorFull + `')`,
		`WITH thread AS (
			INSERT INTO users.parent_message_threads (tenant_id, student_id, guardian_account_id) VALUES (?0, ?1, ?3) RETURNING id)
		INSERT INTO users.parent_messages (tenant_id, thread_id, student_id, sender_account_id, sender_kind, sender_name, body)
		SELECT ?0, id, ?1, ?3, 'guardian', '` + visitorFull + `', 'Hallo' FROM thread`,
		`WITH outbox AS (
			INSERT INTO platform.email_outbox (tenant_id, kind, recipient, payload, status)
			VALUES (?0, 'parent_message', '{}'::jsonb,
				jsonb_build_object('first_name', '` + visitorFirst + `', 'last_name', '` + visitorLast + `'), 'sent')
			RETURNING id)
		INSERT INTO platform.email_delivery (tenant_id, related_entity_type, related_entity_id, outbox_id, account_id, recipient_email)
		SELECT ?0, 'parent_message', ?1, id, ?3, 'eltern@example.org' FROM outbox`,
	} {
		_, err := db.NewRaw(statement, schoolID, student.ID, guardian.ID, parent.ID).Exec(ctx)
		require.NoError(t, err, statement)
	}
	_, err = db.NewRaw(`UPDATE audit.guardian_changes SET actor_account_id = ? WHERE tenant_id = ?`, parent.ID, schoolID).Exec(ctx)
	require.NoError(t, err)
	return visitorAccounts{caregiver: caregiver.ID, parent: parent.ID}
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
	unrelatedID := schoolOfOrganization(t, db, retiredID, true)
	openID := schoolOfOrganization(t, db, retiredID, false)
	failedID := schoolOfOrganization(t, db, retiredID, false)
	foreignID, _ := testpkg.CreateTestTenant(t, db)
	_, err := db.NewRaw(`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, foreignID).Exec(t.Context())
	require.NoError(t, err)
	accounts := make(map[int64]visitorAccounts)
	for _, schoolID := range []int64{retiredID, abandonedID, unrelatedID, openID, failedID, foreignID} {
		accounts[schoolID] = leaveVisitorTraces(t, db, schoolID)
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
	require.NoError(t, queue.FinishDemoSchoolOrder(ctx, retired, accounts[retiredID].caregiver, accounts[retiredID].parent))
	_, err = db.NewRaw(`UPDATE platform.demo_school_states
		SET visitor_account_ids = ARRAY[?], visitor_parent_account_ids = ARRAY[?], visitor_school_ids = ARRAY[?, ?] WHERE name = ?`,
		accounts[abandonedID].caregiver, accounts[abandonedID].parent, retiredID, abandonedID, retired).Exec(ctx)
	require.NoError(t, err)
	// The same visitor account can also be referenced by a different retired
	// school. It must not expand this order's cleanup scope.
	for _, statement := range []string{
		`UPDATE users.persons SET account_id = ? WHERE tenant_id = ? AND first_name = ? AND last_name = ?`,
		`UPDATE users.guardian_profiles SET account_id = ? WHERE tenant_id = ?`,
		`UPDATE enrollment.requests SET guardian_account_id = ? WHERE tenant_id = ?`,
		`UPDATE audit.guardian_changes SET actor_account_id = ? WHERE tenant_id = ?`,
		`UPDATE users.parent_messages SET sender_account_id = ? WHERE tenant_id = ?`,
		`UPDATE platform.email_delivery SET account_id = ? WHERE tenant_id = ?`,
	} {
		accountID := accounts[retiredID].parent
		if statement == `UPDATE users.persons SET account_id = ? WHERE tenant_id = ? AND first_name = ? AND last_name = ?` {
			accountID = accounts[retiredID].caregiver
			_, err = db.NewRaw(statement, accountID, unrelatedID, visitorFirst, visitorLast).Exec(ctx)
		} else {
			_, err = db.NewRaw(statement, accountID, unrelatedID).Exec(ctx)
		}
		require.NoError(t, err, statement)
	}
	failed := orderDemoSchool(t, db, "OGS Ost")
	_, err = queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	require.NoError(t, schools.ReserveDemoSchool(ctx, failed, failedID, 0, 0))
	require.NoError(t, schools.UpdateDemoSchool(ctx, failed, organizationtenancy.DemoSchoolState{
		SchoolID: failedID, SeedJSON: []byte(`{}`),
		VisitorAccountID: accounts[failedID].caregiver, VisitorParentAccountID: accounts[failedID].parent,
	}))
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
	assert.Equal(t, everyTrace(0), visitorTraces(t, db, failedID), "the final failed school")
	assert.Equal(t, everyTrace(1), visitorTraces(t, db, unrelatedID), "a different visitor with the same name")
	assert.Equal(t, everyTrace(1), visitorTraces(t, db, openID), "a school still open")
	assert.Equal(t, everyTrace(1), visitorTraces(t, db, foreignID), "a hidden school outside the Demo-Träger")
	var deliveries int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.email_delivery
		WHERE tenant_id IN (?, ?, ?)`, retiredID, abandonedID, failedID).Scan(ctx, &deliveries))
	assert.Zero(t, deliveries, "delivery records must not retain the visitor after their outbox is erased")

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

func TestDemoSchoolQueueErasesVisitorOutboxWithoutDelivery(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	accounts := leaveVisitorTraces(t, db, schoolID)
	var email string
	require.NoError(t, db.NewRaw(`SELECT email FROM auth.accounts WHERE id = ?`, accounts.parent).Scan(t.Context(), &email))
	var orphanedOutboxID int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.email_outbox (tenant_id, kind, recipient, payload, status)
		VALUES (?, 'parent_message', jsonb_build_object('address', ?),
			jsonb_build_object('first_name', ?, 'last_name', ?), 'sent') RETURNING id`,
		schoolID, email, visitorFirst, visitorLast).Scan(t.Context(), &orphanedOutboxID))
	slug := orderDemoSchool(t, db, "OGS Nord")
	_, err = queue.ClaimDemoSchoolOrder(t.Context())
	require.NoError(t, err)
	require.NoError(t, schools.RememberDemoSchool(t.Context(), slug, organizationtenancy.DemoSchoolState{
		SchoolID: schoolID, SeedJSON: []byte(`{}`),
		VisitorAccountID: accounts.caregiver, VisitorParentAccountID: accounts.parent,
	}))
	require.NoError(t, queue.FinishDemoSchoolOrder(t.Context(), slug, accounts.caregiver, accounts.parent))

	_, erased, err := queue.RetireDemoSchools(t.Context(), []string{slug})
	require.NoError(t, err)
	assert.Equal(t, 1, erased)
	var remaining int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.email_outbox WHERE id = ?`, orphanedOutboxID).Scan(t.Context(), &remaining))
	assert.Zero(t, remaining, "an outbox without a delivery must still be erased by its recipient account")
}

func TestDemoSchoolQueueRecoversVisitorAccountsAfterTheirStateWriteFails(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	accounts := leaveVisitorTraces(t, db, schoolID)
	ctx := t.Context()

	slug := orderDemoSchool(t, db, "OGS Nord")
	_, err = queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	// Simulate a seed where both SaveVisitor callbacks reached an account
	// creation but their state writes failed. The scoped seed addresses are
	// durable enough to recover exactly those two accounts at erasure time.
	_, err = db.NewRaw(`UPDATE auth.accounts SET email = CASE id
		WHEN ? THEN 'julia.klein@demo-' || regexp_replace(?, '^.*-', '') || '.moto-ogs.de'
		WHEN ? THEN 'sabine.schneider@demo-' || regexp_replace(?, '^.*-', '') || '.moto-ogs.de'
		END WHERE id IN (?, ?)`,
		accounts.caregiver, slug, accounts.parent, slug, accounts.caregiver, accounts.parent).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, schools.RememberDemoSchool(ctx, slug, organizationtenancy.DemoSchoolState{
		SchoolID: schoolID, SeedJSON: []byte(`{}`),
	}))
	require.NoError(t, queue.FinishDemoSchoolOrder(ctx, slug, 0, 0))

	_, erased, err := queue.RetireDemoSchools(ctx, []string{slug})
	require.NoError(t, err)
	assert.Equal(t, 1, erased)
	assert.Equal(t, everyTrace(0), visitorTraces(t, db, schoolID))
}

func TestDemoSchoolQueueAnonymizesExclusiveVisitorAccounts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	accounts := leaveVisitorTraces(t, db, schoolID)
	ctx := t.Context()
	for _, accountID := range []int64{accounts.caregiver, accounts.parent} {
		_, err = db.NewRaw(`DELETE FROM auth.account_tenants WHERE account_id = ?`, accountID).Exec(ctx)
		require.NoError(t, err)
		testpkg.MapAccountToTenant(t, db, accountID, schoolID)
	}
	_, err = db.NewRaw(`UPDATE auth.accounts
		SET username = 'demo-visitor-' || id, password_hash = 'hash', avatar = 'avatar', calendar_feed_token = 'calendar-' || id
		WHERE id IN (?, ?)`, accounts.caregiver, accounts.parent).Exec(ctx)
	require.NoError(t, err)
	var roleID int64
	require.NoError(t, db.NewRaw(`SELECT id FROM auth.roles ORDER BY id LIMIT 1`).Scan(ctx, &roleID))
	_, err = db.NewRaw(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id)
		VALUES (?, ?, ?), (?, ?, ?)`, accounts.caregiver, roleID, schoolID, accounts.parent, roleID, schoolID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO auth.tokens (account_id, token, expiry, tenant_id, portal_scope)
		VALUES (?, encode(sha256('visitor-session-caregiver'::bytea), 'hex'), NOW() + INTERVAL '1 day', ?, 'tenant'),
			(?, encode(sha256('visitor-session-parent'::bytea), 'hex'), NOW() + INTERVAL '1 day', ?, 'parent')`,
		accounts.caregiver, schoolID, accounts.parent, schoolID).Exec(ctx)
	require.NoError(t, err)
	slug := orderDemoSchool(t, db, "OGS Nord")
	_, err = queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	require.NoError(t, schools.RememberDemoSchool(ctx, slug, organizationtenancy.DemoSchoolState{
		SchoolID: schoolID, SeedJSON: []byte(`{}`),
		VisitorAccountID: accounts.caregiver, VisitorParentAccountID: accounts.parent,
	}))
	require.NoError(t, queue.FinishDemoSchoolOrder(ctx, slug, accounts.caregiver, accounts.parent))

	_, erased, err := queue.RetireDemoSchools(ctx, []string{slug})
	require.NoError(t, err)
	assert.Equal(t, 1, erased)
	var remaining int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.account_tenants WHERE account_id IN (?, ?)`, accounts.caregiver, accounts.parent).Scan(ctx, &remaining))
	assert.Zero(t, remaining, "school mappings must not retain an erased visitor")
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.account_roles WHERE account_id IN (?, ?)`, accounts.caregiver, accounts.parent).Scan(ctx, &remaining))
	assert.Zero(t, remaining, "roles must not retain an erased visitor")
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.tokens WHERE account_id IN (?, ?)`, accounts.caregiver, accounts.parent).Scan(ctx, &remaining))
	assert.Zero(t, remaining, "sessions must not retain an erased visitor")
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.guardian_student_access
		WHERE account_id = ? AND tenant_id = ?`, accounts.parent, schoolID).Scan(ctx, &remaining))
	assert.Zero(t, remaining, "guardian permissions must not retain an erased visitor")
	var retainedCredentials int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.accounts
		WHERE id IN (?, ?) AND (active OR password_hash IS NOT NULL OR username IS NOT NULL OR avatar IS NOT NULL OR calendar_feed_token IS NOT NULL
			OR email NOT LIKE 'demo-besuch-%@example.invalid')`, accounts.caregiver, accounts.parent).Scan(ctx, &retainedCredentials))
	assert.Zero(t, retainedCredentials, "the retained account tombstones must not contain usable or identifying data")
}

func TestDemoSchoolQueueRevokesSharedVisitorAccessOnlyInRetiredSchools(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	outsideID, _ := testpkg.CreateTestTenant(t, db)
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	accounts := leaveVisitorTraces(t, db, schoolID)
	ctx := t.Context()
	accountIDs := []int64{accounts.caregiver, accounts.parent}
	for _, accountID := range accountIDs {
		for _, table := range []string{"auth.account_tenants", "auth.account_roles", "auth.tokens"} {
			_, err = db.NewRaw(`DELETE FROM `+table+` WHERE account_id = ?`, accountID).Exec(ctx)
			require.NoError(t, err, table)
		}
		testpkg.MapAccountToTenant(t, db, accountID, schoolID)
		testpkg.MapAccountToTenant(t, db, accountID, outsideID)
	}
	var roleID int64
	require.NoError(t, db.NewRaw(`SELECT id FROM auth.roles ORDER BY id LIMIT 1`).Scan(ctx, &roleID))
	for _, accountID := range accountIDs {
		_, err = db.NewRaw(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?), (?, ?, ?)`,
			accountID, roleID, schoolID, accountID, roleID, outsideID).Exec(ctx)
		require.NoError(t, err)
		_, err = db.NewRaw(`INSERT INTO auth.tokens (account_id, token, expiry, tenant_id, portal_scope) VALUES
			(?, encode(sha256(convert_to('hidden-' || ?::text || '-' || ?::text, 'UTF8')), 'hex'), NOW() + INTERVAL '1 day', ?, 'tenant'),
			(?, encode(sha256(convert_to('outside-' || ?::text || '-' || ?::text, 'UTF8')), 'hex'), NOW() + INTERVAL '1 day', ?, 'tenant')`,
			accountID, accountID, schoolID, schoolID, accountID, accountID, outsideID, outsideID).Exec(ctx)
		require.NoError(t, err)
	}
	slug := orderDemoSchool(t, db, "OGS Nord")
	_, err = queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	require.NoError(t, schools.RememberDemoSchool(ctx, slug, organizationtenancy.DemoSchoolState{
		SchoolID: schoolID, SeedJSON: []byte(`{}`),
		VisitorAccountID: accounts.caregiver, VisitorParentAccountID: accounts.parent,
	}))
	require.NoError(t, queue.FinishDemoSchoolOrder(ctx, slug, accounts.caregiver, accounts.parent))

	_, erased, err := queue.RetireDemoSchools(ctx, []string{slug})
	require.NoError(t, err)
	assert.Equal(t, 1, erased)
	for _, table := range []string{"auth.account_tenants", "auth.account_roles", "auth.tokens"} {
		var hidden, outside int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM `+table+` WHERE account_id IN (?, ?) AND tenant_id = ?`,
			accounts.caregiver, accounts.parent, schoolID).Scan(ctx, &hidden), table)
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM `+table+` WHERE account_id IN (?, ?) AND tenant_id = ?`,
			accounts.caregiver, accounts.parent, outsideID).Scan(ctx, &outside), table)
		assert.Zero(t, hidden, table+" must not retain hidden-school access")
		assert.Equal(t, len(accountIDs), outside, table+" must preserve the outside-school account")
	}
	var active int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.accounts WHERE id IN (?, ?) AND active`, accounts.caregiver, accounts.parent).Scan(ctx, &active))
	assert.Equal(t, len(accountIDs), active, "a shared account stays usable outside the retired school")
}
