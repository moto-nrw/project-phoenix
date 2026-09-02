package parent_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// enrollmentRow is a minimal fixture for inserting one request + one
// child. The repo-under-test reads via raw SQL, so going around the BUN
// models keeps the test independent of model hooks/timestamps.
type enrollmentRow struct {
	tenantID          int64
	phaseID           int64
	guardianAccountID *int64
	guardianEmail     string
	statusToken       string
	submittedAt       time.Time
	withdrawnAt       *time.Time
	childFirstName    string
	childLastName     string
	childStatus       string
	createdStudentID  *int64
}

// insertEnrollment inserts one enrollment.requests row and one
// enrollment.request_children row, returning the request id. Runs
// under phoenix_admin so RLS is bypassed.
func insertEnrollment(t *testing.T, db *bun.DB, row enrollmentRow) int64 {
	t.Helper()
	if row.submittedAt.IsZero() {
		row.submittedAt = time.Now()
	}
	if row.childStatus == "" {
		row.childStatus = "submitted"
	}

	var requestID int64
	err := tenant.WithAdminTx(testpkg.WithTenantRuntime(t, context.Background(), db), db, func(ctx context.Context, tx bun.Tx) error {
		if err := tx.NewRaw(`
			INSERT INTO enrollment.requests
			  (tenant_id, phase_id, guardian_account_id, guardian_first_name,
			   guardian_last_name, guardian_email, status_token, submitted_at,
			   withdrawn_at, consent_flags, custom_data)
			VALUES (?, ?, ?, 'Anna', 'Beispiel', ?, ?, ?, ?, '{}'::jsonb, '{}'::jsonb)
			RETURNING id
		`,
			row.tenantID, row.phaseID, row.guardianAccountID,
			row.guardianEmail, row.statusToken, row.submittedAt, row.withdrawnAt,
		).Scan(ctx, &requestID); err != nil {
			return err
		}
		_, err := tx.NewRaw(`
			INSERT INTO enrollment.request_children
			  (tenant_id, request_id, first_name, last_name, date_of_birth,
			   status, activation_mode, sort_order, custom_data, created_student_id)
			VALUES (?, ?, ?, ?, '2018-04-15', ?, 'scheduled', 0, '{}'::jsonb, ?)
		`, row.tenantID, requestID, row.childFirstName, row.childLastName, row.childStatus, row.createdStudentID).Exec(ctx)
		return err
	})
	require.NoError(t, err)
	return requestID
}

func insertEnrollmentChild(
	t *testing.T,
	db *bun.DB,
	tenantID, requestID int64,
	firstName, lastName string,
	sortOrder int,
	createdStudentID *int64,
) {
	t.Helper()
	err := tenant.WithAdminTx(testpkg.WithTenantRuntime(t, context.Background(), db), db, func(ctx context.Context, tx bun.Tx) error {
		_, e := tx.NewRaw(`
			INSERT INTO enrollment.request_children
			  (tenant_id, request_id, first_name, last_name, date_of_birth,
			   status, activation_mode, sort_order, custom_data, created_student_id)
			VALUES (?, ?, ?, ?, '2018-04-15', 'submitted',
			        'scheduled', ?, '{}'::jsonb, ?)
		`, tenantID, requestID, firstName, lastName, sortOrder, createdStudentID).Exec(ctx)
		return e
	})
	require.NoError(t, err)
}

// insertTestPhase inserts a minimal phase row directly. Returns the new
// phase id. The repo test only needs phase_id + name + service date
// columns to exist, so we skip the enrollment-service constructor.
func insertTestPhase(t *testing.T, db *bun.DB, tenantID int64, name string) int64 {
	t.Helper()
	var id int64
	err := tenant.WithAdminTx(testpkg.WithTenantRuntime(t, context.Background(), db), db, func(ctx context.Context, tx bun.Tx) error {
		return tx.NewRaw(`
			INSERT INTO enrollment.phases
			  (tenant_id, name, kind, service_start_date, service_end_date,
			   is_active, care_overflow_mode, show_status_reason_to_parent)
			VALUES (?, ?, 'school_year', '2026-09-01', '2027-07-31', TRUE, 'waitlist', FALSE)
			RETURNING id
		`, tenantID, name).Scan(ctx, &id)
	})
	require.NoError(t, err)
	return id
}

// listByAccount runs the repo's ListByAccount inside an admin tx, which
// is how the production code path wraps the call (parent service uses
// tenant.WithAdminTx around it).
func listByAccount(t *testing.T, db *bun.DB, accountID int64) []*parentModels.EnrollmentRequestSummary {
	t.Helper()
	repo := newSchoolProjectionFactory(t, db).ParentEnrollmentRequest
	var out []*parentModels.EnrollmentRequestSummary
	err := tenant.WithAdminTx(testpkg.WithTenantRuntime(t, context.Background(), db), db, func(ctx context.Context, _ bun.Tx) error {
		got, listErr := repo.ListByAccount(ctx, accountID)
		out = got
		return listErr
	})
	require.NoError(t, err)
	return out
}

// backfill runs BackfillGuardianAccountID inside an admin tx and
// returns the affected row count.
func backfill(t *testing.T, db *bun.DB, accountID int64, email string) int {
	t.Helper()
	repo := parentRepo.NewEnrollmentRequestRepository(db)
	var n int
	err := tenant.WithAdminTx(testpkg.WithTenantRuntime(t, context.Background(), db), db, func(ctx context.Context, _ bun.Tx) error {
		got, bErr := repo.BackfillGuardianAccountID(ctx, accountID, email)
		n = got
		return bErr
	})
	require.NoError(t, err)
	return n
}

// readGuardianAccountID returns the guardian_account_id column for one
// row (or nil if NULL / row missing). Runs as admin so cross-tenant
// lookups work. Tolerates sql.ErrNoRows because some tests delete the
// row mid-defer cleanup if scheduled in an unfortunate order.
func readGuardianAccountID(t *testing.T, db *bun.DB, requestID int64) *int64 {
	t.Helper()
	var raw *int64
	err := tenant.WithAdminTx(testpkg.WithTenantRuntime(t, context.Background(), db), db, func(ctx context.Context, tx bun.Tx) error {
		scanErr := tx.NewRaw(`SELECT guardian_account_id FROM enrollment.requests WHERE id = ?`, requestID).
			Scan(ctx, &raw)
		if errors.Is(scanErr, sql.ErrNoRows) {
			return nil
		}
		return scanErr
	})
	require.NoError(t, err)
	return raw
}

// --- ListByAccount ---

func TestEnrollmentRequestRepository_ListByAccount_BasicHappyPath(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.Tenant(t)

	account := testpkg.CreateTestAccount(t, db, "parentlist-basic")
	defer func() {
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	}()

	phaseID := insertTestPhase(t, db, tenantA, "phase-list-basic-"+t.Name())

	insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           phaseID,
		guardianAccountID: &account.ID,
		guardianEmail:     "anna@example.com",
		statusToken:       fmt.Sprintf("tok-basic-%d", time.Now().UnixNano()),
		childFirstName:    "Lina",
		childLastName:     "Beispiel",
		childStatus:       "submitted",
	})

	results := listByAccount(t, db, account.ID)
	require.Len(t, results, 1)
	assert.Equal(t, tenantA, results[0].TenantID)
	assert.Equal(t, phaseID, results[0].PhaseID)
	require.Len(t, results[0].Children, 1)
	assert.Equal(t, "Lina", results[0].Children[0].FirstName)
	assert.Equal(t, "submitted", results[0].Children[0].Status)
	assert.NotEmpty(t, results[0].SchoolName, "school name must be joined in")
	assert.NotEmpty(t, results[0].SchoolSlug, "school slug must be joined in")
}

func TestEnrollmentRequestRepository_ListByAccount_OrdersBySubmittedAtDesc(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.Tenant(t)

	account := testpkg.CreateTestAccount(t, db, "parentlist-order")
	defer func() {
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	}()

	phaseID := insertTestPhase(t, db, tenantA, "phase-list-order-"+t.Name())

	now := time.Now()
	older := now.Add(-2 * time.Hour)
	middle := now.Add(-1 * time.Hour)

	oldID := insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           phaseID,
		guardianAccountID: &account.ID,
		guardianEmail:     "anna@example.com",
		statusToken:       fmt.Sprintf("tok-old-%d", time.Now().UnixNano()),
		submittedAt:       older,
		childFirstName:    "Old",
		childLastName:     "Submission",
	})
	midID := insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           phaseID,
		guardianAccountID: &account.ID,
		guardianEmail:     "anna@example.com",
		statusToken:       fmt.Sprintf("tok-mid-%d", time.Now().UnixNano()),
		submittedAt:       middle,
		childFirstName:    "Mid",
		childLastName:     "Submission",
	})
	newID := insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           phaseID,
		guardianAccountID: &account.ID,
		guardianEmail:     "anna@example.com",
		statusToken:       fmt.Sprintf("tok-new-%d", time.Now().UnixNano()),
		submittedAt:       now,
		childFirstName:    "New",
		childLastName:     "Submission",
	})

	results := listByAccount(t, db, account.ID)
	require.Len(t, results, 3)
	// DESC: newest first.
	assert.Equal(t, newID, results[0].RequestID)
	assert.Equal(t, midID, results[1].RequestID)
	assert.Equal(t, oldID, results[2].RequestID)
}

func TestEnrollmentRequestRepository_ListByAccount_CrossTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.Tenant(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantB)

	// One account with submissions at two different tenants — the cross-
	// tenant query must return both rows (parent dashboard is global).
	account := testpkg.CreateTestAccount(t, db, "parentlist-cross")
	defer func() {
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	}()

	t1Phase := insertTestPhase(t, db, tenantA, "phase-cross-t1-"+t.Name())
	t2Phase := insertTestPhase(t, db, tenantB, "phase-cross-t2-"+t.Name())

	insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           t1Phase,
		guardianAccountID: &account.ID,
		guardianEmail:     "anna@example.com",
		statusToken:       fmt.Sprintf("tok-cross-1-%d", time.Now().UnixNano()),
		childFirstName:    "Lina",
		childLastName:     "T1",
	})
	insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantB,
		phaseID:           t2Phase,
		guardianAccountID: &account.ID,
		guardianEmail:     "anna@example.com",
		statusToken:       fmt.Sprintf("tok-cross-2-%d", time.Now().UnixNano()),
		childFirstName:    "Lina",
		childLastName:     "T2",
	})

	results := listByAccount(t, db, account.ID)
	require.Len(t, results, 2)
	seenTenants := map[int64]bool{}
	for _, r := range results {
		seenTenants[r.TenantID] = true
	}
	assert.True(t, seenTenants[tenantA])
	assert.True(t, seenTenants[tenantB])
}

func TestEnrollmentRequestRepository_ListByAccount_IgnoresOtherAccounts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.Tenant(t)

	mine := testpkg.CreateTestAccount(t, db, "parentlist-mine")
	other := testpkg.CreateTestAccount(t, db, "parentlist-other")
	defer func() {
		bg := context.Background()
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id IN (?, ?)", mine.ID, other.ID).Exec(bg)
	}()

	phaseID := insertTestPhase(t, db, tenantA, "phase-list-mine-"+t.Name())

	insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           phaseID,
		guardianAccountID: &mine.ID,
		guardianEmail:     "mine-" + strings.ToLower(t.Name()) + "@example.com",
		statusToken:       fmt.Sprintf("tok-mine-%d", time.Now().UnixNano()),
		childFirstName:    "Mine",
		childLastName:     "Child",
	})
	insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           phaseID,
		guardianAccountID: &other.ID,
		guardianEmail:     "other@example.com",
		statusToken:       fmt.Sprintf("tok-other-%d", time.Now().UnixNano()),
		childFirstName:    "Other",
		childLastName:     "Child",
	})
	// Also a row with no guardian_account_id at all — must not leak in.
	insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           phaseID,
		guardianAccountID: nil,
		guardianEmail:     "anonymous@example.com",
		statusToken:       fmt.Sprintf("tok-anon-%d", time.Now().UnixNano()),
		childFirstName:    "Anon",
		childLastName:     "Child",
	})

	results := listByAccount(t, db, mine.ID)
	require.Len(t, results, 1)
	require.Len(t, results[0].Children, 1)
	assert.Equal(t, "Mine", results[0].Children[0].FirstName)
}

func TestEnrollmentRequestRepository_ListByAccount_FiltersMaterializedChildWithoutEnrollmentPermission(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.Tenant(t)

	account := testpkg.CreateTestAccount(t, db, "parentlist-permission")
	profile := testpkg.CreateTestGuardianProfile(t, db, "parentlist-permission")
	student := testpkg.CreateTestStudent(t, db, "Materialized", "Child", "1a")

	factory := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	require.NoError(t, factory.GuardianProfile.LinkAccount(ctx, profile.ID, account.ID))
	relationship := &usersModels.StudentGuardian{
		StudentID:         student.ID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "other",
		GuardianRole:      authorize.GuardianRolePickupOnly,
		Permissions:       map[string]interface{}{},
	}
	relationship.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, factory.StudentGuardian.Create(ctx, relationship))

	phaseID := insertTestPhase(t, db, tenantA, "phase-list-permission-"+t.Name())
	insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           phaseID,
		guardianAccountID: &account.ID,
		guardianEmail:     "anna@example.com",
		statusToken:       fmt.Sprintf("tok-perm-%d", time.Now().UnixNano()),
		childFirstName:    "Materialized",
		childLastName:     "Child",
		createdStudentID:  &student.ID,
	})

	results := listByAccount(t, db, account.ID)
	assert.Empty(t, results)
}

func TestEnrollmentRequestRepository_ListByAccount_HidesMixedPermissionMaterializedRequest(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.Tenant(t)

	account := testpkg.CreateTestAccount(t, db, "parentlist-mixed-permission")
	profile := testpkg.CreateTestGuardianProfile(t, db, "parentlist-mixed-permission")
	visible := testpkg.CreateTestStudent(t, db, "Visible", "Child", "1a")
	hidden := testpkg.CreateTestStudent(t, db, "Hidden", "Child", "1b")

	factory := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	require.NoError(t, factory.GuardianProfile.LinkAccount(ctx, profile.ID, account.ID))
	relationship := &usersModels.StudentGuardian{
		StudentID:         visible.ID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "parent",
		GuardianRole:      authorize.GuardianRoleLegalGuardian,
		Permissions: map[string]interface{}{
			authorize.GuardianPermissionEnrollmentsView: true,
		},
	}
	relationship.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, factory.StudentGuardian.Create(ctx, relationship))

	phaseID := insertTestPhase(t, db, tenantA, "phase-list-mixed-permission-"+t.Name())
	requestID := insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           phaseID,
		guardianAccountID: &account.ID,
		guardianEmail:     "anna@example.com",
		statusToken:       fmt.Sprintf("tok-mixed-perm-%d", time.Now().UnixNano()),
		childFirstName:    "Visible",
		childLastName:     "Child",
		createdStudentID:  &visible.ID,
	})
	insertEnrollmentChild(t, db, tenantA, requestID, "Hidden", "Child", 1, &hidden.ID)

	results := listByAccount(t, db, account.ID)
	assert.Empty(t, results, "a shared request must not expose siblings when any materialized child lacks view permission")
}

func TestEnrollmentRequestRepository_ListByAccount_EmptyForUnknownAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	// Don't insert any requests for this account.
	account := testpkg.CreateTestAccount(t, db, "parentlist-empty")
	defer func() {
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	}()

	results := listByAccount(t, db, account.ID)
	assert.NotNil(t, results)
	assert.Empty(t, results, "no rows must yield an empty (non-nil) slice")
}

func TestEnrollmentRequestRepository_ListByAccount_GroupsMultipleChildren(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.Tenant(t)

	account := testpkg.CreateTestAccount(t, db, "parentlist-multikid")
	defer func() {
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	}()

	phaseID := insertTestPhase(t, db, tenantA, "phase-list-multikid-"+t.Name())

	requestID := insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           phaseID,
		guardianAccountID: &account.ID,
		guardianEmail:     "anna@example.com",
		statusToken:       fmt.Sprintf("tok-multi-%d", time.Now().UnixNano()),
		childFirstName:    "First",
		childLastName:     "Child",
	})
	insertEnrollmentChild(t, db, tenantA, requestID, "Second", "Child", 1, nil)

	results := listByAccount(t, db, account.ID)
	require.Len(t, results, 1)
	require.Len(t, results[0].Children, 2, "both children must be attached to the same envelope")
	// sort_order ASC: First (0), Second (1).
	assert.Equal(t, "First", results[0].Children[0].FirstName)
	assert.Equal(t, "Second", results[0].Children[1].FirstName)
}

func TestEnrollmentRequestRepository_ListByAccount_RejectsZeroAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := parentRepo.NewEnrollmentRequestRepository(db)
	err := tenant.WithAdminTx(testpkg.WithTenantRuntime(t, context.Background(), db), db, func(ctx context.Context, _ bun.Tx) error {
		_, listErr := repo.ListByAccount(ctx, 0)
		return listErr
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account_id must be positive")
}

// --- BackfillGuardianAccountID ---

func TestEnrollmentRequestRepository_Backfill_HappyPath(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.Tenant(t)

	account := testpkg.CreateTestAccount(t, db, "backfill-happy")
	defer func() {
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	}()

	phaseID := insertTestPhase(t, db, tenantA, "phase-backfill-happy-"+t.Name())

	requestID := insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           phaseID,
		guardianAccountID: nil,
		guardianEmail:     "needs-claim-" + strings.ToLower(t.Name()) + "@example.com",
		statusToken:       fmt.Sprintf("tok-backfill-%d", time.Now().UnixNano()),
		childFirstName:    "Claim",
		childLastName:     "Me",
	})

	n := backfill(t, db, account.ID, "needs-claim-"+strings.ToLower(t.Name())+"@example.com")
	assert.Equal(t, 1, n)

	got := readGuardianAccountID(t, db, requestID)
	require.NotNil(t, got)
	assert.Equal(t, account.ID, *got)
}

func TestEnrollmentRequestRepository_Backfill_CaseInsensitive(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.Tenant(t)

	account := testpkg.CreateTestAccount(t, db, "backfill-case")
	defer func() {
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	}()

	phaseID := insertTestPhase(t, db, tenantA, "phase-backfill-case-"+t.Name())

	requestID := insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           phaseID,
		guardianAccountID: nil,
		guardianEmail:     "  Mixed.Case-" + strings.ToLower(t.Name()) + "@Example.COM  ", // stored with whitespace + mixed case
		statusToken:       fmt.Sprintf("tok-case-%d", time.Now().UnixNano()),
		childFirstName:    "Case",
		childLastName:     "Child",
	})

	// Caller normalizes differently: lower-case + no whitespace
	n := backfill(t, db, account.ID, "MIXED.case-"+strings.ToLower(t.Name())+"@example.com")
	assert.Equal(t, 1, n, "trim+lower comparison must match regardless of whitespace and case")

	got := readGuardianAccountID(t, db, requestID)
	require.NotNil(t, got)
	assert.Equal(t, account.ID, *got)
}

func TestEnrollmentRequestRepository_Backfill_SkipsAlreadyStamped(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.Tenant(t)

	owner := testpkg.CreateTestAccount(t, db, "backfill-owner")
	intruder := testpkg.CreateTestAccount(t, db, "backfill-intruder")
	defer func() {
		bg := context.Background()
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id IN (?, ?)", owner.ID, intruder.ID).Exec(bg)
	}()

	phaseID := insertTestPhase(t, db, tenantA, "phase-backfill-skip-"+t.Name())

	requestID := insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           phaseID,
		guardianAccountID: &owner.ID, // already claimed
		guardianEmail:     "shared-" + strings.ToLower(t.Name()) + "@example.com",
		statusToken:       fmt.Sprintf("tok-skip-%d", time.Now().UnixNano()),
		childFirstName:    "Already",
		childLastName:     "Claimed",
	})

	n := backfill(t, db, intruder.ID, "shared-"+strings.ToLower(t.Name())+"@example.com")
	assert.Equal(t, 0, n, "stamped rows must not be reassigned")

	got := readGuardianAccountID(t, db, requestID)
	require.NotNil(t, got)
	assert.Equal(t, owner.ID, *got, "row must still belong to the original owner")
}

func TestEnrollmentRequestRepository_Backfill_DoesNotTouchOtherEmails(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.Tenant(t)

	account := testpkg.CreateTestAccount(t, db, "backfill-other")
	defer func() {
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	}()

	phaseID := insertTestPhase(t, db, tenantA, "phase-backfill-other-"+t.Name())

	myID := insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           phaseID,
		guardianAccountID: nil,
		guardianEmail:     "mine-" + strings.ToLower(t.Name()) + "@example.com",
		statusToken:       fmt.Sprintf("tok-mine-%d", time.Now().UnixNano()),
		childFirstName:    "Mine",
		childLastName:     "Child",
	})
	otherID := insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           phaseID,
		guardianAccountID: nil,
		guardianEmail:     "different-" + strings.ToLower(t.Name()) + "@example.com",
		statusToken:       fmt.Sprintf("tok-other-%d", time.Now().UnixNano()),
		childFirstName:    "Different",
		childLastName:     "Child",
	})

	n := backfill(t, db, account.ID, "mine-"+strings.ToLower(t.Name())+"@example.com")
	assert.Equal(t, 1, n)

	mineGuardian := readGuardianAccountID(t, db, myID)
	require.NotNil(t, mineGuardian)
	assert.Equal(t, account.ID, *mineGuardian)

	otherGuardian := readGuardianAccountID(t, db, otherID)
	assert.Nil(t, otherGuardian, "unrelated email must remain unclaimed")
}

func TestEnrollmentRequestRepository_Backfill_EmptyEmailIsNoop(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.Tenant(t)

	account := testpkg.CreateTestAccount(t, db, "backfill-empty")
	defer func() {
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	}()

	phaseID := insertTestPhase(t, db, tenantA, "phase-backfill-empty-"+t.Name())

	requestID := insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           phaseID,
		guardianAccountID: nil,
		guardianEmail:     "needs-claim-" + strings.ToLower(t.Name()) + "@example.com",
		statusToken:       fmt.Sprintf("tok-empty-%d", time.Now().UnixNano()),
		childFirstName:    "Needs",
		childLastName:     "Claim",
	})

	for _, email := range []string{"", "   "} {
		n := backfill(t, db, account.ID, email)
		assert.Equal(t, 0, n, "empty/whitespace email %q must short-circuit", email)
	}

	got := readGuardianAccountID(t, db, requestID)
	assert.Nil(t, got, "row must remain unclaimed when backfill is called with empty email")
}

func TestEnrollmentRequestRepository_Backfill_CrossTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.Tenant(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantB)

	account := testpkg.CreateTestAccount(t, db, "backfill-cross")
	defer func() {
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	}()

	t1Phase := insertTestPhase(t, db, tenantA, "phase-backfill-cross-t1-"+t.Name())
	t2Phase := insertTestPhase(t, db, tenantB, "phase-backfill-cross-t2-"+t.Name())

	t1ReqID := insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantA,
		phaseID:           t1Phase,
		guardianAccountID: nil,
		guardianEmail:     "global-" + strings.ToLower(t.Name()) + "@example.com",
		statusToken:       fmt.Sprintf("tok-cross-t1-%d", time.Now().UnixNano()),
		childFirstName:    "T1",
		childLastName:     "Child",
	})
	t2ReqID := insertEnrollment(t, db, enrollmentRow{
		tenantID:          tenantB,
		phaseID:           t2Phase,
		guardianAccountID: nil,
		guardianEmail:     "global-" + strings.ToLower(t.Name()) + "@example.com",
		statusToken:       fmt.Sprintf("tok-cross-t2-%d", time.Now().UnixNano()),
		childFirstName:    "T2",
		childLastName:     "Child",
	})

	n := backfill(t, db, account.ID, "global-"+strings.ToLower(t.Name())+"@example.com")
	assert.Equal(t, 2, n, "backfill must claim across tenants under admin tx")

	for _, id := range []int64{t1ReqID, t2ReqID} {
		got := readGuardianAccountID(t, db, id)
		require.NotNil(t, got, "request %d must be claimed", id)
		assert.Equal(t, account.ID, *got)
	}
}

func TestEnrollmentRequestRepository_Backfill_RejectsZeroAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := parentRepo.NewEnrollmentRequestRepository(db)
	err := tenant.WithAdminTx(testpkg.WithTenantRuntime(t, context.Background(), db), db, func(ctx context.Context, _ bun.Tx) error {
		_, bErr := repo.BackfillGuardianAccountID(ctx, 0, "x@example.com")
		return bErr
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account_id must be positive")
}
