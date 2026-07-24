package users_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setPersonBirthday stamps a birthday onto the fixture person — the
// student fixture creates persons without one, and the enrolled-child
// lookup (#1663) matches on it.
func setPersonBirthday(t *testing.T, db *bun.DB, personID int64, birthday timezone.Date) {
	t.Helper()
	_, err := db.NewRaw(`UPDATE users.persons SET birthday = ? WHERE id = ?`, birthday, personID).
		Exec(context.Background())
	require.NoError(t, err)
}

func TestStudentRepository_ExistsEnrolledByNameAndBirthday(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	var tenantID int64 = 1
	testpkg.EnsureTestTenant(t, db, tenantID)

	student := testpkg.CreateTestStudent(t, db, "Milan", "Eligibilitytest", "2a")
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, db, student.ID) })
	birthday := timezone.NewDate(2018, 5, 20)
	setPersonBirthday(t, db, student.PersonID, birthday)

	repo := repositories.NewFactory(db).Student
	ctx := context.Background()

	// Exact match → true.
	exists, err := repo.ExistsEnrolledByNameAndBirthday(ctx, tenantID, "Milan", "Eligibilitytest", birthday)
	require.NoError(t, err)
	assert.True(t, exists)

	// Case-insensitive + surrounding whitespace → still true (matches
	// how a parent types the same child into the form).
	exists, err = repo.ExistsEnrolledByNameAndBirthday(ctx, tenantID, "  milan ", "ELIGIBILITYTEST", birthday)
	require.NoError(t, err)
	assert.True(t, exists)

	// Different birthday → false.
	exists, err = repo.ExistsEnrolledByNameAndBirthday(ctx, tenantID, "Milan", "Eligibilitytest", timezone.NewDate(2018, 5, 21))
	require.NoError(t, err)
	assert.False(t, exists)

	// Different tenant → false (explicit tenant scoping, no RLS reliance).
	exists, err = repo.ExistsEnrolledByNameAndBirthday(ctx, tenantID+1, "Milan", "Eligibilitytest", birthday)
	require.NoError(t, err)
	assert.False(t, exists)

	// Inactive student → false: the check targets currently enrolled
	// children only.
	_, err = db.NewRaw(`UPDATE users.students SET status = ? WHERE id = ?`, users.StudentStatusInactive, student.ID).
		Exec(ctx)
	require.NoError(t, err)
	exists, err = repo.ExistsEnrolledByNameAndBirthday(ctx, tenantID, "Milan", "Eligibilitytest", birthday)
	require.NoError(t, err)
	assert.False(t, exists)
}
