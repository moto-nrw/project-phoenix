package schoolmembership_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// recordingEngine captures what the module hands to persistence so the
// validation and normalisation the facade owns can be asserted without a
// database.
type recordingEngine struct {
	calls int

	staffLock     string
	createdStaff  schoolmembership.CreateStaff
	updatedStaff  schoolmembership.UpdateStaff
	staffFilter   schoolmembership.StaffFilter
	appendedNotes string
	optOut        bool
	anchorDate    string

	createdTeacher schoolmembership.CreateTeacher
	updatedTeacher schoolmembership.UpdateTeacher
	teacherFilter  schoolmembership.TeacherFilter

	createdGuest schoolmembership.CreateGuest
	updatedGuest schoolmembership.UpdateGuest
	guestFilter  schoolmembership.GuestFilter
}

func (e *recordingEngine) FindStaff(_ context.Context, _ int64, lock string) (schoolmembership.Staff, error) {
	e.calls++
	e.staffLock = lock
	return schoolmembership.Staff{}, nil
}

func (e *recordingEngine) FindStaffByPerson(context.Context, int64) (schoolmembership.Staff, error) {
	e.calls++
	return schoolmembership.Staff{}, nil
}

func (e *recordingEngine) ListStaff(_ context.Context, filter schoolmembership.StaffFilter) ([]schoolmembership.Staff, error) {
	e.calls++
	e.staffFilter = filter
	return nil, nil
}

func (e *recordingEngine) CreateStaff(_ context.Context, input schoolmembership.CreateStaff) (schoolmembership.Staff, error) {
	e.calls++
	e.createdStaff = input
	return schoolmembership.Staff{PersonID: input.PersonID}, nil
}

func (e *recordingEngine) UpdateStaff(_ context.Context, input schoolmembership.UpdateStaff) (schoolmembership.Staff, error) {
	e.calls++
	e.updatedStaff = input
	return schoolmembership.Staff{ID: input.ID}, nil
}

func (e *recordingEngine) DeleteStaff(context.Context, int64) error { e.calls++; return nil }

func (e *recordingEngine) ClearWorkTimeModel(context.Context, int64) error { e.calls++; return nil }

func (e *recordingEngine) AppendStaffNotes(_ context.Context, _ int64, notes string) (schoolmembership.Staff, error) {
	e.calls++
	e.appendedNotes = notes
	return schoolmembership.Staff{StaffNotes: notes}, nil
}

func (e *recordingEngine) SetBirthdayDisplayOptOut(_ context.Context, _ int64, optOut bool) error {
	e.calls++
	e.optOut = optOut
	return nil
}

func (e *recordingEngine) RebaseWorkTimeModelAnchor(_ context.Context, _ int64, anchorDate string) ([]int64, error) {
	e.calls++
	e.anchorDate = anchorDate
	return nil, nil
}

func (e *recordingEngine) FindTeacher(context.Context, int64) (schoolmembership.Teacher, error) {
	e.calls++
	return schoolmembership.Teacher{}, nil
}

func (e *recordingEngine) FindTeacherByStaff(context.Context, int64) (schoolmembership.Teacher, error) {
	e.calls++
	return schoolmembership.Teacher{}, nil
}

func (e *recordingEngine) ListTeachers(_ context.Context, filter schoolmembership.TeacherFilter) ([]schoolmembership.Teacher, error) {
	e.calls++
	e.teacherFilter = filter
	return nil, nil
}

func (e *recordingEngine) CreateTeacher(_ context.Context, input schoolmembership.CreateTeacher) (schoolmembership.Teacher, error) {
	e.calls++
	e.createdTeacher = input
	return schoolmembership.Teacher{StaffID: input.StaffID}, nil
}

func (e *recordingEngine) UpdateTeacher(_ context.Context, input schoolmembership.UpdateTeacher) (schoolmembership.Teacher, error) {
	e.calls++
	e.updatedTeacher = input
	return schoolmembership.Teacher{ID: input.ID}, nil
}

func (e *recordingEngine) DeleteTeacher(context.Context, int64) error { e.calls++; return nil }

func (e *recordingEngine) FindGuest(context.Context, int64) (schoolmembership.Guest, error) {
	e.calls++
	return schoolmembership.Guest{}, nil
}

func (e *recordingEngine) FindGuestByStaff(context.Context, int64) (schoolmembership.Guest, error) {
	e.calls++
	return schoolmembership.Guest{}, nil
}

func (e *recordingEngine) ListGuests(_ context.Context, filter schoolmembership.GuestFilter) ([]schoolmembership.Guest, error) {
	e.calls++
	e.guestFilter = filter
	return nil, nil
}

func (e *recordingEngine) CreateGuest(_ context.Context, input schoolmembership.CreateGuest) (schoolmembership.Guest, error) {
	e.calls++
	e.createdGuest = input
	return schoolmembership.Guest{StaffID: input.StaffID}, nil
}

func (e *recordingEngine) UpdateGuest(_ context.Context, input schoolmembership.UpdateGuest) (schoolmembership.Guest, error) {
	e.calls++
	e.updatedGuest = input
	return schoolmembership.Guest{ID: input.ID}, nil
}

func (e *recordingEngine) DeleteGuest(context.Context, int64) error { e.calls++; return nil }

// newModule returns a module over a fresh recording engine.
func newModule() (*schoolmembership.Module, *recordingEngine) {
	engine := &recordingEngine{}
	return schoolmembership.NewModule(engine), engine
}

// requireInvalid asserts the error is the typed validation error carrying the
// expected reason, and that persistence was never reached.
func requireInvalid(t *testing.T, engine *recordingEngine, err error, reason string) {
	t.Helper()
	if !errors.Is(err, schoolmembership.ErrInvalidMembership) {
		t.Fatalf("expected ErrInvalidMembership, got %v", err)
	}
	var invalid *schoolmembership.InvalidMembershipError
	if !errors.As(err, &invalid) || invalid.Reason != reason {
		t.Fatalf("expected reason %q, got %v", reason, err)
	}
	if engine.calls != 0 {
		t.Fatalf("engine must not be called for invalid input (%d calls)", engine.calls)
	}
}

func TestReadsRequirePositiveIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	module, engine := newModule()
	_, err := module.FindStaff(ctx, 0)
	requireInvalid(t, engine, err, "staff ID is required")

	module, engine = newModule()
	_, err = module.FindStaffForMutation(ctx, -1)
	requireInvalid(t, engine, err, "staff ID is required")

	module, engine = newModule()
	_, err = module.FindStaffByPerson(ctx, 0)
	requireInvalid(t, engine, err, "person ID is required")

	module, engine = newModule()
	_, err = module.FindTeacher(ctx, 0)
	requireInvalid(t, engine, err, "teacher ID is required")

	module, engine = newModule()
	_, err = module.FindTeacherByStaff(ctx, 0)
	requireInvalid(t, engine, err, "staff ID is required")

	module, engine = newModule()
	_, err = module.FindGuest(ctx, 0)
	requireInvalid(t, engine, err, "guest ID is required")

	module, engine = newModule()
	_, err = module.FindGuestByStaff(ctx, 0)
	requireInvalid(t, engine, err, "staff ID is required")
}

func TestFindStaffForMutationAsksForTheRowLock(t *testing.T) {
	t.Parallel()
	module, engine := newModule()

	if _, err := module.FindStaff(context.Background(), 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.staffLock != "" {
		t.Fatalf("a plain read must not lock, got %q", engine.staffLock)
	}
	if _, err := module.FindStaffForMutation(context.Background(), 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.staffLock != "UPDATE" {
		t.Fatalf("expected the UPDATE lock, got %q", engine.staffLock)
	}
}

func TestCommandsRequirePositiveIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	module, engine := newModule()
	requireInvalid(t, engine, module.DeleteStaff(ctx, 0), "staff ID is required")

	module, engine = newModule()
	requireInvalid(t, engine, module.ClearWorkTimeModel(ctx, 0), "staff ID is required")

	module, engine = newModule()
	_, err := module.AppendStaffNotes(ctx, 0, "note")
	requireInvalid(t, engine, err, "staff ID is required")

	module, engine = newModule()
	requireInvalid(t, engine, module.SetBirthdayDisplayOptOut(ctx, 0, true), "staff ID is required")

	module, engine = newModule()
	_, err = module.UpdateStaff(ctx, schoolmembership.UpdateStaff{StaffFields: schoolmembership.StaffFields{PersonID: 7}})
	requireInvalid(t, engine, err, "staff ID is required")

	module, engine = newModule()
	_, err = module.UpdateTeacher(ctx, schoolmembership.UpdateTeacher{TeacherFields: schoolmembership.TeacherFields{StaffID: 7}})
	requireInvalid(t, engine, err, "teacher ID is required")

	module, engine = newModule()
	requireInvalid(t, engine, module.DeleteTeacher(ctx, 0), "teacher ID is required")

	module, engine = newModule()
	_, err = module.UpdateGuest(ctx, schoolmembership.UpdateGuest{GuestFields: schoolmembership.GuestFields{StaffID: 7, ActivityExpertise: "Sport"}})
	requireInvalid(t, engine, err, "guest ID is required")

	module, engine = newModule()
	requireInvalid(t, engine, module.DeleteGuest(ctx, 0), "guest ID is required")
}

func TestCreateStaffValidatesEmploymentTypeAndWorkTimeModel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	module, engine := newModule()
	_, err := module.CreateStaff(ctx, schoolmembership.CreateStaff{})
	requireInvalid(t, engine, err, "person ID is required")

	unknown := "Aushilfe"
	module, engine = newModule()
	_, err = module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: 7, EmploymentType: &unknown}})
	requireInvalid(t, engine, err, "employment_type must be 'full_time', 'part_time', or 'minijob'")

	zero := int64(0)
	module, engine = newModule()
	_, err = module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: 7, WorkTimeModelID: &zero}})
	requireInvalid(t, engine, err, "work time model ID must be positive")

	module, engine = newModule()
	_, err = module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: 7, RotationAnchorDate: "01.03.2026"}})
	requireInvalid(t, engine, err, "rotation anchor date must be a calendar date in YYYY-MM-DD format")

	for _, employmentType := range []string{
		schoolmembership.EmploymentTypeFullTime,
		schoolmembership.EmploymentTypePartTime,
		schoolmembership.EmploymentTypeMinijob,
	} {
		module, engine := newModule()
		value := employmentType
		_, err := module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: 7, EmploymentType: &value}})
		if err != nil || engine.calls != 1 {
			t.Fatalf("%q must be accepted: %v (%d calls)", employmentType, err, engine.calls)
		}
	}
}

func TestStaffWritesNormalizeThePersonnelNumber(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	padded := "  P-100  "
	module, engine := newModule()
	if _, err := module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: 7, PersonnelNumber: &padded}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.createdStaff.PersonnelNumber == nil || *engine.createdStaff.PersonnelNumber != "P-100" {
		t.Fatalf("personnel number was not trimmed: %v", engine.createdStaff.PersonnelNumber)
	}

	blank := "   "
	module, engine = newModule()
	if _, err := module.UpdateStaff(ctx, schoolmembership.UpdateStaff{ID: 3, StaffFields: schoolmembership.StaffFields{PersonID: 7, PersonnelNumber: &blank}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.updatedStaff.PersonnelNumber != nil {
		t.Fatalf("a blank personnel number must become unset, got %q", *engine.updatedStaff.PersonnelNumber)
	}
}

func TestListStaffDeduplicatesIDsAndKeepsTheEmptySliceApartFromNil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	module, engine := newModule()
	if _, err := module.ListStaff(ctx, schoolmembership.StaffFilter{IDs: []int64{5, 5, 0, -3, 2}, PersonIDs: []int64{9, 9}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := engine.staffFilter.IDs; len(got) != 2 || got[0] != 5 || got[1] != 2 {
		t.Fatalf("IDs were not deduplicated in order: %v", got)
	}
	if got := engine.staffFilter.PersonIDs; len(got) != 1 || got[0] != 9 {
		t.Fatalf("person IDs were not deduplicated: %v", got)
	}

	module, engine = newModule()
	if _, err := module.ListStaff(ctx, schoolmembership.StaffFilter{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.staffFilter.IDs != nil || engine.staffFilter.PersonIDs != nil {
		t.Fatalf("an unset ID filter must stay nil: %+v", engine.staffFilter)
	}

	module, engine = newModule()
	if _, err := module.ListStaff(ctx, schoolmembership.StaffFilter{IDs: []int64{-1}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.staffFilter.IDs == nil || len(engine.staffFilter.IDs) != 0 {
		t.Fatalf("a filter that keeps no ID must stay empty-but-set: %v", engine.staffFilter.IDs)
	}

	module, engine = newModule()
	zero := int64(0)
	_, err := module.ListStaff(ctx, schoolmembership.StaffFilter{WorkTimeModelID: &zero})
	requireInvalid(t, engine, err, "work time model ID must be positive")
}

func TestRebaseWorkTimeModelAnchorValidatesItsArguments(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	module, engine := newModule()
	_, err := module.RebaseWorkTimeModelAnchor(ctx, 0, "2026-03-01")
	requireInvalid(t, engine, err, "work time model ID is required")

	module, engine = newModule()
	_, err = module.RebaseWorkTimeModelAnchor(ctx, 4, "2026-13-01")
	requireInvalid(t, engine, err, "rotation anchor date must be a calendar date in YYYY-MM-DD format")

	module, engine = newModule()
	if _, err := module.RebaseWorkTimeModelAnchor(ctx, 4, "2026-03-01"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.anchorDate != "2026-03-01" {
		t.Fatalf("anchor date was not passed through: %q", engine.anchorDate)
	}
}

func TestTeacherWritesTrimAndValidate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	module, engine := newModule()
	_, err := module.CreateTeacher(ctx, schoolmembership.CreateTeacher{})
	requireInvalid(t, engine, err, "staff ID is required")

	module, engine = newModule()
	_, err = module.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{
		StaffID: 7, Specialization: "  Mathematik ", Role: " Leitung ", Qualifications: "Diplom",
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.createdTeacher.Specialization != "Mathematik" || engine.createdTeacher.Role != "Leitung" {
		t.Fatalf("teacher fields were not trimmed: %+v", engine.createdTeacher)
	}
	if engine.createdTeacher.Qualifications != "Diplom" {
		t.Fatalf("qualifications must pass through untouched: %q", engine.createdTeacher.Qualifications)
	}
}

func TestListTeachersTrimsFiltersAndDeduplicatesIDs(t *testing.T) {
	t.Parallel()
	module, engine := newModule()

	if _, err := module.ListTeachers(context.Background(), schoolmembership.TeacherFilter{
		IDs: []int64{4, 4}, StaffIDs: []int64{8, 0, 8},
		Specialization: " Mathematik ", SpecializationContains: " athema ", RoleContains: " Leit ",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	filter := engine.teacherFilter
	if len(filter.IDs) != 1 || len(filter.StaffIDs) != 1 {
		t.Fatalf("teacher ID filters were not deduplicated: %+v", filter)
	}
	if filter.Specialization != "Mathematik" || filter.SpecializationContains != "athema" || filter.RoleContains != "Leit" {
		t.Fatalf("teacher filter was not trimmed: %+v", filter)
	}
}

func TestGuestWritesValidateContactDataAndDates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	base := schoolmembership.GuestFields{StaffID: 7, ActivityExpertise: "Zirkus"}

	module, engine := newModule()
	_, err := module.CreateGuest(ctx, schoolmembership.CreateGuest{})
	requireInvalid(t, engine, err, "staff ID is required")

	module, engine = newModule()
	_, err = module.CreateGuest(ctx, schoolmembership.CreateGuest{GuestFields: schoolmembership.GuestFields{StaffID: 7, ActivityExpertise: "   "}})
	requireInvalid(t, engine, err, "activity expertise is required")

	invalidEmail := base
	invalidEmail.ContactEmail = "not-an-address"
	module, engine = newModule()
	_, err = module.CreateGuest(ctx, schoolmembership.CreateGuest{GuestFields: invalidEmail})
	requireInvalid(t, engine, err, "invalid contact email format")

	invalidPhone := base
	invalidPhone.ContactPhone = "0221-abc"
	module, engine = newModule()
	_, err = module.CreateGuest(ctx, schoolmembership.CreateGuest{GuestFields: invalidPhone})
	requireInvalid(t, engine, err, "invalid contact phone format")

	shortPhone := base
	shortPhone.ContactPhone = "12345"
	module, engine = newModule()
	_, err = module.CreateGuest(ctx, schoolmembership.CreateGuest{GuestFields: shortPhone})
	requireInvalid(t, engine, err, "invalid contact phone format")

	badStart := base
	badStart.StartDate = "2026-3-1"
	module, engine = newModule()
	_, err = module.CreateGuest(ctx, schoolmembership.CreateGuest{GuestFields: badStart})
	requireInvalid(t, engine, err, "start date must be a calendar date in YYYY-MM-DD format")

	badEnd := base
	badEnd.EndDate = "31.03.2026"
	module, engine = newModule()
	_, err = module.CreateGuest(ctx, schoolmembership.CreateGuest{GuestFields: badEnd})
	requireInvalid(t, engine, err, "end date must be a calendar date in YYYY-MM-DD format")

	reversed := base
	reversed.StartDate = "2026-03-31"
	reversed.EndDate = "2026-03-01"
	module, engine = newModule()
	_, err = module.CreateGuest(ctx, schoolmembership.CreateGuest{GuestFields: reversed})
	requireInvalid(t, engine, err, "end date cannot be before start date")
}

func TestCreateGuestTrimsTheContactFields(t *testing.T) {
	t.Parallel()
	module, engine := newModule()

	_, err := module.CreateGuest(context.Background(), schoolmembership.CreateGuest{GuestFields: schoolmembership.GuestFields{
		StaffID: 7, ActivityExpertise: "  Zirkus  ", Organization: "  Circus e.V. ",
		ContactEmail: "  gast@example.com ", ContactPhone: " +49 221 1234567 ",
		StartDate: "2026-03-01", EndDate: "2026-03-31",
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	created := engine.createdGuest
	if created.ActivityExpertise != "Zirkus" || created.Organization != "Circus e.V." {
		t.Fatalf("guest fields were not trimmed: %+v", created)
	}
	if created.ContactEmail != "gast@example.com" || created.ContactPhone != "+49 221 1234567" {
		t.Fatalf("contact data was not trimmed: %+v", created)
	}
}

func TestListGuestsTrimsFiltersAndValidatesTheActiveOnDate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	module, engine := newModule()
	if _, err := module.ListGuests(ctx, schoolmembership.GuestFilter{
		IDs: []int64{2, 2}, StaffIDs: []int64{0},
		OrganizationContains: " Circus ", ExpertiseContains: " Zirkus ", ActiveOn: "2026-03-15",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	filter := engine.guestFilter
	if len(filter.IDs) != 1 || filter.StaffIDs == nil || len(filter.StaffIDs) != 0 {
		t.Fatalf("guest ID filters were not normalized: %+v", filter)
	}
	if filter.OrganizationContains != "Circus" || filter.ExpertiseContains != "Zirkus" {
		t.Fatalf("guest filter was not trimmed: %+v", filter)
	}

	module, engine = newModule()
	_, err := module.ListGuests(ctx, schoolmembership.GuestFilter{ActiveOn: "15.03.2026"})
	requireInvalid(t, engine, err, "active-on date must be a calendar date in YYYY-MM-DD format")
}

func TestNotesAndOptOutReachTheEngineUnchanged(t *testing.T) {
	t.Parallel()
	module, engine := newModule()
	ctx := context.Background()

	staff, err := module.AppendStaffNotes(ctx, 7, "Zweiter Absatz")
	if err != nil || staff.StaffNotes != "Zweiter Absatz" || engine.appendedNotes != "Zweiter Absatz" {
		t.Fatalf("notes were altered on the way in: %q err=%v", engine.appendedNotes, err)
	}
	if err := module.SetBirthdayDisplayOptOut(ctx, 7, true); err != nil || !engine.optOut {
		t.Fatalf("opt-out flag was not passed through: %v err=%v", engine.optOut, err)
	}
}

func TestErrorCodeIsStable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		want string
		err  error
	}{
		{"none", nil},
		{"not_found", schoolmembership.ErrStaffNotFound},
		{"not_found", schoolmembership.ErrTeacherNotFound},
		{"not_found", schoolmembership.ErrGuestNotFound},
		{"invalid", schoolmembership.ErrInvalidMembership},
		{"invalid", &schoolmembership.InvalidMembershipError{Reason: "staff ID is required"}},
		{"membership_conflict", schoolmembership.ErrStaffPersonConflict},
		{"membership_conflict", schoolmembership.ErrTeacherStaffConflict},
		{"membership_conflict", schoolmembership.ErrGuestStaffConflict},
		{"personnel_number_conflict", schoolmembership.ErrPersonnelNumberConflict},
		{"internal_error", errors.New("boom")},
	}
	for _, testCase := range cases {
		if got := schoolmembership.ErrorCode(testCase.err); got != testCase.want {
			t.Errorf("ErrorCode(%v) = %q, want %q", testCase.err, got, testCase.want)
		}
	}
}
