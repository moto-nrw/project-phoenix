package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A school role invitation left the account with a person and nothing else
// (#2222). The repair must attach the staff record to exactly that person, and
// give a caregiver-tier role its profile too.
func TestRepairSchoolIdentities(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.UniqueTestTenantID(t)

	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	adminRole := createIdentityRepairRole(t, db, tenantID, "reparatur-leitung", strPtrValue("admin"))
	userRole := createIdentityRepairRole(t, db, tenantID, "reparatur-kraft", strPtrValue("user"))
	legacyRole := createIdentityRepairRole(t, db, tenantID, "reparatur-alt", nil)
	guardianRole := createIdentityRepairRole(t, db, tenantID, "reparatur-sorge", strPtrValue("guardian"))

	admin := createBrokenSchoolIdentity(t, db, tenantID, "Repair", "Admin", adminRole)
	caregiver := createBrokenSchoolIdentity(t, db, tenantID, "Repair", "Kraft", userRole)
	legacy := createBrokenSchoolIdentity(t, db, tenantID, "Repair", "Alt", legacyRole)
	guardian := createBrokenSchoolIdentity(t, db, tenantID, "Repair", "Sorge", guardianRole)

	require.NoError(t, repairSchoolIdentitiesUp(ctx, db))
	require.NoError(t, repairSchoolIdentitiesUp(ctx, db), "repair migration must be idempotent")

	assert.Equal(t, 1, liveStaffCount(t, db, admin.personID), "admin-tier role must get a staff record")
	assert.Equal(t, 1, liveStaffCount(t, db, caregiver.personID), "caregiver-tier role must get a staff record")
	assert.Equal(t, 1, liveStaffCount(t, db, legacy.personID), "unknown tier counts as personnel")
	assert.Equal(t, 0, liveStaffCount(t, db, guardian.personID), "guardians are not personnel")

	assert.Equal(t, 0, liveTeacherCount(t, db, admin.personID), "an admin-tier role runs without a caregiver profile")
	assert.Equal(t, 1, liveTeacherCount(t, db, caregiver.personID), "a caregiver-tier role needs its profile")
	assert.Equal(t, 0, liveTeacherCount(t, db, legacy.personID))
}

// The retired platform 'teacher' role predates base_role and was never
// backfilled, so its tier reads as unknown. RoleNeedsCaregiverProfile still
// matches it by name, and the repair has to agree. Otherwise an account holding
// it gets a staff record and no caregiver profile, which leaves its groups and
// supervisions empty: the same bug one level down.
func TestRepairSchoolIdentitiesCoversLegacyTeacherRole(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.UniqueTestTenantID(t)

	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	legacyTeacher := ensureLegacySystemTeacherRole(t, db)

	holder := createBrokenSchoolIdentity(t, db, tenantID, "Repair", "Lehrer", legacyTeacher)

	require.NoError(t, repairSchoolIdentitiesUp(ctx, db))
	require.NoError(t, repairSchoolIdentitiesUp(ctx, db), "repair migration must be idempotent")

	assert.Equal(t, 1, liveStaffCount(t, db, holder.personID), "the retired teacher role is personnel")
	assert.Equal(t, 1, liveTeacherCount(t, db, holder.personID), "the retired teacher role needs its caregiver profile")
}

// ensureLegacySystemTeacherRole returns the id of the retired platform
// 'teacher' role, creating it when the schema no longer seeds it. The name is
// what identifies it: base_role stays NULL, which is the whole point.
func ensureLegacySystemTeacherRole(t *testing.T, db *testpkg.DB) int64 {
	t.Helper()
	ctx := context.Background()

	var ids []int64
	require.NoError(t, db.NewRaw(
		`SELECT id FROM auth.roles WHERE is_system AND LOWER(TRIM(name)) = 'teacher'`,
	).Scan(ctx, &ids))
	if len(ids) > 0 {
		return ids[0]
	}

	var id int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO auth.roles (name, description, is_system, tenant_id, base_role, created_at, updated_at)
		VALUES ('teacher', 'Retired platform caregiver role', TRUE, NULL, NULL, NOW(), NOW())
		RETURNING id`).Scan(ctx, &id))
	// A system role has no tenant: the row this test had to create is shared
	// state, so it goes away with the test (#2419).
	t.Cleanup(func() { deleteSystemRoleRow(t, db, id) })
	return id
}

// An account whose access was revoked keeps its person row on purpose, but must
// not be silently re-staffed by the repair.
func TestRepairSchoolIdentitiesSkipsInactiveAccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.UniqueTestTenantID(t)

	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	role := createIdentityRepairRole(t, db, tenantID, "reparatur-inaktiv", strPtrValue("admin"))

	revoked := createBrokenSchoolIdentity(t, db, tenantID, "Repair", "Inaktiv", role)

	_, err := db.ExecContext(ctx, `
		UPDATE auth.account_tenants SET status = 'inactive'
		WHERE account_id = ? AND tenant_id = ?`, revoked.accountID, tenantID)
	require.NoError(t, err)

	require.NoError(t, repairSchoolIdentitiesUp(ctx, db))
	assert.Equal(t, 0, liveStaffCount(t, db, revoked.personID))
}

type brokenSchoolIdentity struct {
	accountID int64
	personID  int64
}

func strPtrValue(v string) *string { return &v }

func createIdentityRepairRole(t *testing.T, db *testpkg.DB, tenantID int64, name string, baseRole *string) int64 {
	t.Helper()
	var id int64
	err := db.NewRaw(`
		INSERT INTO auth.roles (name, description, is_system, tenant_id, base_role, created_at, updated_at)
		VALUES (?, 'Reparatur-Testrolle', FALSE, ?, ?, NOW(), NOW())
		RETURNING id`,
		fmt.Sprintf("%s-%d", name, tenantID), tenantID, baseRole,
	).Scan(context.Background(), &id)
	require.NoError(t, err)
	return id
}

// createBrokenSchoolIdentity reproduces the state the invitation flow produced:
// an active account at the school, holding a role, with a person record and no
// staff record.
func createBrokenSchoolIdentity(t *testing.T, db *testpkg.DB, tenantID int64, firstName, lastName string, roleID int64) brokenSchoolIdentity {
	t.Helper()
	ctx := context.Background()

	var accountID int64
	err := db.NewRaw(`
		INSERT INTO auth.accounts (email, username, password_hash, active, created_at, updated_at)
		VALUES (?, ?, 'x', TRUE, NOW(), NOW())
		RETURNING id`,
		fmt.Sprintf("repair-%s-%d@example.com", lastName, tenantID),
		fmt.Sprintf("repair_%s_%d", lastName, tenantID),
	).Scan(ctx, &accountID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO auth.account_tenants (account_id, tenant_id, status, activated_at, created_at, updated_at)
		VALUES (?, ?, 'active', NOW(), NOW(), NOW())`, accountID, tenantID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())`, accountID, roleID, tenantID)
	require.NoError(t, err)

	var personID int64
	err = db.NewRaw(`
		INSERT INTO users.persons (tenant_id, first_name, last_name, account_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, NOW(), NOW())
		RETURNING id`, tenantID, firstName, lastName, accountID).Scan(ctx, &personID)
	require.NoError(t, err)

	return brokenSchoolIdentity{accountID: accountID, personID: personID}
}

func liveStaffCount(t *testing.T, db *testpkg.DB, personID int64) int {
	t.Helper()
	count, err := db.NewSelect().
		TableExpr(`users.staff AS "s"`).
		Where(`"s".person_id = ?`, personID).
		Where(`"s".deleted_at IS NULL`).
		Count(context.Background())
	require.NoError(t, err)
	return count
}

// The repair matches persons, staff and teachers within one school, and the
// statements say so explicitly. What makes an unscoped match unreachable rather
// than merely unlikely is the schema: the composite foreign keys from migration
// 1.15.2 tie each row's tenant to its parent's, so a staff row on another
// school's person — the shape that could otherwise read as "already repaired"
// and hide an account from both the fix and the report — cannot be stored at
// all.
//
// Pinned here because the SQL above leans on it. Drop either constraint and the
// tenant predicates stop being belt-and-braces and start being the only thing
// holding the invariant up, which is worth failing a test over.
func TestRepairSchoolIdentitiesTenantScopingIsEnforcedBySchema(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.UniqueTestTenantID(t)
	otherTenantID := testpkg.UniqueTestTenantID(t)

	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, otherTenantID)

	role := createIdentityRepairRole(t, db, tenantID, "reparatur-fremdschule", strPtrValue("user"))

	holder := createBrokenSchoolIdentity(t, db, tenantID, "Repair", "Fremd", role)

	_, err := db.ExecContext(ctx, `
		INSERT INTO users.staff (tenant_id, person_id, staff_notes, created_at, updated_at)
		VALUES (?, ?, '', NOW(), NOW())`, otherTenantID, holder.personID)
	require.Error(t, err, "a staff row must not outlive the school of the person it belongs to")
	assert.Contains(t, err.Error(), "fk_staff_person_tenant")

	// And with the invariant intact, the repair does what it says on the tin.
	require.NoError(t, repairSchoolIdentitiesUp(ctx, db))
	require.NoError(t, repairSchoolIdentitiesUp(ctx, db), "repair migration must be idempotent")

	assert.Equal(t, 1, liveStaffCountForTenant(t, db, holder.personID, tenantID),
		"the school the account holds its role at gets the staff record")
	assert.Equal(t, 1, liveTeacherCountForTenant(t, db, holder.personID, tenantID),
		"and the caregiver profile lands under that same school")
}

func liveStaffCountForTenant(t *testing.T, db *testpkg.DB, personID, tenantID int64) int {
	t.Helper()
	count, err := db.NewSelect().
		TableExpr(`users.staff AS "s"`).
		Where(`"s".person_id = ?`, personID).
		Where(`"s".tenant_id = ?`, tenantID).
		Where(`"s".deleted_at IS NULL`).
		Count(context.Background())
	require.NoError(t, err)
	return count
}

func liveTeacherCountForTenant(t *testing.T, db *testpkg.DB, personID, tenantID int64) int {
	t.Helper()
	count, err := db.NewSelect().
		TableExpr(`users.teachers AS "t"`).
		Join(`JOIN users.staff AS "s" ON "s".id = "t".staff_id`).
		Where(`"s".person_id = ?`, personID).
		Where(`"t".tenant_id = ?`, tenantID).
		Where(`"t".deleted_at IS NULL`).
		Count(context.Background())
	require.NoError(t, err)
	return count
}

func liveTeacherCount(t *testing.T, db *testpkg.DB, personID int64) int {
	t.Helper()
	count, err := db.NewSelect().
		TableExpr(`users.teachers AS "t"`).
		Join(`JOIN users.staff AS "s" ON "s".id = "t".staff_id`).
		Where(`"s".person_id = ?`, personID).
		Where(`"t".deleted_at IS NULL`).
		Count(context.Background())
	require.NoError(t, err)
	return count
}

// The second source of the same broken state: an account that reached the
// school through /auth/link-to-tenant never got a person here, so there is
// nothing for the staff step to attach to. Where the account holds a person at
// another of its schools the name is not lost — the login already serves it
// from there, which is why such an account shows a name in the header and is
// absent from the school's own staff list. The repair writes that name into a
// person at this school and staffs it, by tier, exactly as the provisioning
// would have.
func TestRepairSchoolIdentitiesCreatesPersonFromMappedSchool(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	homeTenantID := testpkg.UniqueTestTenantID(t)
	targetTenantID := testpkg.UniqueTestTenantID(t)

	testpkg.EnsureTestTenant(t, db, homeTenantID)
	testpkg.EnsureTestTenant(t, db, targetTenantID)
	defer testpkg.CleanupTenantTestData(t, db, homeTenantID, targetTenantID)

	adminRole := createIdentityRepairRole(t, db, targetTenantID, "quer-leitung", strPtrValue("admin"))
	userRole := createIdentityRepairRole(t, db, targetTenantID, "quer-kraft", strPtrValue("user"))
	guardianRole := createIdentityRepairRole(t, db, targetTenantID, "quer-sorge", strPtrValue("guardian"))

	admin := createSchoolAccessWithoutPerson(t, db, homeTenantID, targetTenantID, "Quer", "Leitung", adminRole)
	caregiver := createSchoolAccessWithoutPerson(t, db, homeTenantID, targetTenantID, "Quer", "Kraft", userRole)
	guardian := createSchoolAccessWithoutPerson(t, db, homeTenantID, targetTenantID, "Quer", "Sorge", guardianRole)

	require.NoError(t, repairSchoolIdentitiesUp(ctx, db))
	require.NoError(t, repairSchoolIdentitiesUp(ctx, db), "repair migration must be idempotent")

	adminPerson := requireSinglePersonAt(t, db, targetTenantID, admin)
	assert.Equal(t, "Quer", personFirstName(t, db, adminPerson))
	assert.Equal(t, "Leitung", personLastName(t, db, adminPerson))
	assert.Equal(t, 1, liveStaffCount(t, db, adminPerson), "a staff-tier role needs its staff record here")
	assert.Equal(t, 0, liveTeacherCount(t, db, adminPerson), "an admin-tier role runs without a caregiver profile")

	caregiverPerson := requireSinglePersonAt(t, db, targetTenantID, caregiver)
	assert.Equal(t, 1, liveStaffCount(t, db, caregiverPerson))
	assert.Equal(t, 1, liveTeacherCount(t, db, caregiverPerson), "a caregiver-tier role needs its profile")

	assert.Equal(t, 0, livePersonCount(t, db, targetTenantID, guardian),
		"guardians are not personnel and must not receive a person record here")
}

// Two schools that disagree on the name is an ambiguity the login refuses to
// resolve (it mints a token without a name). The migration is no more
// entitled to pick one, so the account falls through to the report instead of
// getting a guessed identity.
func TestRepairSchoolIdentitiesSkipsAmbiguousNameAcrossSchools(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	firstTenantID := testpkg.UniqueTestTenantID(t)
	secondTenantID := testpkg.UniqueTestTenantID(t)
	targetTenantID := testpkg.UniqueTestTenantID(t)

	testpkg.EnsureTestTenant(t, db, firstTenantID)
	testpkg.EnsureTestTenant(t, db, secondTenantID)
	testpkg.EnsureTestTenant(t, db, targetTenantID)
	defer testpkg.CleanupTenantTestData(t, db, firstTenantID, secondTenantID, targetTenantID)

	role := createIdentityRepairRole(t, db, targetTenantID, "quer-uneindeutig", strPtrValue("admin"))

	account := createSchoolAccessWithoutPerson(t, db, firstTenantID, targetTenantID, "Alex", "Wechsel", role)

	addActiveSchoolAccess(t, db, account, secondTenantID)
	addPersonAt(t, db, secondTenantID, account, "Alexandra", "Wechsel")

	require.NoError(t, repairSchoolIdentitiesUp(ctx, db))

	assert.Equal(t, 0, livePersonCount(t, db, targetTenantID, account),
		"a name the account's schools disagree on must not be copied")
}

// createSchoolAccessWithoutPerson reproduces the state /auth/link-to-tenant
// left behind: an account with a person at its home school, actively mapped to
// a second school where it holds a role and has no person at all.
func createSchoolAccessWithoutPerson(
	t *testing.T,
	db *testpkg.DB,
	homeTenantID, targetTenantID int64,
	firstName, lastName string,
	targetRoleID int64,
) int64 {
	t.Helper()
	ctx := context.Background()

	var accountID int64
	err := db.NewRaw(`
		INSERT INTO auth.accounts (email, username, password_hash, active, created_at, updated_at)
		VALUES (?, ?, 'x', TRUE, NOW(), NOW())
		RETURNING id`,
		fmt.Sprintf("quer-%s-%d@example.com", lastName, targetTenantID),
		fmt.Sprintf("quer_%s_%d", lastName, targetTenantID),
	).Scan(ctx, &accountID)
	require.NoError(t, err)

	addActiveSchoolAccess(t, db, accountID, homeTenantID)
	addActiveSchoolAccess(t, db, accountID, targetTenantID)
	addPersonAt(t, db, homeTenantID, accountID, firstName, lastName)

	_, err = db.ExecContext(ctx, `
		INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())`, accountID, targetRoleID, targetTenantID)
	require.NoError(t, err)

	return accountID
}

func addActiveSchoolAccess(t *testing.T, db *testpkg.DB, accountID, tenantID int64) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO auth.account_tenants (account_id, tenant_id, status, activated_at, created_at, updated_at)
		VALUES (?, ?, 'active', NOW(), NOW(), NOW())`, accountID, tenantID)
	require.NoError(t, err)
}

func addPersonAt(t *testing.T, db *testpkg.DB, tenantID, accountID int64, firstName, lastName string) int64 {
	t.Helper()
	var personID int64
	err := db.NewRaw(`
		INSERT INTO users.persons (tenant_id, first_name, last_name, account_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, NOW(), NOW())
		RETURNING id`, tenantID, firstName, lastName, accountID).Scan(context.Background(), &personID)
	require.NoError(t, err)
	return personID
}

func livePersonCount(t *testing.T, db *testpkg.DB, tenantID, accountID int64) int {
	t.Helper()
	count, err := db.NewSelect().
		TableExpr(`users.persons AS "p"`).
		Where(`"p".tenant_id = ?`, tenantID).
		Where(`"p".account_id = ?`, accountID).
		Where(`"p".deleted_at IS NULL`).
		Count(context.Background())
	require.NoError(t, err)
	return count
}

func requireSinglePersonAt(t *testing.T, db *testpkg.DB, tenantID, accountID int64) int64 {
	t.Helper()
	var ids []int64
	require.NoError(t, db.NewRaw(`
		SELECT id FROM users.persons
		WHERE tenant_id = ? AND account_id = ? AND deleted_at IS NULL`,
		tenantID, accountID).Scan(context.Background(), &ids))
	require.Len(t, ids, 1, "exactly one person record must exist for this account at this school")
	return ids[0]
}

func personFirstName(t *testing.T, db *testpkg.DB, personID int64) string {
	t.Helper()
	var name string
	require.NoError(t, db.NewRaw(`SELECT first_name FROM users.persons WHERE id = ?`, personID).
		Scan(context.Background(), &name))
	return name
}

func personLastName(t *testing.T, db *testpkg.DB, personID int64) string {
	t.Helper()
	var name string
	require.NoError(t, db.NewRaw(`SELECT last_name FROM users.persons WHERE id = ?`, personID).
		Scan(context.Background(), &name))
	return name
}

// Lehrkraft never earns a caregiver profile on its own (#1772) — but that is a
// property of the ROLE, not of the account. An account that also holds a real
// caregiver-tier role has always needed the profile that role reads through,
// and RoleNeedsCaregiverProfile decides per role. Skipping every account with a
// Lehrkraft role would leave those in the half-written state this migration
// exists to end, and unreported too, since their staff record does get created.
func TestRepairSchoolIdentitiesCaregiverProfileIsDecidedPerRole(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.UniqueTestTenantID(t)

	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	lehrkraftRole := ensureLehrkraftSystemRole(t, db)
	caregiverRole := createIdentityRepairRole(t, db, tenantID, "doppel-kraft", strPtrValue("user"))

	lehrkraftOnly := createBrokenSchoolIdentity(t, db, tenantID, "Doppel", "Nur", lehrkraftRole)
	dualRole := createBrokenSchoolIdentity(t, db, tenantID, "Doppel", "Beides", lehrkraftRole)
	addRoleAt(t, db, dualRole.accountID, caregiverRole, tenantID)

	require.NoError(t, repairSchoolIdentitiesUp(ctx, db))
	require.NoError(t, repairSchoolIdentitiesUp(ctx, db), "repair migration must be idempotent")

	assert.Equal(t, 1, liveStaffCount(t, db, lehrkraftOnly.personID), "Lehrkraft is personnel")
	assert.Equal(t, 0, liveTeacherCount(t, db, lehrkraftOnly.personID),
		"a Lehrkraft with no other caregiver-tier role must stay without a profile")

	assert.Equal(t, 1, liveStaffCount(t, db, dualRole.personID))
	assert.Equal(t, 1, liveTeacherCount(t, db, dualRole.personID),
		"the caregiver-tier role the account also holds needs its profile")
}

// A person that is a child's record is never turned into staff. The account is
// then still broken, so it has to reach the report — otherwise the run looks
// clean while the account stays unusable.
func TestRepairSchoolIdentitiesReportsStudentLinkedIdentity(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.UniqueTestTenantID(t)

	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	role := createIdentityRepairRole(t, db, tenantID, "kind-verknuepft", strPtrValue("admin"))

	linked := createBrokenSchoolIdentity(t, db, tenantID, "Kind", "Verknuepft", role)
	markPersonAsStudent(t, db, tenantID, linked.personID)

	require.NoError(t, repairSchoolIdentitiesUp(ctx, db))

	assert.Equal(t, 0, liveStaffCount(t, db, linked.personID),
		"a child's person record must never be filed as personnel")

	rows, err := listUnrepairableSchoolIdentities(ctx, db)
	require.NoError(t, err)

	reason, listed := reasonForAccount(rows, linked.accountID, tenantID)
	require.True(t, listed, "the account the repair could not complete must be reported")
	assert.Contains(t, reason, "child")
}

func reasonForAccount(rows []unrepairableSchoolIdentity, accountID, tenantID int64) (string, bool) {
	for _, row := range rows {
		if row.AccountID == accountID && row.TenantID == tenantID {
			return row.Reason, true
		}
	}
	return "", false
}

// ensureLehrkraftSystemRole returns the id of the platform Lehrkraft role
// (migration 1.15.278), creating it if this database predates it.
func ensureLehrkraftSystemRole(t *testing.T, db *testpkg.DB) int64 {
	t.Helper()
	ctx := context.Background()

	var ids []int64
	require.NoError(t, db.NewRaw(
		`SELECT id FROM auth.roles WHERE is_system AND LOWER(TRIM(name)) = 'lehrkraft'`,
	).Scan(ctx, &ids))
	if len(ids) > 0 {
		return ids[0]
	}

	var id int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO auth.roles (name, description, is_system, tenant_id, base_role, created_at, updated_at)
		VALUES ('lehrkraft', 'Lehrkraft', TRUE, NULL, 'user', NOW(), NOW())
		RETURNING id`).Scan(ctx, &id))
	t.Cleanup(func() { deleteSystemRoleRow(t, db, id) })
	return id
}

// deleteSystemRoleRow removes a tenant-less role this package had to create,
// plus the catalog rows hanging off it (#2419).
func deleteSystemRoleRow(t *testing.T, db *testpkg.DB, roleID int64) {
	t.Helper()
	bg := context.Background()
	_, _ = db.ExecContext(bg, `DELETE FROM auth.account_roles WHERE role_id = ?`, roleID)
	_, _ = db.ExecContext(bg, `DELETE FROM auth.role_permissions WHERE role_id = ?`, roleID)
	_, _ = db.ExecContext(bg, `DELETE FROM auth.roles WHERE id = ?`, roleID)
}

func addRoleAt(t *testing.T, db *testpkg.DB, accountID, roleID, tenantID int64) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())`, accountID, roleID, tenantID)
	require.NoError(t, err)
}

func markPersonAsStudent(t *testing.T, db *testpkg.DB, tenantID, personID int64) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO users.students (tenant_id, person_id, school_class, created_at, updated_at)
		VALUES (?, ?, '1a', NOW(), NOW())`, tenantID, personID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM users.students WHERE person_id = ?`, personID)
	})
}

// A child is never personnel, and the caregiver step has to refuse it on its own
// account: it starts from users.staff, so a staff row that legacy data already
// put on a child's person would collect a caregiver profile here even though the
// staff step declined to create one for it.
func TestRepairSchoolIdentitiesSkipsStudentPersonForCaregiverProfile(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.UniqueTestTenantID(t)

	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	userRole := createIdentityRepairRole(t, db, tenantID, "reparatur-kind-kraft", strPtrValue("user"))

	child := createBrokenSchoolIdentity(t, db, tenantID, "Kind", "Kraft", userRole)

	markPersonAsStudent(t, db, tenantID, child.personID)
	// Legacy data the staff step would never create: a staff row on the child.
	_, err := db.ExecContext(ctx, `
		INSERT INTO users.staff (tenant_id, person_id, staff_notes, created_at, updated_at)
		VALUES (?, ?, '', NOW(), NOW())`, tenantID, child.personID)
	require.NoError(t, err)

	require.NoError(t, repairSchoolIdentitiesUp(ctx, db))
	require.NoError(t, repairSchoolIdentitiesUp(ctx, db), "repair migration must be idempotent")

	assert.Equal(t, 0, liveTeacherCount(t, db, child.personID),
		"a child's person must not receive a caregiver profile, even with a staff row on it")

	// And it has to be reported. The staff row means nothing is missing by the
	// "no staff record" measure, so asking only that question would let the one
	// genuinely invalid identity in this migration's scope pass unmentioned —
	// a child filed as personnel, left in place and never named.
	rows, err := listUnrepairableSchoolIdentities(ctx, db)
	require.NoError(t, err)

	reason, listed := reasonForAccount(rows, child.accountID, tenantID)
	require.True(t, listed, "a child's person carrying a staff row must still be reported")
	assert.Contains(t, reason, "child")
}
