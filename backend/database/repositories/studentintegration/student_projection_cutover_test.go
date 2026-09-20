package studentintegration_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStudentDirectoryUsesLiveMembershipIdentity(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Live", "Membership", "1a")
	directory := buildStudentOwnersModule(t, db)
	membership := buildStudentMembership(t, db)
	_, err := db.NewRaw("UPDATE users.student_school_memberships SET deleted_at = NOW() WHERE tenant_id = ? AND student_profile_id = ?", testpkg.Tenant(t), student.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = directory.FindStudentRecord(ctx, student.ID)
	require.ErrorIs(t, err, peopledirectory.ErrStudentNotFound)
	memberID, err := membership.Enroll(ctx, schoolmembership.StudentEnrollment{StudentID: student.ID, SchoolClass: "2a", Status: "active"})
	require.NoError(t, err)
	require.NotEqual(t, student.ID, memberID, "membership identity must not be assumed to equal the public profile ID")
	_, err = directory.FindStudentRecord(ctx, student.ID)
	require.ErrorIs(t, err, peopledirectory.ErrStudentNotFound, "an incomplete membership without care is not projected")
	care, err := careCompose.NewStudentProfiles(db, func(careCompose.Observation) {})
	require.NoError(t, err)
	require.NoError(t, care.SaveStudentCareProfile(ctx, careplan.StudentCareProfile{MembershipID: memberID}, nil))
	record, err := directory.FindStudentRecord(ctx, student.ID)
	require.NoError(t, err)
	require.Equal(t, "2a", record.SchoolClass, "deleted historical membership must not duplicate or shadow the current membership")
	records, err := directory.ListStudentRecordsByID(ctx, []int64{student.ID})
	require.NoError(t, err)
	require.Len(t, records, 1)
}

func TestStudentOwnersWorkWithoutRollbackView(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	person := testpkg.CreateTestPerson(t, db, "No", "RollbackView")
	var retired bool
	require.NoError(t, db.NewRaw("SELECT to_regclass('users.students') IS NULL AND to_regclass('users.students_legacy') IS NULL").Scan(ctx, &retired))
	require.True(t, retired)
	directory := buildStudentOwnersModule(t, db)
	record, err := directory.CreateStudent(ctx, peopledirectory.StudentWrite{Record: peopledirectory.StudentRecord{PersonID: person.ID, SchoolClass: "1a", Status: "active"}})
	require.NoError(t, err)
	repo := repositories.NewStudentRepository(db)
	rows, err := repo.List(ctx, map[string]any{"id": record.ID})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	count, err := repo.CountWithOptions(ctx, nil)
	require.NoError(t, err)
	require.Positive(t, count)
	record.SchoolClass = "2a"
	_, err = directory.UpdateStudent(ctx, peopledirectory.StudentWrite{Record: record})
	require.NoError(t, err)
	stored, err := repo.FindByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, "2a", stored.SchoolClass)
	require.NoError(t, directory.DeleteStudentRecord(ctx, record.ID))
	_, err = repo.FindByID(ctx, record.ID)
	require.Error(t, err)
}

func TestRollbackViewSupportsOwnerCreatedStudents(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.RestoreStudentCompatibilityBeforeContract(t, db)
	ctx := testpkg.Ctx(t)
	person := testpkg.CreateTestPerson(t, db, "Rollback", "NewOwner")
	directory := buildStudentOwnersModule(t, db)
	record, err := directory.CreateStudent(ctx, peopledirectory.StudentWrite{Record: peopledirectory.StudentRecord{PersonID: person.ID, SchoolClass: "1a", Status: "active"}})
	require.NoError(t, err)
	var archived int
	require.NoError(t, db.NewRaw("SELECT count(*) FROM users.students_legacy WHERE tenant_id = ? AND id = ?", testpkg.Tenant(t), record.ID).Scan(ctx, &archived))
	require.Zero(t, archived, "the current application must not recreate the Guardian archive")
	_, err = db.NewRaw("UPDATE users.students SET extra_info = 'rollback write', sick = true WHERE tenant_id = ? AND id = ?", testpkg.Tenant(t), record.ID).Exec(ctx)
	require.NoError(t, err)
	stored, err := directory.FindStudentRecord(ctx, record.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.ExtraInfo)
	require.Equal(t, "rollback write", *stored.ExtraInfo)
	require.NotNil(t, stored.Sick)
	require.True(t, *stored.Sick)
	require.NoError(t, directory.DeleteStudentRecord(ctx, record.ID))
	// A previous image may re-enroll the same person after a current-image
	// deletion. Its archive row must not block the compatibility INSERT.
	var replacementID int64
	require.NoError(t, db.NewRaw("INSERT INTO users.students (tenant_id, person_id, school_class) VALUES (?, ?, '2a') RETURNING id", testpkg.Tenant(t), person.ID).Scan(ctx, &replacementID))
	replacement, err := directory.FindStudentRecord(ctx, replacementID)
	require.NoError(t, err)
	require.Equal(t, "2a", replacement.SchoolClass)
}
