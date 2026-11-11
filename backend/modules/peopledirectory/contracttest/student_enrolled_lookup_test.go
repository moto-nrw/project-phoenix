package contracttest_test

import (
	"context"
	"testing"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setPersonBirthday stamps a birthday onto the fixture person — the
// student fixture creates persons without one, and the enrolled-child
// lookup (#1663) matches on it.
func setPersonBirthday(t *testing.T, db *bun.DB, personID int64, birthday calendar.Date) {
	t.Helper()
	_, err := db.NewRaw(`UPDATE users.persons SET birthday = ? WHERE id = ?`, birthday, personID).
		Exec(context.Background())
	require.NoError(t, err)
}

func TestStudentRepository_ExistsEnrolledByNameAndBirthday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	student := testpkg.CreateTestStudent(t, db, "Milan", "Eligibilitytest", "2a")
	birthday := calendar.NewDate(2018, 5, 20)
	setPersonBirthday(t, db, student.PersonID, birthday)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Student
	// The parent submission resolves the school before querying eligibility.
	ctx := testpkg.ContextForTenant(testpkg.WithPackageTenantRuntime(context.Background()), tenantID)

	// Exact match → true.
	exists, err := testpkg.TenantQuery4(t, ctx, db, repo.ExistsEnrolledByNameAndBirthday, tenantID, "Milan", "Eligibilitytest", birthday)
	require.NoError(t, err)
	assert.True(t, exists)

	// Case-insensitive + surrounding whitespace → still true (matches
	// how a parent types the same child into the form).
	exists, err = testpkg.TenantQuery4(t, ctx, db, repo.ExistsEnrolledByNameAndBirthday, tenantID, "  milan ", "ELIGIBILITYTEST", birthday)
	require.NoError(t, err)
	assert.True(t, exists)

	// Different birthday → false.
	exists, err = testpkg.TenantQuery4(t, ctx, db, repo.ExistsEnrolledByNameAndBirthday, tenantID, "Milan", "Eligibilitytest", calendar.NewDate(2018, 5, 21))
	require.NoError(t, err)
	assert.False(t, exists)

	// The same identity is absent from a different school under that school's RLS.
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	otherCtx := testpkg.ContextForTenant(ctx, otherTenant)
	exists, err = testpkg.TenantQuery4(t, otherCtx, db, repo.ExistsEnrolledByNameAndBirthday, otherTenant, "Milan", "Eligibilitytest", birthday)
	require.NoError(t, err)
	assert.False(t, exists)

	// Inactive student → false: the check targets currently enrolled
	// children only.
	_, err = db.NewRaw(`UPDATE users.student_school_memberships SET status = ? WHERE student_profile_id = ? AND deleted_at IS NULL`, peopleValues.StudentStatusInactive, student.ID).
		Exec(ctx)
	require.NoError(t, err)
	exists, err = testpkg.TenantQuery4(t, ctx, db, repo.ExistsEnrolledByNameAndBirthday, tenantID, "Milan", "Eligibilitytest", birthday)
	require.NoError(t, err)
	assert.False(t, exists)
}

// TestStudentRepository_ExistsEnrolledByNameAndBirthday_Pending guards the
// #1663 fix: a child approved before its service start date is created with
// status pending until the activation scheduler runs. Such a child is
// already enrolled and must be matched, or it could be re-submitted through
// a new_students phase and duplicated.
func TestStudentRepository_ExistsEnrolledByNameAndBirthday_Pending(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	student := testpkg.CreateTestStudent(t, db, "Pending", "Enrolltest", "1a")
	birthday := calendar.NewDate(2019, 3, 10)
	setPersonBirthday(t, db, student.PersonID, birthday)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Student
	// The parent submission resolves the school before querying eligibility.
	ctx := testpkg.ContextForTenant(testpkg.WithPackageTenantRuntime(context.Background()), tenantID)

	// Flip the student to pending (approved-but-not-yet-activated).
	_, err := db.NewRaw(`UPDATE users.student_school_memberships SET status = ? WHERE student_profile_id = ? AND deleted_at IS NULL`, peopleValues.StudentStatusPending, student.ID).
		Exec(ctx)
	require.NoError(t, err)

	exists, err := testpkg.TenantQuery4(t, ctx, db, repo.ExistsEnrolledByNameAndBirthday, tenantID, "Pending", "Enrolltest", birthday)
	require.NoError(t, err)
	assert.True(t, exists, "a pending (approved, not-yet-activated) student must count as already enrolled")
}

func TestStudentRepository_BatchEnrolledIdentities(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	birthday := calendar.NewDate(2018, 5, 20)
	first := testpkg.CreateTestStudent(t, db, "Lina", "Müller", "2a")
	second := testpkg.CreateTestStudent(t, db, "Mila", "Test", "2a")
	duplicate := testpkg.CreateTestStudent(t, db, "Mila", "Test", "2a")
	for _, student := range []*testpkg.Student{first, second, duplicate} {
		setPersonBirthday(t, db, student.PersonID, birthday)
	}
	_, err := db.NewRaw(`UPDATE users.student_school_memberships SET status = ? WHERE student_profile_id = ? AND deleted_at IS NULL`, peopleValues.StudentStatusPending, first.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Student
	identities := []peopleValues.StudentIdentity{
		{FirstName: "  LINA ", LastName: "MU\u0308LLER", Birthday: birthday},
		{FirstName: "Mila", LastName: "Test", Birthday: birthday},
		{FirstName: "Unknown", LastName: "Test", Birthday: birthday},
		{FirstName: "Lina", LastName: "Müller", Birthday: birthday},
		{FirstName: "Lina", LastName: "Müller", Birthday: birthday.AddDays(1)},
	}
	counter := testpkg.CaptureQueriesForContext(t, db)
	ctx := counter.Context(testpkg.Ctx(t))
	matches, err := testpkg.TenantQuery2(t, ctx, db, repo.FindEnrolledStudentIDsByIdentities, tenantID, identities)
	require.NoError(t, err)
	require.Equal(t, [][]int64{{first.ID}, {second.ID, duplicate.ID}, nil, {first.ID}, nil}, matches)
	require.Len(t, counter.Selects("users.persons"), 1, "all identities share one query")

	counter.Reset()
	empty, err := testpkg.TenantQuery2(t, ctx, db, repo.FindEnrolledStudentIDsByIdentities, tenantID, nil)
	require.NoError(t, err)
	require.Empty(t, empty)
	require.Empty(t, counter.Selects("users.persons"), "an empty batch performs no lookup")
	_, err = repo.FindEnrolledStudentIDsByIdentities(testpkg.Ctx(t), tenantID, identities)
	require.Error(t, err, "a nonempty batch requires the caller's school transaction")

	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	matches, err = testpkg.TenantQuery2(t, testpkg.ContextForTenant(testpkg.Ctx(t), otherTenant), db,
		repo.FindEnrolledStudentIDsByIdentities, otherTenant, identities)
	require.NoError(t, err)
	for _, ids := range matches {
		require.Empty(t, ids, "a foreign school's children never match")
	}

	_, err = testpkg.TenantQuery2(t, testpkg.Ctx(t), db, repo.FindEnrolledStudentIDsByIdentities, otherTenant, identities)
	require.Error(t, err, "a conflicting explicit tenant is refused")
}
