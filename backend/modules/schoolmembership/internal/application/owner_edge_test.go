package application

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/ports"
)

// The owner edge (#4448): validation and normalisation the School Membership
// facade used to own, asserted on the service without a database.

// recordingStore captures what the owner hands to persistence so the
// validation and normalisation at the owner edge can be asserted without a
// database. It serves the real application service; a store method the
// service is not expected to reach is left to the embedded nil Store.
type recordingStore struct {
	ports.Store

	calls         int
	studentIDs    []int64
	fromClass     string
	toClass       string
	studentStatus string

	staffLock    string
	createdStaff schoolmembership.CreateStaff
	updatedStaff schoolmembership.UpdateStaff
	staffWrite   *schoolmembership.StaffFields
	staffFilter  schoolmembership.StaffFilter

	createdTeacher schoolmembership.CreateTeacher
	teacherFilter  schoolmembership.TeacherFilter

	createdRecord schoolmembership.CreateExternalCaregiverRecord
	recordFilter  schoolmembership.ExternalCaregiverRecordFilter

	entryLock    string
	createdEntry schoolmembership.CreateClassListEntry
	updatedEntry schoolmembership.UpdateClassListEntry
	entryFilter  schoolmembership.ClassListEntryFilter
}

func (e *recordingStore) ChangeStudentClass(_ context.Context, ids []int64, from, to string) (int64, domain.OperationStats, error) {
	e.calls++
	e.studentIDs, e.fromClass, e.toClass = ids, from, to
	return 0, domain.OperationStats{}, nil
}

func (e *recordingStore) LockStudentClassWrites(context.Context, bool) (domain.OperationStats, error) {
	return domain.OperationStats{}, nil
}

func (e *recordingStore) LockChildQuota(context.Context) (domain.OperationStats, error) {
	return domain.OperationStats{}, nil
}

func (e *recordingStore) GraduateStudents(_ context.Context, ids []int64) (int64, domain.OperationStats, error) {
	e.calls++
	e.studentIDs = ids
	return 0, domain.OperationStats{}, nil
}

func (e *recordingStore) ReactivateStudents(_ context.Context, ids []int64, status string) ([]int64, domain.OperationStats, error) {
	e.calls++
	e.studentIDs, e.studentStatus = ids, status
	return nil, domain.OperationStats{}, nil
}

func (e *recordingStore) FindStaff(_ context.Context, id int64, lock string, _ bool) (schoolmembership.Staff, bool, domain.OperationStats, error) {
	e.calls++
	e.staffLock = lock
	return schoolmembership.Staff{ID: id}, true, domain.OperationStats{}, nil
}

func (e *recordingStore) ListStaff(_ context.Context, filter schoolmembership.StaffFilter) ([]schoolmembership.Staff, domain.OperationStats, error) {
	e.calls++
	e.staffFilter = filter
	return nil, domain.OperationStats{}, nil
}

func (e *recordingStore) CreateStaff(_ context.Context, personID int64) (schoolmembership.Staff, domain.OperationStats, error) {
	e.calls++
	e.createdStaff.PersonID = personID
	e.staffWrite = &e.createdStaff.StaffFields
	return schoolmembership.Staff{PersonID: personID}, domain.OperationStats{}, nil
}

func (e *recordingStore) UpdateStaff(_ context.Context, id, personID int64) (schoolmembership.Staff, domain.OperationStats, error) {
	e.calls++
	e.updatedStaff.ID, e.updatedStaff.PersonID = id, personID
	e.staffWrite = &e.updatedStaff.StaffFields
	return schoolmembership.Staff{ID: id}, domain.OperationStats{}, nil
}

func (e *recordingStore) ListTeachers(_ context.Context, filter schoolmembership.TeacherFilter) ([]schoolmembership.Teacher, domain.OperationStats, error) {
	e.calls++
	e.teacherFilter = filter
	return nil, domain.OperationStats{}, nil
}

func (e *recordingStore) CreateTeacher(_ context.Context, fields schoolmembership.TeacherFields) (schoolmembership.Teacher, domain.OperationStats, error) {
	e.calls++
	e.createdTeacher = schoolmembership.CreateTeacher{TeacherFields: fields}
	return schoolmembership.Teacher{StaffID: fields.StaffID}, domain.OperationStats{}, nil
}

func (e *recordingStore) ListExternalCaregiverRecords(_ context.Context, filter schoolmembership.ExternalCaregiverRecordFilter) ([]schoolmembership.ExternalCaregiverRecord, domain.OperationStats, error) {
	e.calls++
	e.recordFilter = filter
	return nil, domain.OperationStats{}, nil
}

func (e *recordingStore) CreateExternalCaregiverRecord(_ context.Context, fields schoolmembership.ExternalCaregiverRecordFields) (schoolmembership.ExternalCaregiverRecord, domain.OperationStats, error) {
	e.calls++
	e.createdRecord = schoolmembership.CreateExternalCaregiverRecord{ExternalCaregiverRecordFields: fields}
	return schoolmembership.ExternalCaregiverRecord{StaffID: fields.StaffID}, domain.OperationStats{}, nil
}

func (e *recordingStore) FindClassListEntry(_ context.Context, id int64, lock string) (schoolmembership.ClassListEntry, bool, domain.OperationStats, error) {
	e.calls++
	e.entryLock = lock
	return schoolmembership.ClassListEntry{ID: id}, true, domain.OperationStats{}, nil
}

func (e *recordingStore) ListClassListEntries(_ context.Context, filter schoolmembership.ClassListEntryFilter) ([]schoolmembership.ClassListEntry, domain.OperationStats, error) {
	e.calls++
	e.entryFilter = filter
	return nil, domain.OperationStats{}, nil
}

func (e *recordingStore) CreateClassListEntry(_ context.Context, fields schoolmembership.ClassListEntryFields, createdBy *int64) (schoolmembership.ClassListEntry, domain.OperationStats, error) {
	e.calls++
	e.createdEntry = schoolmembership.CreateClassListEntry{ClassListEntryFields: fields, CreatedBy: createdBy}
	return schoolmembership.ClassListEntry{FirstName: fields.FirstName}, domain.OperationStats{}, nil
}

func (e *recordingStore) UpdateClassListEntry(_ context.Context, id int64, fields schoolmembership.ClassListEntryFields) (schoolmembership.ClassListEntry, domain.OperationStats, error) {
	e.calls++
	e.updatedEntry = schoolmembership.UpdateClassListEntry{ID: id, ClassListEntryFields: fields}
	return schoolmembership.ClassListEntry{ID: id}, domain.OperationStats{}, nil
}

// StaffEmployments and SaveStaffEmployment are the Workforce half of a staff
// write: the saved profile completes the recorded create or update.
func (e *recordingStore) StaffEmployments(context.Context, []int64) (map[int64]domain.StaffEmployment, error) {
	return map[int64]domain.StaffEmployment{}, nil
}

func (e *recordingStore) SaveStaffEmployment(_ context.Context, employment domain.StaffEmployment) error {
	if e.staffWrite != nil {
		e.staffWrite.StaffNotes, e.staffWrite.EmploymentType = employment.StaffNotes, employment.EmploymentType
		e.staffWrite.WorkTimeModelID, e.staffWrite.PersonnelNumber = employment.WorkTimeModelID, employment.PersonnelNumber
		e.staffWrite.RotationAnchorDate, e.staffWrite.BirthdayDisplayOptOut = employment.RotationAnchorDate, employment.BirthdayDisplayOptOut
	}
	return nil
}

// ChildQuotaLimit reports a school without Kinderkontingent.
func (e *recordingStore) ChildQuotaLimit(context.Context) (int, bool, error) { return 0, false, nil }

// joinedTransaction runs every unit of work on the caller's context.
type joinedTransaction struct{}

func (joinedTransaction) RunWrite(ctx context.Context, work func(context.Context) error) error {
	return work(ctx)
}

func (joinedTransaction) RunSavepoint(ctx context.Context, work func(context.Context) error) error {
	return work(ctx)
}

func (joinedTransaction) RunRead(ctx context.Context, work func(context.Context) error) error {
	return work(ctx)
}

// newModule returns the owner over a fresh recording store.
func newModule() (schoolmembership.Owner, *recordingStore) {
	store := &recordingStore{}
	today := func() string { return "2026-03-02" }
	return New(store, store, store, today, joinedTransaction{}, func(ports.Observation) {}, nil), store
}

// requireInvalid asserts the error is the typed validation error carrying the
// expected reason, and that persistence was never reached.
func requireInvalid(t *testing.T, engine *recordingStore, err error, reason string) {
	t.Helper()
	if !errors.Is(err, schoolmembership.ErrInvalidMembership) {
		t.Fatalf("expected ErrInvalidMembership, got %v", err)
	}
	var invalid *schoolmembership.InvalidMembershipError
	if !errors.As(err, &invalid) || invalid.Reason != reason {
		t.Fatalf("expected reason %q, got %v", reason, err)
	}
	if engine.calls != 0 {
		t.Fatalf("persistence must not be reached for invalid input (%d calls)", engine.calls)
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
	_, err = module.FindExternalCaregiverRecord(ctx, 0)
	requireInvalid(t, engine, err, "external caregiver record ID is required")

	module, engine = newModule()
	_, err = module.FindExternalCaregiverRecordByStaff(ctx, 0)
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
	_, err := module.UpdateStaff(ctx, schoolmembership.UpdateStaff{StaffFields: schoolmembership.StaffFields{PersonID: 7}})
	requireInvalid(t, engine, err, "staff ID is required")

	module, engine = newModule()
	_, err = module.UpdateTeacher(ctx, schoolmembership.UpdateTeacher{TeacherFields: schoolmembership.TeacherFields{StaffID: 7}})
	requireInvalid(t, engine, err, "teacher ID is required")

	module, engine = newModule()
	requireInvalid(t, engine, module.DeleteTeacher(ctx, 0), "teacher ID is required")

	module, engine = newModule()
	_, err = module.UpdateExternalCaregiverRecord(ctx, schoolmembership.UpdateExternalCaregiverRecord{ExternalCaregiverRecordFields: schoolmembership.ExternalCaregiverRecordFields{StaffID: 7, ActivityExpertise: "Sport"}})
	requireInvalid(t, engine, err, "external caregiver record ID is required")

	module, engine = newModule()
	requireInvalid(t, engine, module.DeleteExternalCaregiverRecord(ctx, 0), "external caregiver record ID is required")
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

func TestExternalCaregiverRecordWritesValidateContactDataAndDates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	base := schoolmembership.ExternalCaregiverRecordFields{StaffID: 7, ActivityExpertise: "Zirkus"}

	module, engine := newModule()
	_, err := module.CreateExternalCaregiverRecord(ctx, schoolmembership.CreateExternalCaregiverRecord{})
	requireInvalid(t, engine, err, "staff ID is required")

	module, engine = newModule()
	_, err = module.CreateExternalCaregiverRecord(ctx, schoolmembership.CreateExternalCaregiverRecord{ExternalCaregiverRecordFields: schoolmembership.ExternalCaregiverRecordFields{StaffID: 7, ActivityExpertise: "   "}})
	requireInvalid(t, engine, err, "activity expertise is required")

	invalidEmail := base
	invalidEmail.ContactEmail = "not-an-address"
	module, engine = newModule()
	_, err = module.CreateExternalCaregiverRecord(ctx, schoolmembership.CreateExternalCaregiverRecord{ExternalCaregiverRecordFields: invalidEmail})
	requireInvalid(t, engine, err, "invalid contact email format")

	invalidPhone := base
	invalidPhone.ContactPhone = "0221-abc"
	module, engine = newModule()
	_, err = module.CreateExternalCaregiverRecord(ctx, schoolmembership.CreateExternalCaregiverRecord{ExternalCaregiverRecordFields: invalidPhone})
	requireInvalid(t, engine, err, "invalid contact phone format")

	shortPhone := base
	shortPhone.ContactPhone = "12345"
	module, engine = newModule()
	_, err = module.CreateExternalCaregiverRecord(ctx, schoolmembership.CreateExternalCaregiverRecord{ExternalCaregiverRecordFields: shortPhone})
	requireInvalid(t, engine, err, "invalid contact phone format")

	badStart := base
	badStart.StartDate = "2026-3-1"
	module, engine = newModule()
	_, err = module.CreateExternalCaregiverRecord(ctx, schoolmembership.CreateExternalCaregiverRecord{ExternalCaregiverRecordFields: badStart})
	requireInvalid(t, engine, err, "start date must be a calendar date in YYYY-MM-DD format")

	badEnd := base
	badEnd.EndDate = "31.03.2026"
	module, engine = newModule()
	_, err = module.CreateExternalCaregiverRecord(ctx, schoolmembership.CreateExternalCaregiverRecord{ExternalCaregiverRecordFields: badEnd})
	requireInvalid(t, engine, err, "end date must be a calendar date in YYYY-MM-DD format")

	reversed := base
	reversed.StartDate = "2026-03-31"
	reversed.EndDate = "2026-03-01"
	module, engine = newModule()
	_, err = module.CreateExternalCaregiverRecord(ctx, schoolmembership.CreateExternalCaregiverRecord{ExternalCaregiverRecordFields: reversed})
	requireInvalid(t, engine, err, "end date cannot be before start date")
}

func TestCreateExternalCaregiverRecordTrimsTheContactFields(t *testing.T) {
	t.Parallel()
	module, engine := newModule()

	_, err := module.CreateExternalCaregiverRecord(context.Background(), schoolmembership.CreateExternalCaregiverRecord{ExternalCaregiverRecordFields: schoolmembership.ExternalCaregiverRecordFields{
		StaffID: 7, ActivityExpertise: "  Zirkus  ", Organization: "  Circus e.V. ",
		ContactEmail: "  gast@example.com ", ContactPhone: " +49 221 1234567 ",
		StartDate: "2026-03-01", EndDate: "2026-03-31",
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	created := engine.createdRecord
	if created.ActivityExpertise != "Zirkus" || created.Organization != "Circus e.V." {
		t.Fatalf("external caregiver record fields were not trimmed: %+v", created)
	}
	if created.ContactEmail != "gast@example.com" || created.ContactPhone != "+49 221 1234567" {
		t.Fatalf("contact data was not trimmed: %+v", created)
	}
}

func TestListExternalCaregiverRecordsTrimsFiltersAndValidatesTheActiveOnDate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	module, engine := newModule()
	if _, err := module.ListExternalCaregiverRecords(ctx, schoolmembership.ExternalCaregiverRecordFilter{
		IDs: []int64{2, 2}, StaffIDs: []int64{0},
		OrganizationContains: " Circus ", ExpertiseContains: " Zirkus ", ActiveOn: "2026-03-15",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	filter := engine.recordFilter
	if len(filter.IDs) != 1 || filter.StaffIDs == nil || len(filter.StaffIDs) != 0 {
		t.Fatalf("external caregiver record ID filters were not normalized: %+v", filter)
	}
	if filter.OrganizationContains != "Circus" || filter.ExpertiseContains != "Zirkus" {
		t.Fatalf("external caregiver record filter was not trimmed: %+v", filter)
	}

	module, engine = newModule()
	_, err := module.ListExternalCaregiverRecords(ctx, schoolmembership.ExternalCaregiverRecordFilter{ActiveOn: "15.03.2026"})
	requireInvalid(t, engine, err, "active-on date must be a calendar date in YYYY-MM-DD format")
}

func TestListStaffNormalizesTenantIDsLikeTheOtherIDFilters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	module, engine := newModule()
	if _, err := module.ListStaff(ctx, schoolmembership.StaffFilter{TenantIDs: []int64{4, 4, 0, -1, 2}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := engine.staffFilter.TenantIDs; len(got) != 2 || got[0] != 4 || got[1] != 2 {
		t.Fatalf("tenant IDs were not deduplicated in order: %v", got)
	}

	module, engine = newModule()
	if _, err := module.ListStaff(ctx, schoolmembership.StaffFilter{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.staffFilter.TenantIDs != nil {
		t.Fatalf("an unset tenant filter must stay nil: %+v", engine.staffFilter)
	}
}

func TestClassListEntryReadsRequirePositiveIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	module, engine := newModule()
	_, err := module.FindClassListEntry(ctx, 0)
	requireInvalid(t, engine, err, "class list entry ID is required")

	module, engine = newModule()
	_, err = module.FindClassListEntryForMutation(ctx, -3)
	requireInvalid(t, engine, err, "class list entry ID is required")
}

func TestFindClassListEntryForMutationAsksForTheRowLock(t *testing.T) {
	t.Parallel()
	module, engine := newModule()

	if _, err := module.FindClassListEntry(context.Background(), 7); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.entryLock != "" {
		t.Fatalf("a plain read must not lock, got %q", engine.entryLock)
	}
	if _, err := module.FindClassListEntryForMutation(context.Background(), 7); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.entryLock != "UPDATE" {
		t.Fatalf("expected the UPDATE lock, got %q", engine.entryLock)
	}
}

func TestClassListEntryWritesTrimAndValidate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	module, engine := newModule()
	_, err := module.CreateClassListEntry(ctx, schoolmembership.CreateClassListEntry{ClassListEntryFields: schoolmembership.ClassListEntryFields{
		FirstName: "  ", LastName: "Aalders", SchoolClass: "1a",
	}})
	requireInvalid(t, engine, err, "first name is required")

	module, engine = newModule()
	_, err = module.CreateClassListEntry(ctx, schoolmembership.CreateClassListEntry{ClassListEntryFields: schoolmembership.ClassListEntryFields{
		FirstName: "Zoe", LastName: "\t", SchoolClass: "1a",
	}})
	requireInvalid(t, engine, err, "last name is required")

	module, engine = newModule()
	_, err = module.CreateClassListEntry(ctx, schoolmembership.CreateClassListEntry{ClassListEntryFields: schoolmembership.ClassListEntryFields{
		FirstName: "Zoe", LastName: "Aalders",
	}})
	requireInvalid(t, engine, err, "school class is required")

	module, engine = newModule()
	_, err = module.UpdateClassListEntry(ctx, schoolmembership.UpdateClassListEntry{ClassListEntryFields: schoolmembership.ClassListEntryFields{
		FirstName: "Zoe", LastName: "Aalders", SchoolClass: "1a",
	}})
	requireInvalid(t, engine, err, "class list entry ID is required")

	module, engine = newModule()
	requireInvalid(t, engine, module.DeleteClassListEntry(ctx, 0), "class list entry ID is required")

	module, engine = newModule()
	zeroActor := int64(0)
	created, err := module.CreateClassListEntry(ctx, schoolmembership.CreateClassListEntry{
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: " Zoe ", LastName: " Aalders ", SchoolClass: " 1a "},
		CreatedBy:            &zeroActor,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.FirstName != "Zoe" {
		t.Fatalf("the engine must receive trimmed fields, got %q", created.FirstName)
	}
	if engine.createdEntry.LastName != "Aalders" || engine.createdEntry.SchoolClass != "1a" {
		t.Fatalf("expected trimmed fields, got %+v", engine.createdEntry.ClassListEntryFields)
	}
	if engine.createdEntry.CreatedBy != nil {
		t.Fatalf("a zero actor means no creating account, got %v", *engine.createdEntry.CreatedBy)
	}

	module, engine = newModule()
	if _, err := module.UpdateClassListEntry(ctx, schoolmembership.UpdateClassListEntry{ID: 9, ClassListEntryFields: schoolmembership.ClassListEntryFields{
		FirstName: "Zoe", LastName: "Aalders ", SchoolClass: " 1b",
	}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.updatedEntry.ID != 9 || engine.updatedEntry.SchoolClass != "1b" || engine.updatedEntry.LastName != "Aalders" {
		t.Fatalf("expected the trimmed update, got %+v", engine.updatedEntry)
	}
}

func TestListClassListEntriesTrimsFiltersAndDeduplicatesIDs(t *testing.T) {
	t.Parallel()
	module, engine := newModule()

	if _, err := module.ListClassListEntries(context.Background(), schoolmembership.ClassListEntryFilter{
		IDs: []int64{3, 0, 3, -1, 5}, FirstName: " Zoe ", LastName: " Aalders", SchoolClass: "1a ",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	filter := engine.entryFilter
	if len(filter.IDs) != 2 || filter.IDs[0] != 3 || filter.IDs[1] != 5 {
		t.Fatalf("expected deduplicated positive IDs, got %v", filter.IDs)
	}
	if filter.FirstName != "Zoe" || filter.LastName != "Aalders" || filter.SchoolClass != "1a" {
		t.Fatalf("expected trimmed matches, got %+v", filter)
	}
}

func TestStudentLifecycleCommandsNormalizeAndValidate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	module, engine := newModule()
	_, err := module.ChangeClass(ctx, []int64{4, 4, -1, 0, 7}, "1a", "2a")
	if err != nil || !reflect.DeepEqual(engine.studentIDs, []int64{4, 7}) || engine.fromClass != "1a" || engine.toClass != "2a" {
		t.Fatalf("class command lost normalization or comparison: %+v, %v", engine, err)
	}
	_, err = module.Reactivate(ctx, []int64{7, 7, 0}, " active ", schoolmembership.EnforceChildQuota)
	if err != nil || !reflect.DeepEqual(engine.studentIDs, []int64{7}) || engine.studentStatus != "active" {
		t.Fatalf("reactivation lost normalization: %+v, %v", engine, err)
	}
	calls := engine.calls
	if _, err := module.Graduate(ctx, []int64{0, -1}); err != nil || engine.calls != calls {
		t.Fatalf("empty normalized batch reached persistence: %v", err)
	}
	for _, run := range []func() error{
		func() error { _, err := module.ChangeClass(ctx, []int64{1}, " ", "2a"); return err },
		func() error {
			_, err := module.Reactivate(ctx, []int64{1}, "alumnus", schoolmembership.EnforceChildQuota)
			return err
		},
		// Skipping the Kinderkontingent is never a silent default (#3567).
		func() error { _, err := module.Reactivate(ctx, []int64{1}, "active", 0); return err },
		func() error { _, err := module.TransitionStatus(ctx, 1, "alumnus", "active"); return err },
		func() error { _, err := module.SetStatus(ctx, 1, "alumnus"); return err },
		func() error { _, err := module.EndCare(ctx, []int64{1}, "not-a-date"); return err },
		func() error { _, err := module.ResumeCare(ctx, 1, "2035-01-01", "active", ""); return err },
	} {
		if err := run(); err == nil {
			t.Fatal("invalid lifecycle command accepted")
		}
		if engine.calls != calls {
			t.Fatal("invalid lifecycle command reached persistence")
		}
	}
}
