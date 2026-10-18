package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// This file is the translation seam of the student write path (#3349): People
// Directory owns users.students and the departure plan, and the retained
// callers still carry users.Student rows. The ordering, the gates, the
// companion reconcile and the stranding refusal are the owner's; this only
// carries values across.

// StudentWriteCapability is the owner surface this seam translates to.
type StudentWriteCapability interface {
	peopleModule.StudentWriteCommand
}

// StudentEnrollmentStatus is the School Membership half of the child's
// lifecycle the write seam reaches: the status of the enrollment. The
// composition root binds the owner's public capability.
type StudentEnrollmentStatus interface {
	SetStatus(ctx context.Context, studentID int64, status string) (bool, error)
	TransitionStatus(ctx context.Context, studentID int64, expected, next string) (bool, error)
}

// StudentWrites adapts the owner's child lifecycle to the retained model-typed
// contract. It satisfies the write half of models/users.StudentRepository
// structurally, so the seam does not depend on that package's interface.
type StudentWrites struct {
	directory  StudentWriteCapability
	membership StudentEnrollmentStatus
}

func NewStudentWrites(directory StudentWriteCapability, membership StudentEnrollmentStatus) *StudentWrites {
	return &StudentWrites{directory: directory, membership: membership}
}

// Create inserts the child and reflects the stored row — the assigned id, the
// timestamps and the plan as persisted — back onto the caller's struct, which
// is the contract the retained callers rely on.
func (w *StudentWrites) Create(ctx context.Context, student *userModels.Student) error {
	if student == nil {
		return errors.New("student cannot be nil")
	}
	record, err := w.directory.CreateStudent(ctx, toOwnerStudentWrite(student))
	if err != nil {
		return translateStudentWriteError(err)
	}
	applyStudentRecord(student, record)
	return nil
}

func (w *StudentWrites) Update(ctx context.Context, student *userModels.Student) error {
	if student == nil {
		return errors.New("student cannot be nil")
	}
	record, err := w.directory.UpdateStudent(ctx, toOwnerStudentWrite(student))
	if err != nil {
		return translateStudentWriteError(err)
	}
	applyStudentRecord(student, record)
	return nil
}

func (w *StudentWrites) Delete(ctx context.Context, id int64) error {
	return translateStudentWriteError(w.directory.DeleteStudentRecord(ctx, id))
}

// SetEnrollmentStatus (the retained UpdateStatus), TransitionStatus,
// SetEnrolledUntilByIDs and SetEnrollmentWindowByID are the child's lifecycle
// and care window. They take the same class-writes gate every other student
// write does, because either can move a child into or out of a class a grade
// transition is mid-way through.
func (w *StudentWrites) SetEnrollmentStatus(ctx context.Context, studentID int64, status userModels.StudentStatus) error {
	changed, err := w.membership.SetStatus(ctx, studentID, string(status))
	if err == nil && !changed {
		err = peopleModule.ErrStudentNotFound
	}
	return translateStudentWriteError(err)
}

func (w *StudentWrites) TransitionStatus(
	ctx context.Context,
	studentID int64,
	expected, next userModels.StudentStatus,
) (bool, error) {
	moved, err := w.membership.TransitionStatus(ctx, studentID, string(expected), string(next))
	return moved, translateStudentWriteError(err)
}

func (w *StudentWrites) FindCareBoundsByIDs(ctx context.Context, ids []int64) (map[int64]userModels.CalendarDate, error) {
	values, err := w.directory.ListStudentCareEnds(ctx, ids)
	if err != nil {
		return nil, translateStudentWriteError(err)
	}
	bounds := make(map[int64]userModels.CalendarDate, len(values))
	for id, value := range values {
		if parsed := userModels.OptionalCalendarDate(value); parsed != nil {
			bounds[id] = *parsed
		}
	}
	return bounds, nil
}

func (w *StudentWrites) VerifyCompanionStrandingBatch(ctx context.Context) error {
	return translateStudentWriteError(w.directory.VerifyStudentStrandingBatch(ctx))
}

// toOwnerStudentWrite reads the plan and the baseline off the model. The
// baseline is only present when the read that produced this struct hydrated the
// plan; without one the owner takes the supplied fields at face value.
func toOwnerStudentWrite(student *userModels.Student) peopleModule.StudentWrite {
	write := peopleModule.StudentWrite{
		Record: studentModelToRecord(student),
		Plan: peopleModule.StudentPlan{
			AllowedDepartureModes: student.AllowedDepartureModes,
			DepartureDays:         student.DepartureDays,
			BusDays:               student.BusDays,
			PickupDays:            student.PickupDays,
			PickupStatus:          student.PickupStatus,
		},
		CompanionNote: student.DepartureCompanionNote,
		NoteSupplied:  student.DepartureCompanionNote != nil,
	}
	if baseline := student.DepartureBaseline; baseline != nil {
		write.Baseline = &peopleModule.StudentPlan{
			AllowedDepartureModes: baseline.AllowedDepartureModes,
			DepartureDays:         baseline.DepartureDays,
			BusDays:               baseline.BusDays,
			PickupDays:            baseline.PickupDays,
		}
	}
	return write
}

func studentModelToRecord(student *userModels.Student) peopleModule.StudentRecord {
	record := peopleModule.StudentRecord{
		ID: student.ID, TenantID: student.GetTenantID(),
		PersonID: student.PersonID, SchoolClass: student.SchoolClass,
		GroupID: student.GroupID, Status: string(student.Status),

		AddressStreet:     student.AddressStreet,
		AddressCity:       student.AddressCity,
		AddressPostalCode: student.AddressPostalCode,

		ExtraInfo:    student.ExtraInfo,
		HealthInfo:   student.HealthInfo,
		PickupStatus: student.PickupStatus,

		DepartureCompanionNote: student.DepartureCompanionNote,

		Sick:         student.Sick,
		SickSince:    student.SickSince,
		Excused:      student.Excused,
		ExcusedSince: student.ExcusedSince,

		PhotoPath:           student.PhotoPath,
		PhotoConsentGivenAt: student.PhotoConsentGivenAt,
		PhotoConsentGivenBy: student.PhotoConsentGivenBy,

		AGBAcceptedAt:            student.AGBAcceptedAt,
		DataProcessingAcceptedAt: student.DataProcessingAcceptedAt,
		EmailContactAcceptedAt:   student.EmailContactAcceptedAt,
	}
	if student.EnrolledFrom != nil {
		record.EnrolledFrom = student.EnrolledFrom.String()
	}
	if student.EnrolledUntil != nil {
		record.EnrolledUntil = student.EnrolledUntil.String()
	}
	return record
}

// applyStudentRecord writes the stored row back onto the caller's struct and
// records the persisted plan as the new baseline: the in-memory plan now equals
// the stored one, so reusing the same instance for a second write must not make
// this write's own result look like a pending caller change.
func applyStudentRecord(student *userModels.Student, record peopleModule.StudentRecord) {
	// Field by field rather than replacing the struct: the caller's value also
	// carries things the row does not — a preloaded Person, the companion days
	// it marked — and a write must not take those away.
	student.ID, student.CreatedAt, student.UpdatedAt = record.ID, record.CreatedAt, record.UpdatedAt
	student.SetTenantID(record.TenantID)
	student.PersonID, student.SchoolClass = record.PersonID, record.SchoolClass
	student.GroupID, student.Status = record.GroupID, userModels.StudentStatus(record.Status)
	student.EnrolledFrom = userModels.OptionalCalendarDate(record.EnrolledFrom)
	student.EnrolledUntil = userModels.OptionalCalendarDate(record.EnrolledUntil)

	student.AllowedDepartureModes = record.AllowedDepartureModes
	student.DepartureDays = record.DepartureDays
	student.BusDays = record.BusDays
	student.PickupDays = record.PickupDays
	student.PickupStatus = record.PickupStatus
	student.DepartureCompanionNote = record.DepartureCompanionNote

	// The in-memory plan now equals the stored one, so it is also the new
	// baseline: reusing the same value for a second write must not make this
	// write's own result look like a pending caller change.
	student.SnapshotDeparturePlan()
}

// translateStudentWriteError restates the owner's sentinels in the vocabulary
// the retained callers branch on.
func translateStudentWriteError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, peopleModule.ErrCompanionWouldLoseDeparture):
		return userModels.ErrCompanionWouldLoseDeparture
	case errors.Is(err, peopleModule.ErrCompanionLockBusy):
		return userModels.ErrCompanionLockBusy
	case errors.Is(err, peopleModule.ErrStudentNotFound):
		return userModels.ErrStudentRowMissing
	default:
		return err
	}
}

// studentRepositoryWithOwnerWrites keeps the retained read surface of the
// student repository and routes its four write entry points to People
// Directory, so every caller of the repository interface — the HTTP flows, the
// enrollment approval, the imports, the master-data review — goes through the
// owner without knowing it.
type studentRepositoryWithOwnerWrites struct {
	peoplePorts.StudentRepository
	writes *StudentWrites
	reads  *StudentReads
	// The teacher assignment is School Membership's, so the roster reads
	// resolve the groups through it first and then ask this owner for the
	// children of those groups.
	teacherGroupIDs      func(context.Context, int64) ([]int64, error)
	teacherStaffGroupIDs func(context.Context, []int64) ([]int64, error)
}

func (r studentRepositoryWithOwnerWrites) FindByTeacherIDWithGroups(
	ctx context.Context,
	teacherID int64,
) ([]*userModels.StudentWithGroupInfo, error) {
	if r.teacherGroupIDs == nil {
		return nil, errors.New("student repository resolves teacher assignments through School Membership")
	}
	groupIDs, err := r.teacherGroupIDs(ctx, teacherID)
	if err != nil {
		return nil, err
	}
	if len(groupIDs) == 0 {
		return []*userModels.StudentWithGroupInfo{}, nil
	}
	return r.reads.FindByTeacherIDWithGroups(ctx, groupIDs, userModels.TodayCalendarDate())
}

func (r studentRepositoryWithOwnerWrites) FindByTeacherStaffIDsWithGroups(
	ctx context.Context,
	staffIDs []int64,
) ([]*userModels.StudentWithGroupInfo, error) {
	if len(staffIDs) == 0 {
		return []*userModels.StudentWithGroupInfo{}, nil
	}
	if r.teacherStaffGroupIDs == nil {
		return nil, errors.New("student repository resolves teacher assignments through School Membership")
	}
	groupIDs, err := r.teacherStaffGroupIDs(ctx, staffIDs)
	if err != nil {
		return nil, err
	}
	if len(groupIDs) == 0 {
		return []*userModels.StudentWithGroupInfo{}, nil
	}
	return r.reads.FindByTeacherIDWithGroups(ctx, groupIDs, userModels.TodayCalendarDate())
}

func (r studentRepositoryWithOwnerWrites) FindAllWithGroups(ctx context.Context) ([]*userModels.StudentWithGroupInfo, error) {
	return r.reads.FindAllWithGroups(ctx, userModels.TodayCalendarDate())
}

func (r studentRepositoryWithOwnerWrites) FindOverlappingWithGroups(
	ctx context.Context,
	from, to, today userModels.CalendarDate,
) ([]*userModels.StudentWithGroupInfo, error) {
	return r.reads.FindOverlappingWithGroups(ctx, from, to, today)
}

// FindOverlappingWithGroupsOnDate is the meal-plan boundary: now is an instant
// so the immediate-activation rule uses the actual current Berlin day rather
// than applying the requested list date retroactively.
func (r studentRepositoryWithOwnerWrites) FindOverlappingWithGroupsOnDate(
	ctx context.Context,
	value string,
	now time.Time,
) ([]*userModels.StudentWithGroupInfo, error) {
	date := userModels.OptionalCalendarDate(value)
	if date == nil {
		return nil, fmt.Errorf("find students overlapping date: %q is not a calendar date", value)
	}
	return r.reads.FindOverlappingWithGroups(ctx, *date, *date, userModels.CalendarDateOf(now))
}

// The reads the owner now serves. Everything not listed falls through to the
// retained repository, which still holds the group-info joins.
func (r studentRepositoryWithOwnerWrites) FindByID(ctx context.Context, id any) (*userModels.Student, error) {
	return r.reads.FindByID(ctx, id)
}

func (r studentRepositoryWithOwnerWrites) FindByPersonID(ctx context.Context, personID int64) (*userModels.Student, error) {
	return r.reads.FindByPersonID(ctx, personID)
}

func (r studentRepositoryWithOwnerWrites) FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Student, error) {
	return r.reads.FindByIDs(ctx, ids)
}

func (r studentRepositoryWithOwnerWrites) FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Student, error) {
	return r.reads.FindReadScopeByIDs(ctx, ids)
}

func (r studentRepositoryWithOwnerWrites) FindByGroupID(ctx context.Context, groupID int64) ([]*userModels.Student, error) {
	return r.reads.FindByGroupID(ctx, groupID)
}

func (r studentRepositoryWithOwnerWrites) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*userModels.Student, error) {
	return r.reads.FindByGroupIDs(ctx, groupIDs)
}

func (r studentRepositoryWithOwnerWrites) FindPendingDueForActivation(ctx context.Context, asOf userModels.CalendarDate) ([]*userModels.Student, error) {
	return r.reads.FindPendingDueForActivation(ctx, asOf)
}

func (r studentRepositoryWithOwnerWrites) FindActiveDueForDeactivation(ctx context.Context, asOf userModels.CalendarDate) ([]*userModels.Student, error) {
	return r.reads.FindActiveDueForDeactivation(ctx, asOf)
}

func (r studentRepositoryWithOwnerWrites) FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*userModels.Student, error) {
	return r.reads.FindByIDsForUpdate(ctx, ids)
}

func (r studentRepositoryWithOwnerWrites) CountByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]int, error) {
	return r.reads.CountByGroupIDs(ctx, groupIDs)
}

func (r studentRepositoryWithOwnerWrites) ExistsEnrolledByNameAndBirthday(
	ctx context.Context, tenantID int64, firstName, lastName string, birthday userModels.CalendarDate,
) (bool, error) {
	return r.reads.ExistsEnrolledByNameAndBirthday(ctx, tenantID, firstName, lastName, birthday)
}

func (r studentRepositoryWithOwnerWrites) FindEnrolledStudentIDByNameAndBirthday(
	ctx context.Context, tenantID int64, firstName, lastName string, birthday userModels.CalendarDate,
) (*int64, error) {
	return r.reads.FindEnrolledStudentIDByNameAndBirthday(ctx, tenantID, firstName, lastName, birthday)
}

func (r studentRepositoryWithOwnerWrites) ListSchoolClasses(ctx context.Context) ([]string, error) {
	return r.reads.ListSchoolClasses(ctx)
}

func (r studentRepositoryWithOwnerWrites) ListByGroupIDsIncludingAlumni(ctx context.Context, groupIDs []int64) ([]*userModels.Student, error) {
	return r.reads.ListByGroupIDsIncludingAlumni(ctx, groupIDs)
}

func (r studentRepositoryWithOwnerWrites) ListClassRoster(ctx context.Context, schoolClass string) ([]*userModels.Student, error) {
	return r.reads.ListClassRoster(ctx, schoolClass)
}

func (r studentRepositoryWithOwnerWrites) CountEnrolled(ctx context.Context) (int, error) {
	return r.reads.CountEnrolled(ctx)
}

func (r studentRepositoryWithOwnerWrites) FindByIDForUpdate(ctx context.Context, id int64) (*userModels.Student, error) {
	return r.reads.FindByIDForUpdate(ctx, id)
}

func (r studentRepositoryWithOwnerWrites) Create(ctx context.Context, student *userModels.Student) error {
	return r.writes.Create(ctx, student)
}

func (r studentRepositoryWithOwnerWrites) Update(ctx context.Context, student *userModels.Student) error {
	return r.writes.Update(ctx, student)
}

// Delete keeps the retained interface's `any` id: its callers still pass an int
// or an int64, and the owner takes only an int64.
func (r studentRepositoryWithOwnerWrites) Delete(ctx context.Context, id any) error {
	studentID, ok := studentIDOf(id)
	if !ok {
		return fmt.Errorf("delete student: unsupported id type %T", id)
	}
	return r.writes.Delete(ctx, studentID)
}

func studentIDOf(id any) (int64, bool) {
	switch value := id.(type) {
	case int64:
		return value, true
	case int:
		return int64(value), true
	case int32:
		return int64(value), true
	case *int64:
		if value == nil {
			return 0, false
		}
		return *value, true
	default:
		return 0, false
	}
}

func (r studentRepositoryWithOwnerWrites) VerifyCompanionStrandingBatch(ctx context.Context) error {
	return r.writes.VerifyCompanionStrandingBatch(ctx)
}

func (r studentRepositoryWithOwnerWrites) UpdateStatus(ctx context.Context, studentID int64, status userModels.StudentStatus) error {
	return r.writes.SetEnrollmentStatus(ctx, studentID, status)
}

func (r studentRepositoryWithOwnerWrites) TransitionStatus(
	ctx context.Context,
	studentID int64,
	expected, next userModels.StudentStatus,
) (bool, error) {
	return r.writes.TransitionStatus(ctx, studentID, expected, next)
}

func (r studentRepositoryWithOwnerWrites) FindCareBoundsByIDs(ctx context.Context, ids []int64) (map[int64]userModels.CalendarDate, error) {
	return r.writes.FindCareBoundsByIDs(ctx, ids)
}

// StudentRepositoryDependencies are what the retained student repository
// composes: the child table's retained read surface, the owner it routes the
// reads and writes to, and the School Membership facts it cannot hold itself.
type StudentRepositoryDependencies struct {
	DB       *bun.DB
	Students usersRepo.StudentDirectoryRows
	// Directory is the People Directory owner of the child's row.
	Directory StudentRepositoryOwner
	// Enrollment is the School Membership status of the child's enrollment.
	Enrollment StudentEnrollmentStatus
	// TeacherGroupIDs and TeacherStaffGroupIDs resolve the groups a teacher
	// (or the teachers of the staff members) is assigned to. The assignment
	// is School Membership's; without a resolver the teacher-scoped roster
	// reads fail closed.
	TeacherGroupIDs      func(context.Context, int64) ([]int64, error)
	TeacherStaffGroupIDs func(context.Context, []int64) ([]int64, error)
}

// StudentRepositoryOwner is everything the composed repository needs from the
// owner: the child's lifecycle and the reads that replaced its own SQL.
type StudentRepositoryOwner interface {
	StudentWriteCapability
	StudentReadCapability
}

// NewStudentRepository composes the retained read surface of the child table
// with People Directory's reads and writes (#3349).
func NewStudentRepository(dependencies StudentRepositoryDependencies) peoplePorts.StudentRepository {
	repository := usersRepo.NewStudentRepository(NewLegacyRepositoryRuntime(dependencies.DB), dependencies.Students)
	if dependencies.Directory == nil {
		return repository
	}
	return studentRepositoryWithOwnerWrites{
		StudentRepository:    repository,
		writes:               NewStudentWrites(dependencies.Directory, dependencies.Enrollment),
		reads:                NewStudentReads(dependencies.Directory),
		teacherGroupIDs:      dependencies.TeacherGroupIDs,
		teacherStaffGroupIDs: dependencies.TeacherStaffGroupIDs,
	}
}
