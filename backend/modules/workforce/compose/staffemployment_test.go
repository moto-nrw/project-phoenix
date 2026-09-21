package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The employment profile moved from School Membership to Workforce with
// #2753; these are the owner's own contracts for it.

func buildStaffEmployment(t *testing.T, db *bun.DB) workforce.StaffEmployments {
	t.Helper()
	employment, err := NewStaffEmployment(db, nil)
	require.NoError(t, err)
	return employment
}

func retireMembership(t *testing.T, db *bun.DB, id int64) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `UPDATE users.staff_school_memberships SET deleted_at = now() WHERE id = ?`, id)
	require.NoError(t, err)
}

func employmentWorkTimeModel(t *testing.T, db *bun.DB, name string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO config.work_time_models (tenant_id, name, rotation_anchor_date)
		VALUES (?, ?, '2026-01-05') RETURNING id`, testpkg.Tenant(t), name).Scan(context.Background(), &id))
	return id
}

func TestStaffEmploymentSavesAndReadsTheProfile(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	employment := buildStaffEmployment(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Employment", "Profile")
	modelID := employmentWorkTimeModel(t, db, "Teilzeit")

	profile := workforce.StaffEmployment{
		MembershipID: staff.ID, StaffNotes: "Notiz", EmploymentType: testpkg.StrPtr("part_time"),
		WorkTimeModelID: &modelID, PersonnelNumber: testpkg.StrPtr("P-1"), RotationAnchorDate: "2026-03-02",
		BirthdayDisplayOptOut: true,
	}
	require.NoError(t, employment.SaveStaffEmployment(ctx, profile))
	stored, err := employment.StaffEmployments(ctx, []int64{staff.ID, 9_223_372_036_854_775_000})
	require.NoError(t, err)
	require.Equal(t, map[int64]workforce.StaffEmployment{staff.ID: profile}, stored, "a membership without a profile is absent")

	cleared := workforce.StaffEmployment{MembershipID: staff.ID}
	require.NoError(t, employment.SaveStaffEmployment(ctx, cleared))
	stored, err = employment.StaffEmployments(ctx, []int64{staff.ID})
	require.NoError(t, err)
	assert.Equal(t, cleared, stored[staff.ID], "a save replaces every field of the profile")

	empty, err := employment.StaffEmployments(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestStaffEmploymentAppendsNotesAndTogglesTheBirthdayOptOut(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	employment := buildStaffEmployment(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Note", "Taker")

	first, err := employment.AppendStaffNotes(ctx, staff.ID, "Erster Absatz")
	require.NoError(t, err)
	assert.Equal(t, "Erster Absatz", first.StaffNotes)
	second, err := employment.AppendStaffNotes(ctx, staff.ID, "Zweiter Absatz")
	require.NoError(t, err)
	assert.Equal(t, "Erster Absatz\nZweiter Absatz", second.StaffNotes)
	stored, err := employment.StaffEmployments(ctx, []int64{staff.ID})
	require.NoError(t, err)
	assert.Equal(t, "Erster Absatz\nZweiter Absatz", stored[staff.ID].StaffNotes)

	require.NoError(t, employment.SetStaffBirthdayDisplayOptOut(ctx, staff.ID, true))
	stored, err = employment.StaffEmployments(ctx, []int64{staff.ID})
	require.NoError(t, err)
	assert.True(t, stored[staff.ID].BirthdayDisplayOptOut)
	require.NoError(t, employment.SetStaffBirthdayDisplayOptOut(ctx, staff.ID, false))
	stored, err = employment.StaffEmployments(ctx, []int64{staff.ID})
	require.NoError(t, err)
	assert.False(t, stored[staff.ID].BirthdayDisplayOptOut)

	_, err = employment.AppendStaffNotes(ctx, 9_223_372_036_854_775_000, "Ins Leere")
	require.ErrorIs(t, err, workforce.ErrStaffEmploymentNotFound)
	require.ErrorIs(t, employment.SetStaffBirthdayDisplayOptOut(ctx, 9_223_372_036_854_775_000, true), workforce.ErrStaffEmploymentNotFound)
}

func TestStaffEmploymentClearsTheWorkTimeModelOfARetiredMembership(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	employment := buildStaffEmployment(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Offboarded", "Colleague")
	modelID := employmentWorkTimeModel(t, db, "Vollzeit")
	require.NoError(t, employment.SaveStaffEmployment(ctx, workforce.StaffEmployment{MembershipID: staff.ID, WorkTimeModelID: &modelID}))
	retireMembership(t, db, staff.ID)

	require.NoError(t, employment.ClearStaffWorkTimeModel(ctx, staff.ID), "offboarding detaches the template after the tombstone")
	stored, err := employment.StaffEmployments(ctx, []int64{staff.ID})
	require.NoError(t, err)
	assert.Nil(t, stored[staff.ID].WorkTimeModelID)
	require.ErrorIs(t, employment.ClearStaffWorkTimeModel(ctx, 9_223_372_036_854_775_000), workforce.ErrStaffEmploymentNotFound)
}

func TestStaffEmploymentRebasesTheAnchorOfLiveAssigneesOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	employment := buildStaffEmployment(t, db)
	capability := buildWorkforce(t, db)
	ctx := testpkg.Ctx(t)
	model, err := capability.CreateWorkTimeModel(ctx, workforce.CreateWorkTimeModel{WorkTimeModelFields: workforce.WorkTimeModelFields{
		Name: "Rotation A/B", RotationLength: 2, RotationAnchorDate: "2026-01-05",
	}})
	require.NoError(t, err)
	first := testpkg.CreateTestStaff(t, db, "First", "Assigned")
	second := testpkg.CreateTestStaff(t, db, "Second", "Assigned")
	offboarded := testpkg.CreateTestStaff(t, db, "Offboarded", "Assigned")
	unassigned := testpkg.CreateTestStaff(t, db, "Not", "Assigned")
	for _, id := range []int64{first.ID, second.ID, offboarded.ID} {
		require.NoError(t, employment.SaveStaffEmployment(ctx, workforce.StaffEmployment{MembershipID: id, WorkTimeModelID: &model.ID}))
	}
	retireMembership(t, db, offboarded.ID)

	bound, err := employment.StaffOnWorkTimeModel(ctx, model.ID)
	require.NoError(t, err)
	assert.Equal(t, []int64{first.ID, second.ID, offboarded.ID}, bound, "the binding is Workforce's, regardless of the membership lifecycle")

	// A template edit stamps its anchor onto the live assignees only; School
	// Membership answers which of them are live.
	_, err = capability.UpdateWorkTimeModel(ctx, workforce.UpdateWorkTimeModel{ID: model.ID, WorkTimeModelFields: workforce.WorkTimeModelFields{
		Name: "Rotation A/B", RotationLength: 2, RotationAnchorDate: "2026-09-07",
	}})
	require.NoError(t, err)
	stored, err := employment.StaffEmployments(ctx, []int64{first.ID, second.ID, offboarded.ID, unassigned.ID})
	require.NoError(t, err)
	assert.Equal(t, "2026-09-07", stored[first.ID].RotationAnchorDate)
	assert.Equal(t, "2026-09-07", stored[second.ID].RotationAnchorDate)
	assert.Empty(t, stored[offboarded.ID].RotationAnchorDate)
	assert.Empty(t, stored[unassigned.ID].RotationAnchorDate)
}

func TestStaffEmploymentKeepsPersonnelNumbersUniqueAmongLiveStaff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	employment := buildStaffEmployment(t, db)
	ctx := testpkg.Ctx(t)
	holder := testpkg.CreateTestStaff(t, db, "Number", "Holder")
	other := testpkg.CreateTestStaff(t, db, "Number", "Clash")
	require.NoError(t, employment.SaveStaffEmployment(ctx, workforce.StaffEmployment{MembershipID: holder.ID, PersonnelNumber: testpkg.StrPtr("P-2000")}))

	err := employment.SaveStaffEmployment(ctx, workforce.StaffEmployment{MembershipID: other.ID, PersonnelNumber: testpkg.StrPtr("P-2000")})
	require.ErrorIs(t, err, workforce.ErrPersonnelNumberTaken)

	retireMembership(t, db, holder.ID)
	require.NoError(t, employment.SaveStaffEmployment(ctx, workforce.StaffEmployment{MembershipID: other.ID, PersonnelNumber: testpkg.StrPtr("P-2000")}),
		"a retired holder frees the number, as the old partial index did")
}

func TestStaffEmploymentIsTenantScoped(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	employment := buildStaffEmployment(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Tenant", "Bound")
	require.NoError(t, employment.SaveStaffEmployment(ctx, workforce.StaffEmployment{MembershipID: staff.ID, StaffNotes: "vertraulich"}))

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	otherCtx := testpkg.TenantContext(otherTenantID)

	visible, err := employment.StaffEmployments(otherCtx, []int64{staff.ID})
	require.NoError(t, err)
	assert.Empty(t, visible, "another school must not read the profile")
	require.ErrorIs(t, employment.SetStaffBirthdayDisplayOptOut(otherCtx, staff.ID, true), workforce.ErrStaffEmploymentNotFound)
	require.ErrorIs(t, employment.ClearStaffWorkTimeModel(otherCtx, staff.ID), workforce.ErrStaffEmploymentNotFound)
	require.ErrorIs(t, employment.SaveStaffEmployment(otherCtx, workforce.StaffEmployment{MembershipID: staff.ID}),
		workforce.ErrStaffEmploymentNotFound, "another school must not take over the profile, and must be told so")

	stored, err := employment.StaffEmployments(ctx, []int64{staff.ID})
	require.NoError(t, err)
	assert.Equal(t, "vertraulich", stored[staff.ID].StaffNotes)
	assert.False(t, stored[staff.ID].BirthdayDisplayOptOut)
}
