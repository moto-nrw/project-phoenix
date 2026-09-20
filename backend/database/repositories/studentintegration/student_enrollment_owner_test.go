package studentintegration_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestEnrollmentStudentCommandsPreserveTenantAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildStudentOwnersModule(t, db)
	ctx := testpkg.Ctx(t)
	person, err := module.CreatePerson(ctx, peopledirectory.CreatePerson{FirstName: "Anna", LastName: "Enrollment"})
	require.NoError(t, err)
	input := peopledirectory.EnrollmentStudent{PersonID: person.ID, SchoolClass: "1a", Status: "pending", EnrolledFrom: "2026-08-01", EnrolledUntil: "2027-07-31"}
	created, err := module.CreateEnrollmentStudent(ctx, input)
	require.NoError(t, err)
	require.Equal(t, testpkg.Tenant(t), created.TenantID)
	require.Equal(t, "2026-08-01", created.EnrolledFrom)
	require.Equal(t, "2027-07-31", created.EnrolledUntil)
	require.Equal(t, person.ID, created.PersonID)
	// Renewal must not carry unrelated state from a previously hydrated row.
	_, err = db.NewRaw("UPDATE users.student_care_profiles SET health_info = ? WHERE membership_id IN (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL)", "unchanged", created.ID).Exec(ctx)
	require.NoError(t, err)
	renewed := input
	renewed.SchoolClass = "2a"
	renewed.Status = "active"
	renewed.EnrolledFrom = "2027-08-01"
	require.NoError(t, module.RenewEnrollmentStudent(ctx, created.ID, renewed))
	var health string
	require.NoError(t, db.NewRaw("SELECT health_info FROM users.student_care_profiles WHERE membership_id IN (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL)", created.ID).Scan(ctx, &health))
	require.Equal(t, "unchanged", health)
	extra := "keep this field"
	require.NoError(t, module.ApplyEnrollmentProfile(ctx, created.ID, peopledirectory.EnrollmentProfilePatch{ExtraInfoSet: true, ExtraInfo: &extra}))
	require.NoError(t, module.ApplyEnrollmentProfile(ctx, created.ID, peopledirectory.EnrollmentProfilePatch{HealthInfoSet: true}))
	var clearedHealth *string
	var storedExtra string
	require.NoError(t, db.NewRaw("SELECT c.health_info, p.extra_info FROM users.student_profiles p JOIN users.student_school_memberships m ON m.tenant_id=p.tenant_id AND m.student_profile_id=p.id AND m.deleted_at IS NULL JOIN users.student_care_profiles c ON c.tenant_id=m.tenant_id AND c.membership_id=m.id WHERE p.id = ?", created.ID).Scan(ctx, &clearedHealth, &storedExtra))
	require.Nil(t, clearedHealth, "explicit NULL must clear the field")
	require.Equal(t, extra, storedExtra, "an unspecified field stays unchanged")

	t.Run("other school", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignCtx := testpkg.Ctx(t)
		_, err := module.CreateEnrollmentStudent(foreignCtx, input)
		require.ErrorIs(t, err, peopledirectory.ErrPersonNotFound)
		require.ErrorIs(t, module.RenewEnrollmentStudent(foreignCtx, created.ID, renewed), peopledirectory.ErrStudentNotFound)
		require.ErrorIs(t, module.ApplyEnrollmentProfile(foreignCtx, created.ID, peopledirectory.EnrollmentProfilePatch{ExtraInfoSet: true}), peopledirectory.ErrStudentNotFound)
	})

	rollback := errors.New("later owner failed")
	var rolledBackID int64
	err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		anotherPerson, createErr := module.CreatePerson(txCtx, peopledirectory.CreatePerson{FirstName: "Rollback", LastName: "Enrollment"})
		require.NoError(t, createErr)
		anotherInput := input
		anotherInput.PersonID = anotherPerson.ID
		another, createErr := module.CreateEnrollmentStudent(txCtx, anotherInput)
		require.NoError(t, createErr)
		rolledBackID = another.ID
		require.NoError(t, module.RenewEnrollmentStudent(txCtx, created.ID, input))
		require.NoError(t, module.ApplyEnrollmentProfile(txCtx, created.ID, peopledirectory.EnrollmentProfilePatch{ExtraInfoSet: true}))
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	rows, err := module.ListStudentsByID(ctx, []int64{rolledBackID, created.ID})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, created.ID, rows[0].ID)
	require.Equal(t, "2a", rows[0].SchoolClass)
	require.NoError(t, db.NewRaw("SELECT extra_info FROM users.student_profiles WHERE id = ?", created.ID).Scan(ctx, &storedExtra))
	require.Equal(t, extra, storedExtra, "profile writes must roll back with the outer workflow")
}

func TestEnrollmentInitialProfileReachesAllOwners(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildStudentOwnersModule(t, db)
	ctx := testpkg.Ctx(t)
	person := testpkg.CreateTestPerson(t, db, "Initial", "Profile")
	group := testpkg.CreateTestEducationGroup(t, db, "Initial profile group")
	extra, health, street := "directory note", "care note", "Schulstraße 1"
	created, err := module.CreateEnrollmentStudent(ctx, peopledirectory.EnrollmentStudent{
		PersonID: person.ID, SchoolClass: "1a", Status: "active",
		InitialProfile: &peopledirectory.EnrollmentProfilePatch{
			ExtraInfoSet: true, ExtraInfo: &extra, HealthInfoSet: true, HealthInfo: &health,
			AddressSet: true, AddressStreet: &street, GroupIDSet: true, GroupID: &group.ID,
			DepartureSet: true, AllowedDepartureModes: map[string][]string{"tue": {"bus"}},
		},
	})
	require.NoError(t, err)
	record, err := module.FindStudentRecord(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, &extra, record.ExtraInfo)
	require.Equal(t, &health, record.HealthInfo)
	require.Equal(t, &street, record.AddressStreet)
	require.Equal(t, &group.ID, record.GroupID)
	var modes string
	require.NoError(t, db.NewRaw(`SELECT care.allowed_departure_modes::text
		FROM users.student_care_profiles care
		JOIN users.student_school_memberships membership ON membership.id = care.membership_id
		WHERE membership.tenant_id = ? AND membership.student_profile_id = ? AND membership.deleted_at IS NULL`,
		testpkg.Tenant(t), created.ID).Scan(ctx, &modes))
	require.JSONEq(t, `{"tue":["bus"]}`, modes)
}

func TestEnrollmentDepartureMirrorsAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildStudentOwnersModule(t, db)
	ctx := testpkg.Ctx(t)
	person, err := module.CreatePerson(ctx, peopledirectory.CreatePerson{FirstName: "Cara", LastName: "Departure"})
	require.NoError(t, err)
	student, err := module.CreateEnrollmentStudent(ctx, peopledirectory.EnrollmentStudent{PersonID: person.ID, SchoolClass: "1a"})
	require.NoError(t, err)
	note := " Nachbarskind "
	patch := peopledirectory.EnrollmentProfilePatch{
		DepartureSet: true, AllowedDepartureModes: map[string][]string{"tue": {"bus", "accompanied"}},
		DepartureCompanionNote: &note,
	}
	require.NoError(t, module.ApplyEnrollmentProfile(ctx, student.ID, patch))
	var status string
	var storedNote *string
	var departure, allowed string
	read := func() {
		t.Helper()
		storedNote = nil
		require.NoError(t, db.NewRaw("SELECT pickup_status, departure_companion_note, departure_days::text, allowed_departure_modes::text FROM users.student_care_profiles WHERE membership_id IN (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL)", student.ID).Scan(ctx, &status, &storedNote, &departure, &allowed))
	}
	read()
	require.Equal(t, "Geht mit anderem Kind", status, "the exclusive bus mirror must not hide accompanied mode")
	require.Equal(t, "Nachbarskind", *storedNote)
	require.JSONEq(t, `{"tue":"bus"}`, departure)
	require.JSONEq(t, `{"tue":["bus","accompanied"]}`, allowed)
	patch.DepartureCompanionNote = nil
	require.ErrorIs(t, module.ApplyEnrollmentProfile(ctx, student.ID, patch), peopledirectory.ErrInvalidStudent)
	patch.DepartureCompanionDays = map[string]bool{"mon": true}
	require.ErrorIs(t, module.ApplyEnrollmentProfile(ctx, student.ID, patch), peopledirectory.ErrInvalidStudent, "Monday cover cannot satisfy Tuesday")
	patch.DepartureCompanionDays["tue"] = true
	require.NoError(t, module.ApplyEnrollmentProfile(ctx, student.ID, patch))
	read()
	require.Nil(t, storedNote)
	t.Run("foreign tenant", func(t *testing.T) {
		testpkg.OwnTenant(t)
		require.ErrorIs(t, module.ApplyEnrollmentProfile(testpkg.Ctx(t), student.ID, patch), peopledirectory.ErrStudentNotFound)
	})
	rollback := errors.New("later graph command failed")
	err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		require.NoError(t, module.ApplyEnrollmentProfile(txCtx, student.ID, peopledirectory.EnrollmentProfilePatch{DepartureSet: true}))
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	read()
	require.Equal(t, "Geht mit anderem Kind", status)
	require.JSONEq(t, `{"tue":["bus","accompanied"]}`, allowed)
	require.NoError(t, module.ApplyEnrollmentProfile(ctx, student.ID, peopledirectory.EnrollmentProfilePatch{DepartureSet: true, DepartureCompanionNote: &note}))
	read()
	require.Equal(t, "Geht alleine nach Hause", status)
	require.Nil(t, storedNote, "an orphan note must be removed")
	require.JSONEq(t, "{}", allowed)
}

func buildStudentOwnersModule(t *testing.T, db *bun.DB, observations ...func(peopleCompose.Observation)) peopledirectory.Capability {
	t.Helper()
	observe := func(peopleCompose.Observation) {}
	if len(observations) > 0 {
		observe = observations[0]
	}
	module, _, err := repositories.NewPeopleDirectoryWithPhotosAndObserver(db, observe)
	require.NoError(t, err)
	return module
}

func buildStudentMembership(t *testing.T, db *bun.DB) schoolmembership.Capability {
	t.Helper()
	membership, err := repositories.NewSchoolMembership(db)
	require.NoError(t, err)
	return membership
}

func otherTenantContext(t *testing.T, db *bun.DB) (context.Context, int64) {
	t.Helper()
	id := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, id)
	return tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), id), id
}
