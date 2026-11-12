package compose_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/schoolmembership"
	membershipCompose "github.com/moto-ogs/project-phoenix/modules/schoolmembership/compose"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestTeacherStaffReadPreservesSeparateOwnerFactsAndFilters(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := membershipCompose.New(membershipCompose.Dependencies{DB: db, Observe: func(membershipCompose.Observation) {}, Employment: &rosterEmployment{profiles: make(map[int64]membershipCompose.StaffEmploymentProfile)}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	first := createRosterStaff(t, ctx, db, module, "First", "Member", schoolmembership.StaffFields{
		StaffNotes: "employment profile", PersonnelNumber: testpkg.StrPtr("teacher-staff"),
		EmploymentType:     testpkg.StrPtr(schoolmembership.EmploymentTypePartTime),
		RotationAnchorDate: "2026-03-01", BirthdayDisplayOptOut: true,
	})
	second := createRosterStaff(t, ctx, db, module, "Second", "Member", schoolmembership.StaffFields{})
	retired := createRosterStaff(t, ctx, db, module, "Retired", "Member", schoolmembership.StaffFields{})
	for _, staff := range []schoolmembership.Staff{second, first, retired} {
		_, err := module.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{
			StaffID: staff.ID, Specialization: "Mathematik", Role: "Leitung", Qualifications: "Diplom",
		}})
		require.NoError(t, err)
	}
	_, err = module.CreateExternalCaregiverRecord(ctx, schoolmembership.CreateExternalCaregiverRecord{
		ExternalCaregiverRecordFields: schoolmembership.ExternalCaregiverRecordFields{StaffID: first.ID, Organization: "Partner", ActivityExpertise: "AG"},
	})
	require.NoError(t, err)
	require.NoError(t, module.DeleteStaff(ctx, retired.ID))
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	otherCtx := testpkg.ContextForTenant(testpkg.WithPackageTenantRuntime(context.Background()), otherTenantID)
	foreign := testpkg.CreateTestStaffForTenant(t, db, otherTenantID, "Foreign", "Member")
	_, err = module.CreateTeacher(otherCtx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{StaffID: foreign.ID}})
	require.NoError(t, err)
	plain, err := module.ListTeachers(ctx, schoolmembership.TeacherFilter{SpecializationContains: "mathe"})
	require.NoError(t, err)
	require.Len(t, plain, 3)
	members, err := module.ListStaff(ctx, schoolmembership.StaffFilter{IDs: []int64{first.ID, second.ID, retired.ID}})
	require.NoError(t, err)
	require.Len(t, members, 2)
	byID := make(map[int64]schoolmembership.Staff, len(members))
	for _, member := range members {
		byID[member.ID] = member
	}
	joined, err := module.ListTeachers(ctx, schoolmembership.TeacherFilter{SpecializationContains: "mathe", IncludeStaff: true})
	require.NoError(t, err)
	require.Len(t, joined, len(plain))
	for i, teacher := range joined {
		plainJSON, marshalErr := json.Marshal(plain[i])
		require.NoError(t, marshalErr)
		joinedJSON, marshalErr := json.Marshal(teacher)
		require.NoError(t, marshalErr)
		require.JSONEq(t, string(plainJSON), string(joinedJSON), "embedded metadata must not change the teacher wire DTO")
		if member, found := byID[teacher.StaffID]; found {
			require.NotNil(t, teacher.Staff)
			require.Equal(t, member, *teacher.Staff)
		} else {
			require.Equal(t, retired.ID, teacher.StaffID)
			require.Nil(t, teacher.Staff, "a retired member is not roster metadata")
		}
	}
	require.NoError(t, module.DeleteTeacher(ctx, joined[0].ID))
	for _, includeDeleted := range []bool{false, true} {
		rows, listErr := module.ListTeachers(ctx, schoolmembership.TeacherFilter{IDs: []int64{joined[0].ID}, IncludeStaff: true, IncludeDeleted: includeDeleted})
		require.NoError(t, listErr)
		if includeDeleted {
			require.Len(t, rows, 1)
			require.True(t, rows[0].IsDeleted())
		} else {
			require.Empty(t, rows)
		}
	}
	rows, err := module.ListTeachers(ctx, schoolmembership.TeacherFilter{IDs: []int64{}, IncludeStaff: true})
	require.NoError(t, err)
	require.Empty(t, rows)
}

// rosterEmployment is the separate Workforce contract, never a joined foreign read.
type rosterEmployment struct {
	profiles map[int64]membershipCompose.StaffEmploymentProfile
}

func (e *rosterEmployment) StaffEmployments(_ context.Context, ids []int64) (map[int64]membershipCompose.StaffEmploymentProfile, error) {
	result := make(map[int64]membershipCompose.StaffEmploymentProfile, len(ids))
	for _, id := range ids {
		if profile, ok := e.profiles[id]; ok {
			result[id] = profile
		}
	}
	return result, nil
}
func (e *rosterEmployment) SaveStaffEmployment(_ context.Context, profile membershipCompose.StaffEmploymentProfile) error {
	e.profiles[profile.MembershipID] = profile
	return nil
}
func createRosterStaff(t *testing.T, ctx context.Context, db *bun.DB, module schoolmembership.Owner, first, last string, fields schoolmembership.StaffFields) schoolmembership.Staff {
	t.Helper()
	fields.PersonID = testpkg.CreateTestPerson(t, db, first, last).ID
	staff, err := module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: fields})
	require.NoError(t, err)
	return staff
}
