package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The fixed calendar dates every date assertion here uses. Deriving them from
// the wall clock would make the expectations drift across midnight.
const (
	anchorDate     = "2026-03-01"
	rebasedAnchor  = "2026-09-07"
	guestStartDate = "2026-03-01"
	guestEndDate   = "2026-03-31"
	insideWindow   = "2026-03-15"
	afterWindow    = "2026-04-01"
)

func buildModule(t *testing.T, db *bun.DB, observations ...func(Observation)) *schoolmembership.Module {
	t.Helper()
	observe := func(Observation) {}
	if len(observations) > 0 {
		observe = observations[0]
	}
	module, err := New(Dependencies{DB: db, Observe: observe})
	require.NoError(t, err)
	return module
}

func otherTenantContext(t *testing.T, db *bun.DB) (context.Context, int64) {
	t.Helper()
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	return tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), otherTenantID), otherTenantID
}

// createStaff gives the new staff member a person of their own and creates the
// membership row through the module under test.
func createStaff(t *testing.T, ctx context.Context, db *bun.DB, module *schoolmembership.Module, firstName, lastName string, fields schoolmembership.StaffFields) schoolmembership.Staff {
	t.Helper()
	person := testpkg.CreateTestPerson(t, db, firstName, lastName)
	fields.PersonID = person.ID
	staff, err := module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: fields})
	require.NoError(t, err)
	return staff
}

// createWorkTimeModel inserts a work-time template directly: the fixture
// catalog has none, and the module does not own that table.
func createWorkTimeModel(t *testing.T, db *bun.DB, tenantID int64, name string) int64 {
	t.Helper()
	var id int64
	err := db.NewRaw(
		`INSERT INTO config.work_time_models (tenant_id, name, rotation_length, rotation_anchor_date)
		 VALUES (?, ?, 1, ?::date) RETURNING id`,
		tenantID, name, anchorDate,
	).Scan(context.Background(), &id)
	require.NoError(t, err)
	return id
}

func TestModuleRunsTheStaffLifecycleInOneTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)

	created := createStaff(t, ctx, db, module, "Mia", "Membership", schoolmembership.StaffFields{
		StaffNotes:         "Erster Absatz",
		EmploymentType:     testpkg.StrPtr(schoolmembership.EmploymentTypePartTime),
		PersonnelNumber:    testpkg.StrPtr("P-1000"),
		RotationAnchorDate: anchorDate,
	})
	assert.Positive(t, created.ID)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID)
	assert.Equal(t, anchorDate, created.RotationAnchorDate)
	require.NotNil(t, created.EmploymentType)
	assert.Equal(t, schoolmembership.EmploymentTypePartTime, *created.EmploymentType)
	assert.False(t, created.IsDeleted())

	found, err := module.FindStaff(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.PersonID, found.PersonID)

	byPerson, err := module.FindStaffByPerson(ctx, created.PersonID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, byPerson.ID)

	updated, err := module.UpdateStaff(ctx, schoolmembership.UpdateStaff{ID: created.ID, StaffFields: schoolmembership.StaffFields{
		PersonID:              created.PersonID,
		StaffNotes:            "Ersetzt",
		EmploymentType:        testpkg.StrPtr(schoolmembership.EmploymentTypeFullTime),
		BirthdayDisplayOptOut: true,
	}})
	require.NoError(t, err)
	assert.Equal(t, "Ersetzt", updated.StaffNotes)
	assert.True(t, updated.BirthdayDisplayOptOut)
	assert.Empty(t, updated.RotationAnchorDate, "the update writes every field it owns")
	assert.Nil(t, updated.PersonnelNumber)

	listed, err := module.ListStaff(ctx, schoolmembership.StaffFilter{})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)

	require.NoError(t, module.DeleteStaff(ctx, created.ID))
	_, err = module.FindStaff(ctx, created.ID)
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
	require.ErrorIs(t, module.DeleteStaff(ctx, created.ID), schoolmembership.ErrStaffNotFound)

	live, err := module.ListStaff(ctx, schoolmembership.StaffFilter{})
	require.NoError(t, err)
	assert.Empty(t, live, "a soft-deleted staff member stays out of the default listing")

	withDeleted, err := module.ListStaff(ctx, schoolmembership.StaffFilter{IncludeDeleted: true})
	require.NoError(t, err)
	require.Len(t, withDeleted, 1)
	assert.True(t, withDeleted[0].IsDeleted())

	_, err = module.FindStaffByPerson(ctx, created.PersonID)
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
}

func TestModuleFiltersStaffListings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	modelID := createWorkTimeModel(t, db, testpkg.Tenant(t), "Teilzeit 30h")

	assigned := createStaff(t, ctx, db, module, "Assigned", "Model", schoolmembership.StaffFields{WorkTimeModelID: &modelID})
	unassigned := createStaff(t, ctx, db, module, "Without", "Model", schoolmembership.StaffFields{})

	byID, err := module.ListStaff(ctx, schoolmembership.StaffFilter{IDs: []int64{assigned.ID}})
	require.NoError(t, err)
	require.Len(t, byID, 1)
	assert.Equal(t, assigned.ID, byID[0].ID)

	byPerson, err := module.ListStaff(ctx, schoolmembership.StaffFilter{PersonIDs: []int64{unassigned.PersonID}})
	require.NoError(t, err)
	require.Len(t, byPerson, 1)
	assert.Equal(t, unassigned.ID, byPerson[0].ID)

	byModel, err := module.ListStaff(ctx, schoolmembership.StaffFilter{WorkTimeModelID: &modelID})
	require.NoError(t, err)
	require.Len(t, byModel, 1)
	assert.Equal(t, assigned.ID, byModel[0].ID)

	empty, err := module.ListStaff(ctx, schoolmembership.StaffFilter{IDs: []int64{}})
	require.NoError(t, err)
	assert.Empty(t, empty, "an empty but present ID filter matches nothing rather than everything")

	emptyPersons, err := module.ListStaff(ctx, schoolmembership.StaffFilter{PersonIDs: []int64{}})
	require.NoError(t, err)
	assert.Empty(t, emptyPersons)

	all, err := module.ListStaff(ctx, schoolmembership.StaffFilter{})
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestModuleLocksStaffInsideTheCallersTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := createStaff(t, ctx, db, module, "Locked", "Row", schoolmembership.StaffFields{})

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		locked, findErr := module.FindStaffForMutation(txCtx, staff.ID)
		require.NoError(t, findErr)
		assert.Equal(t, staff.ID, locked.ID)

		_, missingErr := module.FindStaffForMutation(txCtx, 9_223_372_036_854_775_000)
		require.ErrorIs(t, missingErr, schoolmembership.ErrStaffNotFound)
		return nil
	})
	require.NoError(t, err)
}

func TestModuleAppendsNotesAndTogglesTheBirthdayOptOut(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := createStaff(t, ctx, db, module, "Note", "Taker", schoolmembership.StaffFields{})

	first, err := module.AppendStaffNotes(ctx, staff.ID, "Erster Absatz")
	require.NoError(t, err)
	assert.Equal(t, "Erster Absatz", first.StaffNotes)

	second, err := module.AppendStaffNotes(ctx, staff.ID, "Zweiter Absatz")
	require.NoError(t, err)
	assert.Equal(t, "Erster Absatz\nZweiter Absatz", second.StaffNotes)

	stored, err := module.FindStaff(ctx, staff.ID)
	require.NoError(t, err)
	assert.Equal(t, "Erster Absatz\nZweiter Absatz", stored.StaffNotes)

	require.NoError(t, module.SetBirthdayDisplayOptOut(ctx, staff.ID, true))
	stored, err = module.FindStaff(ctx, staff.ID)
	require.NoError(t, err)
	assert.True(t, stored.BirthdayDisplayOptOut)

	require.NoError(t, module.SetBirthdayDisplayOptOut(ctx, staff.ID, false))
	stored, err = module.FindStaff(ctx, staff.ID)
	require.NoError(t, err)
	assert.False(t, stored.BirthdayDisplayOptOut)

	_, err = module.AppendStaffNotes(ctx, 9_223_372_036_854_775_000, "Ins Leere")
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
}

func TestModuleClearsTheWorkTimeModelOfASoftDeletedStaffMember(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	modelID := createWorkTimeModel(t, db, testpkg.Tenant(t), "Vollzeit 40h")
	staff := createStaff(t, ctx, db, module, "Offboarded", "Colleague", schoolmembership.StaffFields{WorkTimeModelID: &modelID})

	require.NoError(t, module.DeleteStaff(ctx, staff.ID))
	require.NoError(t, module.ClearWorkTimeModel(ctx, staff.ID),
		"offboarding detaches the template after the tombstone")

	retained, err := module.ListStaff(ctx, schoolmembership.StaffFilter{IDs: []int64{staff.ID}, IncludeDeleted: true})
	require.NoError(t, err)
	require.Len(t, retained, 1)
	assert.Nil(t, retained[0].WorkTimeModelID)
	assert.True(t, retained[0].IsDeleted())

	require.ErrorIs(t, module.ClearWorkTimeModel(ctx, 9_223_372_036_854_775_000), schoolmembership.ErrStaffNotFound)
}

func TestModuleRebasesTheWorkTimeModelAnchorOfLiveStaffOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	modelID := createWorkTimeModel(t, db, testpkg.Tenant(t), "Rotation A/B")

	first := createStaff(t, ctx, db, module, "First", "Assigned", schoolmembership.StaffFields{WorkTimeModelID: &modelID})
	second := createStaff(t, ctx, db, module, "Second", "Assigned", schoolmembership.StaffFields{WorkTimeModelID: &modelID})
	offboarded := createStaff(t, ctx, db, module, "Offboarded", "Assigned", schoolmembership.StaffFields{WorkTimeModelID: &modelID})
	unassigned := createStaff(t, ctx, db, module, "Not", "Assigned", schoolmembership.StaffFields{})
	require.NoError(t, module.DeleteStaff(ctx, offboarded.ID))

	rebased, err := module.RebaseWorkTimeModelAnchor(ctx, modelID, rebasedAnchor)
	require.NoError(t, err)
	assert.Equal(t, []int64{first.ID, second.ID}, rebased, "only live assignees, in ascending ID order")

	stamped, err := module.FindStaff(ctx, second.ID)
	require.NoError(t, err)
	assert.Equal(t, rebasedAnchor, stamped.RotationAnchorDate)

	untouched, err := module.FindStaff(ctx, unassigned.ID)
	require.NoError(t, err)
	assert.Empty(t, untouched.RotationAnchorDate)

	none, err := module.RebaseWorkTimeModelAnchor(ctx, createWorkTimeModel(t, db, testpkg.Tenant(t), "Ohne Zuordnung"), rebasedAnchor)
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestModuleReportsMembershipConflicts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)

	staff := createStaff(t, ctx, db, module, "Conflict", "Holder", schoolmembership.StaffFields{
		PersonnelNumber: testpkg.StrPtr("P-2000"),
	})

	_, err := module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: staff.PersonID}})
	require.ErrorIs(t, err, schoolmembership.ErrStaffPersonConflict)

	secondPerson := testpkg.CreateTestPerson(t, db, "Second", "Number")
	_, err = module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{
		PersonID: secondPerson.ID, PersonnelNumber: testpkg.StrPtr("P-2000"),
	}})
	require.ErrorIs(t, err, schoolmembership.ErrPersonnelNumberConflict)

	_, err = module.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{StaffID: staff.ID}})
	require.NoError(t, err)
	_, err = module.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{StaffID: staff.ID}})
	require.ErrorIs(t, err, schoolmembership.ErrTeacherStaffConflict)

	_, err = module.CreateGuest(ctx, schoolmembership.CreateGuest{GuestFields: schoolmembership.GuestFields{
		StaffID: staff.ID, ActivityExpertise: "Zirkus",
	}})
	require.NoError(t, err)
	_, err = module.CreateGuest(ctx, schoolmembership.CreateGuest{GuestFields: schoolmembership.GuestFields{
		StaffID: staff.ID, ActivityExpertise: "Theater",
	}})
	require.ErrorIs(t, err, schoolmembership.ErrGuestStaffConflict)
}

func TestModuleRunsTheTeacherLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := createStaff(t, ctx, db, module, "Teacher", "Profile", schoolmembership.StaffFields{})
	other := createStaff(t, ctx, db, module, "Without", "Profile", schoolmembership.StaffFields{})

	created, err := module.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{
		StaffID: staff.ID, Specialization: "Mathematik", Role: "Gruppenleitung", Qualifications: "Diplom",
	}})
	require.NoError(t, err)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID)

	found, err := module.FindTeacher(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Mathematik", found.Specialization)

	byStaff, err := module.FindTeacherByStaff(ctx, staff.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, byStaff.ID)

	_, err = module.FindTeacherByStaff(ctx, other.ID)
	require.ErrorIs(t, err, schoolmembership.ErrTeacherNotFound,
		"a staff member without a pedagogical profile is not a teacher")

	updated, err := module.UpdateTeacher(ctx, schoolmembership.UpdateTeacher{ID: created.ID, TeacherFields: schoolmembership.TeacherFields{
		StaffID: staff.ID, Specialization: "Sport", Role: "Vertretung",
	}})
	require.NoError(t, err)
	assert.Equal(t, "Sport", updated.Specialization)
	assert.Empty(t, updated.Qualifications)

	require.NoError(t, module.DeleteTeacher(ctx, created.ID))
	_, err = module.FindTeacher(ctx, created.ID)
	require.ErrorIs(t, err, schoolmembership.ErrTeacherNotFound)
	require.ErrorIs(t, module.DeleteTeacher(ctx, created.ID), schoolmembership.ErrTeacherNotFound)

	live, err := module.ListTeachers(ctx, schoolmembership.TeacherFilter{})
	require.NoError(t, err)
	assert.Empty(t, live, "a soft-deleted teacher leaves the default listing")

	withDeleted, err := module.ListTeachers(ctx, schoolmembership.TeacherFilter{IncludeDeleted: true})
	require.NoError(t, err)
	require.Len(t, withDeleted, 1)
	assert.True(t, withDeleted[0].IsDeleted())
}

func TestModuleFiltersTeacherListings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)

	mathStaff := createStaff(t, ctx, db, module, "Math", "Teacher", schoolmembership.StaffFields{})
	sportStaff := createStaff(t, ctx, db, module, "Sport", "Teacher", schoolmembership.StaffFields{})

	math, err := module.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{
		StaffID: mathStaff.ID, Specialization: "Mathematik", Role: "Gruppenleitung", Qualifications: "Diplom",
	}})
	require.NoError(t, err)
	sport, err := module.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{
		StaffID: sportStaff.ID, Specialization: "Sport", Role: "Betreuung",
	}})
	require.NoError(t, err)

	byStaff, err := module.ListTeachers(ctx, schoolmembership.TeacherFilter{StaffIDs: []int64{sportStaff.ID}})
	require.NoError(t, err)
	require.Len(t, byStaff, 1)
	assert.Equal(t, sport.ID, byStaff[0].ID)

	exact, err := module.ListTeachers(ctx, schoolmembership.TeacherFilter{Specialization: "mathematik"})
	require.NoError(t, err)
	require.Len(t, exact, 1, "the exact specialization match ignores case")
	assert.Equal(t, math.ID, exact[0].ID)

	partial, err := module.ListTeachers(ctx, schoolmembership.TeacherFilter{Specialization: "Mathe"})
	require.NoError(t, err)
	assert.Empty(t, partial, "the exact match is not a substring match")

	contains, err := module.ListTeachers(ctx, schoolmembership.TeacherFilter{SpecializationContains: "athemat"})
	require.NoError(t, err)
	require.Len(t, contains, 1)
	assert.Equal(t, math.ID, contains[0].ID)

	byRole, err := module.ListTeachers(ctx, schoolmembership.TeacherFilter{RoleContains: "betreu"})
	require.NoError(t, err)
	require.Len(t, byRole, 1)
	assert.Equal(t, sport.ID, byRole[0].ID)

	qualified, err := module.ListTeachers(ctx, schoolmembership.TeacherFilter{HasQualifications: boolPtr(true)})
	require.NoError(t, err)
	require.Len(t, qualified, 1)
	assert.Equal(t, math.ID, qualified[0].ID)

	unqualified, err := module.ListTeachers(ctx, schoolmembership.TeacherFilter{HasQualifications: boolPtr(false)})
	require.NoError(t, err)
	require.Len(t, unqualified, 1)
	assert.Equal(t, sport.ID, unqualified[0].ID)

	empty, err := module.ListTeachers(ctx, schoolmembership.TeacherFilter{StaffIDs: []int64{}})
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestModuleRunsTheGuestLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := createStaff(t, ctx, db, module, "Guest", "Instructor", schoolmembership.StaffFields{})

	created, err := module.CreateGuest(ctx, schoolmembership.CreateGuest{GuestFields: schoolmembership.GuestFields{
		StaffID: staff.ID, ActivityExpertise: "Zirkus", Organization: "Circus e.V.",
		ContactEmail: "gast@example.com", ContactPhone: "+49 221 1234567",
		StartDate: guestStartDate, EndDate: guestEndDate, Notes: "Kommt dienstags",
	}})
	require.NoError(t, err)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID)
	assert.Equal(t, guestStartDate, created.StartDate)
	assert.Equal(t, guestEndDate, created.EndDate)

	found, err := module.FindGuest(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Circus e.V.", found.Organization)

	byStaff, err := module.FindGuestByStaff(ctx, staff.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, byStaff.ID)

	updated, err := module.UpdateGuest(ctx, schoolmembership.UpdateGuest{ID: created.ID, GuestFields: schoolmembership.GuestFields{
		StaffID: staff.ID, ActivityExpertise: "Theater", Organization: "Circus e.V.",
	}})
	require.NoError(t, err)
	assert.Equal(t, "Theater", updated.ActivityExpertise)
	assert.Empty(t, updated.StartDate, "clearing the window writes NULL rather than keeping the old date")

	require.NoError(t, module.DeleteGuest(ctx, created.ID))
	_, err = module.FindGuest(ctx, created.ID)
	require.ErrorIs(t, err, schoolmembership.ErrGuestNotFound)
	require.ErrorIs(t, module.DeleteGuest(ctx, created.ID), schoolmembership.ErrGuestNotFound,
		"guests carry no tombstone, the delete is final")

	remaining, err := module.ListGuests(ctx, schoolmembership.GuestFilter{})
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

func TestModuleFiltersGuestListings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)

	boundStaff := createStaff(t, ctx, db, module, "Bound", "Guest", schoolmembership.StaffFields{})
	openStaff := createStaff(t, ctx, db, module, "Open", "Guest", schoolmembership.StaffFields{})

	bound, err := module.CreateGuest(ctx, schoolmembership.CreateGuest{GuestFields: schoolmembership.GuestFields{
		StaffID: boundStaff.ID, ActivityExpertise: "Zirkus", Organization: "Circus e.V.",
		StartDate: guestStartDate, EndDate: guestEndDate,
	}})
	require.NoError(t, err)
	open, err := module.CreateGuest(ctx, schoolmembership.CreateGuest{GuestFields: schoolmembership.GuestFields{
		StaffID: openStaff.ID, ActivityExpertise: "Theater", Organization: "Buehne gGmbH",
	}})
	require.NoError(t, err)

	inWindow, err := module.ListGuests(ctx, schoolmembership.GuestFilter{ActiveOn: insideWindow})
	require.NoError(t, err)
	assert.Len(t, inWindow, 2, "an open-ended guest is active on every day")

	afterEnd, err := module.ListGuests(ctx, schoolmembership.GuestFilter{ActiveOn: afterWindow})
	require.NoError(t, err)
	require.Len(t, afterEnd, 1)
	assert.Equal(t, open.ID, afterEnd[0].ID)

	byStaff, err := module.ListGuests(ctx, schoolmembership.GuestFilter{StaffIDs: []int64{boundStaff.ID}})
	require.NoError(t, err)
	require.Len(t, byStaff, 1)
	assert.Equal(t, bound.ID, byStaff[0].ID)

	byOrganization, err := module.ListGuests(ctx, schoolmembership.GuestFilter{OrganizationContains: "circus"})
	require.NoError(t, err)
	require.Len(t, byOrganization, 1)
	assert.Equal(t, bound.ID, byOrganization[0].ID)

	byExpertise, err := module.ListGuests(ctx, schoolmembership.GuestFilter{ExpertiseContains: "heate"})
	require.NoError(t, err)
	require.Len(t, byExpertise, 1)
	assert.Equal(t, open.ID, byExpertise[0].ID)

	// The organization filter reads the column, not the trimmed value: a
	// guest whose organization was never set carries SQL NULL.
	noOrganizationStaff := createStaff(t, ctx, db, module, "Anonymous", "Guest", schoolmembership.StaffFields{})
	var withoutOrganizationID int64
	require.NoError(t, db.NewRaw(
		`INSERT INTO users.guests (tenant_id, staff_id, activity_expertise, organization)
		 VALUES (?, ?, ?, NULL) RETURNING id`,
		testpkg.Tenant(t), noOrganizationStaff.ID, "Kochen",
	).Scan(context.Background(), &withoutOrganizationID))

	withOrganization, err := module.ListGuests(ctx, schoolmembership.GuestFilter{HasOrganization: boolPtr(true)})
	require.NoError(t, err)
	assert.Len(t, withOrganization, 2)

	withoutOrganization, err := module.ListGuests(ctx, schoolmembership.GuestFilter{HasOrganization: boolPtr(false)})
	require.NoError(t, err)
	require.Len(t, withoutOrganization, 1)
	assert.Equal(t, withoutOrganizationID, withoutOrganization[0].ID)

	empty, err := module.ListGuests(ctx, schoolmembership.GuestFilter{IDs: []int64{}})
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestModuleTenantIsolationHidesAnotherTenantsMemberships(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)

	staff := createStaff(t, ctx, db, module, "Isolated", "Staff", schoolmembership.StaffFields{})
	teacher, err := module.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{
		StaffID: staff.ID, Specialization: "Mathematik",
	}})
	require.NoError(t, err)
	guest, err := module.CreateGuest(ctx, schoolmembership.CreateGuest{GuestFields: schoolmembership.GuestFields{
		StaffID: staff.ID, ActivityExpertise: "Zirkus",
	}})
	require.NoError(t, err)

	otherCtx, _ := otherTenantContext(t, db)

	_, err = module.FindStaff(otherCtx, staff.ID)
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
	_, err = module.FindStaffByPerson(otherCtx, staff.PersonID)
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
	_, err = module.FindTeacher(otherCtx, teacher.ID)
	require.ErrorIs(t, err, schoolmembership.ErrTeacherNotFound)
	_, err = module.FindTeacherByStaff(otherCtx, staff.ID)
	require.ErrorIs(t, err, schoolmembership.ErrTeacherNotFound)
	_, err = module.FindGuest(otherCtx, guest.ID)
	require.ErrorIs(t, err, schoolmembership.ErrGuestNotFound)
	_, err = module.FindGuestByStaff(otherCtx, staff.ID)
	require.ErrorIs(t, err, schoolmembership.ErrGuestNotFound)

	listedStaff, err := module.ListStaff(otherCtx, schoolmembership.StaffFilter{IncludeDeleted: true})
	require.NoError(t, err)
	assert.Empty(t, listedStaff)
	listedTeachers, err := module.ListTeachers(otherCtx, schoolmembership.TeacherFilter{IncludeDeleted: true})
	require.NoError(t, err)
	assert.Empty(t, listedTeachers)
	listedGuests, err := module.ListGuests(otherCtx, schoolmembership.GuestFilter{})
	require.NoError(t, err)
	assert.Empty(t, listedGuests)

	_, err = module.UpdateStaff(otherCtx, schoolmembership.UpdateStaff{ID: staff.ID, StaffFields: schoolmembership.StaffFields{PersonID: staff.PersonID}})
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
	_, err = module.UpdateTeacher(otherCtx, schoolmembership.UpdateTeacher{ID: teacher.ID, TeacherFields: schoolmembership.TeacherFields{StaffID: staff.ID}})
	require.ErrorIs(t, err, schoolmembership.ErrTeacherNotFound)
	_, err = module.UpdateGuest(otherCtx, schoolmembership.UpdateGuest{ID: guest.ID, GuestFields: schoolmembership.GuestFields{
		StaffID: staff.ID, ActivityExpertise: "Zirkus",
	}})
	require.ErrorIs(t, err, schoolmembership.ErrGuestNotFound)

	require.ErrorIs(t, module.DeleteStaff(otherCtx, staff.ID), schoolmembership.ErrStaffNotFound)
	require.ErrorIs(t, module.DeleteTeacher(otherCtx, teacher.ID), schoolmembership.ErrTeacherNotFound)
	require.ErrorIs(t, module.DeleteGuest(otherCtx, guest.ID), schoolmembership.ErrGuestNotFound)
	require.ErrorIs(t, module.ClearWorkTimeModel(otherCtx, staff.ID), schoolmembership.ErrStaffNotFound)
	require.ErrorIs(t, module.SetBirthdayDisplayOptOut(otherCtx, staff.ID, true), schoolmembership.ErrStaffNotFound)

	// Everything the other tenant tried left the owner's rows untouched.
	stillThere, err := module.FindStaff(ctx, staff.ID)
	require.NoError(t, err)
	assert.False(t, stillThere.IsDeleted())
	assert.False(t, stillThere.BirthdayDisplayOptOut)
}

func TestModuleWriteRollsBackWithOuterTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	person := testpkg.CreateTestPerson(t, db, "Rolled", "Back")
	wantErr := errors.New("abort outer transaction")

	var createdID int64
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		created, createErr := module.CreateStaff(txCtx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: person.ID}})
		require.NoError(t, createErr)
		createdID = created.ID
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	_, err = module.FindStaff(ctx, createdID)
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)

	listed, err := module.ListStaff(ctx, schoolmembership.StaffFilter{IncludeDeleted: true})
	require.NoError(t, err)
	assert.Empty(t, listed, "the rolled-back insert left no tombstone either")
}

func TestModuleDeleteRollsBackWithOuterFailureAndRetries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := createStaff(t, ctx, db, module, "Retry", "Candidate", schoolmembership.StaffFields{})
	wantErr := errors.New("fail after the delete wrote")

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		require.NoError(t, module.DeleteStaff(txCtx, staff.ID))
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	stillLive, err := module.FindStaff(ctx, staff.ID)
	require.NoError(t, err, "the failed transaction must not have deleted the row")
	assert.False(t, stillLive.IsDeleted())

	require.NoError(t, module.DeleteStaff(ctx, staff.ID), "the retry deletes what the rollback kept")
	require.ErrorIs(t, module.DeleteStaff(ctx, staff.ID), schoolmembership.ErrStaffNotFound,
		"a repeated retry finds nothing left to delete")
}

func TestModuleObservesStableErrorCodes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var observations []Observation
	module := buildModule(t, db, func(observation Observation) {
		observations = append(observations, observation)
	})
	ctx := testpkg.Ctx(t)

	_, err := module.FindStaff(ctx, 9_223_372_036_854_775_000)
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
	require.Len(t, observations, 1)
	assert.Equal(t, "find_staff", observations[0].Operation)
	assert.Equal(t, "not_found", schoolmembership.ErrorCode(observations[0].Err))
	assert.EqualValues(t, 1, observations[0].Stats.Queries)
	assert.Positive(t, observations[0].Stats.StatementDuration)

	_, err = module.FindTeacherByStaff(ctx, 9_223_372_036_854_775_000)
	require.ErrorIs(t, err, schoolmembership.ErrTeacherNotFound)
	require.Len(t, observations, 2)
	assert.Equal(t, "find_teacher_by_staff", observations[1].Operation)
	assert.Equal(t, "not_found", schoolmembership.ErrorCode(observations[1].Err))

	person := testpkg.CreateTestPerson(t, db, "Observed", "Create")
	_, err = module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: person.ID}})
	require.NoError(t, err)
	require.Len(t, observations, 3)
	assert.Equal(t, "create_staff", observations[2].Operation)
	assert.Equal(t, "none", schoolmembership.ErrorCode(observations[2].Err))
	assert.EqualValues(t, 1, observations[2].Stats.Rows)
	assert.Positive(t, observations[2].Stats.StatementDuration)
}

func TestModuleKeepsPersistenceErrorsVisible(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	person := testpkg.CreateTestPerson(t, db, "Ghost", "Tenant")

	missingTenantID := testpkg.UniqueTestTenantID(t)
	missingTenantCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), missingTenantID)
	_, err := module.CreateStaff(missingTenantCtx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: person.ID}})
	require.Error(t, err)
	assert.NotErrorIs(t, err, schoolmembership.ErrInvalidMembership)
	assert.NotErrorIs(t, err, schoolmembership.ErrStaffNotFound)
	assert.NotErrorIs(t, err, schoolmembership.ErrStaffPersonConflict)
	assert.Equal(t, "internal_error", schoolmembership.ErrorCode(err))
}

// boolPtr is the tri-state helper the *bool filters need; the shared test
// package carries pointer helpers for every other type but not for bool.
func boolPtr(value bool) *bool { return &value }
