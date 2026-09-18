package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
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

// StudentWrites adapts the owner's child lifecycle to the retained model-typed
// contract. It satisfies the write half of models/users.StudentRepository
// structurally, so the seam does not depend on that package's interface.
type StudentWrites struct{ directory StudentWriteCapability }

func NewStudentWrites(directory StudentWriteCapability) *StudentWrites {
	return &StudentWrites{directory: directory}
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

		GuardianName:    student.GuardianName,
		GuardianContact: student.GuardianContact,
		GuardianEmail:   student.GuardianEmail,
		GuardianPhone:   student.GuardianPhone,

		AddressStreet:     student.AddressStreet,
		AddressCity:       student.AddressCity,
		AddressPostalCode: student.AddressPostalCode,

		ExtraInfo:       student.ExtraInfo,
		SupervisorNotes: student.SupervisorNotes,
		HealthInfo:      student.HealthInfo,
		PickupStatus:    student.PickupStatus,

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

// careplanCompanions binds Care Plan's companion contract behind the People
// Directory write path's seam. Care Plan owns users.student_companions; the
// directory only asks which links a narrowed plan can no longer justify.
type careplanCompanions struct {
	carePlan careplanCompose.CompanionRecords
}

// NewStudentCompanionSeam binds Care Plan's companion slice behind People
// Directory's write path.
func NewStudentCompanionSeam(carePlan careplanCompose.CompanionRecords) peopleCompose.StudentCompanions {
	return careplanCompanions{carePlan: carePlan}
}

func (c careplanCompanions) ListCompanionEdgesForStudent(
	ctx context.Context,
	studentID int64,
) ([]peopleCompose.StudentCompanionEdge, error) {
	edges, err := c.carePlan.ListCompanionEdges(ctx, studentID)
	if err != nil {
		return nil, err
	}
	result := make([]peopleCompose.StudentCompanionEdge, 0, len(edges))
	for _, edge := range edges {
		// Care Plan stores the pair ordered by id; which side is "the student"
		// is the reader's question, and the owner answers it per edge.
		result = append(result, peopleCompose.StudentCompanionEdge{
			ID: edge.ID, StudentID: edge.StudentLowID,
			CompanionStudentID: edge.StudentHighID, Weekday: edge.Weekday,
		})
	}
	return result, nil
}

func (c careplanCompanions) CompanionDaysCoveredExcluding(
	ctx context.Context,
	studentIDs []int64,
	excludeID int64,
) (map[int64]map[string]bool, error) {
	return c.carePlan.CompanionDaysCoveredExcluding(ctx, studentIDs, excludeID)
}

// DeleteCompanionEdges also records that this write touched links, which is the
// only honest basis for announcing student_companions_changed. It belongs here
// rather than in the owner: the signal is carried on the caller's context, and
// this is the moment the links actually change.
func (c careplanCompanions) DeleteCompanionEdges(ctx context.Context, edgeIDs []int64) error {
	if err := c.carePlan.DeleteCompanionEdges(ctx, edgeIDs); err != nil {
		return err
	}
	if len(edgeIDs) > 0 {
		userModels.RecordCompanionChange(ctx)
	}
	return nil
}

// studentRepositoryWithOwnerWrites keeps the retained read surface of the
// student repository and routes its four write entry points to People
// Directory, so every caller of the repository interface — the HTTP flows, the
// enrollment approval, the imports, the master-data review — goes through the
// owner without knowing it.
type studentRepositoryWithOwnerWrites struct {
	userModels.StudentRepository
	writes *StudentWrites
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

// The composition root installs the retained repository's read-side
// dependencies by type assertion, so the wrapper has to carry them through —
// otherwise binding a teacher's groups would silently stop reaching the reads.
func (r studentRepositoryWithOwnerWrites) BindTeacherGroupIDs(query func(context.Context, int64) ([]int64, error)) {
	if binder, ok := r.StudentRepository.(interface {
		BindTeacherGroupIDs(func(context.Context, int64) ([]int64, error))
	}); ok {
		binder.BindTeacherGroupIDs(query)
	}
}

func (r studentRepositoryWithOwnerWrites) BindTeacherStaffGroupIDs(query func(context.Context, []int64) ([]int64, error)) {
	if binder, ok := r.StudentRepository.(interface {
		BindTeacherStaffGroupIDs(func(context.Context, []int64) ([]int64, error))
	}); ok {
		binder.BindTeacherStaffGroupIDs(query)
	}
}

// bindStudentWrites routes the retained repository's writes through the owner.
func bindStudentWrites(repository userModels.StudentRepository, directory StudentWriteCapability) userModels.StudentRepository {
	if repository == nil || directory == nil {
		return repository
	}
	return studentRepositoryWithOwnerWrites{
		StudentRepository: repository,
		writes:            NewStudentWrites(directory),
	}
}

// NewStudentRepository composes the retained read surface of the child table
// with People Directory's writes (#3349). Every graph gets the owner by
// default; the observed directory replaces it in BindPeopleDirectory.
func NewStudentRepository(db *bun.DB) userModels.StudentRepository {
	return bindStudentWrites(usersRepo.NewStudentRepository(db), MustNewPeopleDirectory(db))
}
