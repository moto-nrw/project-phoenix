package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateEmploymentDetailsChangesOnlyTheNamedFields(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	employment := newMemoryEmployment()
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}, Employment: employment})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	modelID := createWorkTimeModel(t, db, testpkg.Tenant(t), "Vollzeit")
	created := createStaff(t, ctx, db, module, "Details", "Staff", schoolmembership.StaffFields{
		StaffNotes: "Notiz", PersonnelNumber: testpkg.StrPtr("P-1"), WorkTimeModelID: &modelID,
		RotationAnchorDate: "2026-01-05", BirthdayDisplayOptOut: true,
	})

	updated, err := module.UpdateEmploymentDetails(ctx, schoolmembership.UpdateEmploymentDetails{
		ID: created.ID, SetEmploymentType: true, EmploymentType: testpkg.StrPtr(schoolmembership.EmploymentTypePartTime),
	})
	require.NoError(t, err)
	assert.Equal(t, schoolmembership.EmploymentTypePartTime, *updated.EmploymentType)
	assert.Equal(t, StaffEmploymentProfile{
		MembershipID: created.ID, StaffNotes: "Notiz", EmploymentType: testpkg.StrPtr(schoolmembership.EmploymentTypePartTime),
		WorkTimeModelID: &modelID, PersonnelNumber: testpkg.StrPtr("P-1"), RotationAnchorDate: "2026-01-05", BirthdayDisplayOptOut: true,
	}, employment.profiles[created.ID], "the personnel number and the rest of the employment half stay")

	_, err = module.UpdateEmploymentDetails(ctx, schoolmembership.UpdateEmploymentDetails{
		ID: created.ID, SetPersonnelNumber: true, PersonnelNumber: testpkg.StrPtr("  P-2  "),
	})
	require.NoError(t, err)
	assert.Equal(t, "P-2", *employment.profiles[created.ID].PersonnelNumber, "the number is trimmed")
	assert.Equal(t, schoolmembership.EmploymentTypePartTime, *employment.profiles[created.ID].EmploymentType, "the employment type stays")

	_, err = module.UpdateEmploymentDetails(ctx, schoolmembership.UpdateEmploymentDetails{
		ID: created.ID, SetPersonnelNumber: true, PersonnelNumber: testpkg.StrPtr("   "),
		SetEmploymentType: true,
	})
	require.NoError(t, err)
	assert.Nil(t, employment.profiles[created.ID].PersonnelNumber, "a blank number clears it")
	assert.Nil(t, employment.profiles[created.ID].EmploymentType, "a nil type clears it")
}

func TestUpdateEmploymentDetailsRefusesInvalidInputAndRetiredStaff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	employment := newMemoryEmployment()
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}, Employment: employment})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	created := createStaff(t, ctx, db, module, "Refused", "Staff", schoolmembership.StaffFields{StaffNotes: "Bleibt"})

	_, err = module.UpdateEmploymentDetails(ctx, schoolmembership.UpdateEmploymentDetails{
		ID: created.ID, SetEmploymentType: true, EmploymentType: testpkg.StrPtr("volunteer"),
	})
	require.ErrorIs(t, err, schoolmembership.ErrInvalidMembership)
	_, err = module.UpdateEmploymentDetails(ctx, schoolmembership.UpdateEmploymentDetails{})
	require.ErrorIs(t, err, schoolmembership.ErrInvalidMembership)

	require.NoError(t, module.DeleteStaff(ctx, created.ID))
	before := employment.profiles[created.ID]
	_, err = module.UpdateEmploymentDetails(ctx, schoolmembership.UpdateEmploymentDetails{
		ID: created.ID, SetPersonnelNumber: true, PersonnelNumber: testpkg.StrPtr("P-9"),
	})
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
	_, err = module.BindSchedule(ctx, schoolmembership.BindSchedule{ID: created.ID, RotationAnchorDate: "2026-02-02"})
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
	assert.Equal(t, before, employment.profiles[created.ID], "nothing is written for a retired member")
}

func TestBindScheduleReplacesOnlyTheBinding(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	employment := newMemoryEmployment()
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}, Employment: employment})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	modelID := createWorkTimeModel(t, db, testpkg.Tenant(t), "Teilzeit")
	created := createStaff(t, ctx, db, module, "Bound", "Staff", schoolmembership.StaffFields{
		StaffNotes: "Notiz", PersonnelNumber: testpkg.StrPtr("P-3"),
		EmploymentType: testpkg.StrPtr(schoolmembership.EmploymentTypeFullTime),
	})

	bound, err := module.BindSchedule(ctx, schoolmembership.BindSchedule{ID: created.ID, WorkTimeModelID: &modelID, RotationAnchorDate: "2026-03-02"})
	require.NoError(t, err)
	assert.Equal(t, modelID, *bound.WorkTimeModelID)
	assert.Equal(t, StaffEmploymentProfile{
		MembershipID: created.ID, StaffNotes: "Notiz", EmploymentType: testpkg.StrPtr(schoolmembership.EmploymentTypeFullTime),
		WorkTimeModelID: &modelID, PersonnelNumber: testpkg.StrPtr("P-3"), RotationAnchorDate: "2026-03-02",
	}, employment.profiles[created.ID])

	_, err = module.BindSchedule(ctx, schoolmembership.BindSchedule{ID: created.ID})
	require.NoError(t, err)
	assert.Nil(t, employment.profiles[created.ID].WorkTimeModelID, "an unbound schedule clears the model")
	assert.Empty(t, employment.profiles[created.ID].RotationAnchorDate)

	_, err = module.BindSchedule(ctx, schoolmembership.BindSchedule{ID: created.ID, RotationAnchorDate: "2026-02-31"})
	require.ErrorIs(t, err, schoolmembership.ErrInvalidMembership)
	zero := int64(0)
	_, err = module.BindSchedule(ctx, schoolmembership.BindSchedule{ID: created.ID, WorkTimeModelID: &zero})
	require.ErrorIs(t, err, schoolmembership.ErrInvalidMembership)
}

func TestWithLiveStaffWritesOnlyForALiveMembership(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}, Employment: newMemoryEmployment()})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	created := createStaff(t, ctx, db, module, "Live", "Staff", schoolmembership.StaffFields{})

	writes := 0
	write := func(context.Context) error { writes++; return nil }
	require.NoError(t, module.WithLiveStaff(ctx, created.ID, write))
	assert.Equal(t, 1, writes)

	require.NoError(t, module.DeleteStaff(ctx, created.ID))
	require.ErrorIs(t, module.WithLiveStaff(ctx, created.ID, write), schoolmembership.ErrStaffNotFound)
	require.ErrorIs(t, module.WithLiveStaff(ctx, 0, write), schoolmembership.ErrInvalidMembership)
	assert.Equal(t, 1, writes, "a retired or invalid membership is never written")
}
