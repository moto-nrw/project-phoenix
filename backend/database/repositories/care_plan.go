package repositories

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/database/repositories/audit"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

func NewCarePlan(db *bun.DB, students peopledirectory.Capability, slots scheduleModels.InstanceStudentRepository) (careplan.Capability, error) {
	if students == nil || slots == nil {
		return nil, errors.New("compose Care Plan: People Directory and instance-student repository are required")
	}
	statusStudents, err := CarePlanStatusStudents(db, students)
	if err != nil {
		return nil, err
	}
	studentLock, studentNotFound := CareStudentLock(students)
	capability, err := carePlanCompose.New(carePlanCompose.Dependencies{
		DB: db, Observe: func(carePlanCompose.Observation) {}, AmbientDB: carePlanCompose.TenantAmbientDatabase(db),
		StatusStudents: statusStudents,
		StatusSlots:    CarePlanStatusSlots(slots),
		People: carePlanCompose.StudentNameFinderFunc(func(ctx context.Context, ids []int64) ([]carePlanCompose.StudentName, error) {
			values, err := students.ListStudentNamesByID(ctx, ids)
			if err != nil {
				return nil, err
			}
			result := make([]carePlanCompose.StudentName, 0, len(values))
			for _, value := range values {
				result = append(result, carePlanCompose.StudentName{StudentID: value.StudentID, FirstName: value.FirstName, LastName: value.LastName})
			}
			return result, nil
		}),
		StudentLock: studentLock, StudentNotFound: studentNotFound,
	})
	if err != nil {
		return nil, err
	}
	return capability, nil
}

type statusStudentDirectory struct {
	students peopledirectory.Capability
	flags    peopledirectory.StudentStatusFlagCapability
	care     careplan.StudentProfileCommands
}

func CarePlanStatusStudents(db *bun.DB, students peopledirectory.Capability) (carePlanCompose.StatusStudentDirectory, error) {
	statusFlags, ok := students.(peopledirectory.StudentStatusFlagCapability)
	if !ok {
		return nil, errors.New("compose Care Plan: People Directory status-flag capability is required")
	}
	care, err := carePlanCompose.NewStudentProfiles(db, func(carePlanCompose.Observation) {})
	if err != nil {
		return nil, err
	}
	return statusStudentDirectory{students: students, flags: statusFlags, care: care}, nil
}

func (d statusStudentDirectory) ListEnrolledStudents(ctx context.Context) ([]carePlanCompose.StatusStudent, error) {
	students, err := d.students.ListEnrolledStudents(ctx)
	return statusStudents(students), err
}

func (d statusStudentDirectory) ListStudentsWithStatusFlag(ctx context.Context, status string) ([]carePlanCompose.StatusStudent, error) {
	students, err := d.flags.ListStudentsWithStatusFlag(ctx, status)
	return statusStudents(students), err
}

func (d statusStudentDirectory) ClearStudentStatusFlags(ctx context.Context, ids []int64, status string) (int64, error) {
	return d.care.ClearStudentStatusFlags(ctx, ids, status)
}

func (d statusStudentDirectory) LockStudent(ctx context.Context, id int64) error {
	return d.students.LockStudent(ctx, id)
}

func statusStudents(values []peopledirectory.Student) []carePlanCompose.StatusStudent {
	result := make([]carePlanCompose.StatusStudent, 0, len(values))
	for _, value := range values {
		result = append(result, carePlanCompose.StatusStudent{
			ID: value.ID, TenantID: value.TenantID, Status: value.Status,
			Sick: value.Sick, SickSince: value.SickSince, Excused: value.Excused, ExcusedSince: value.ExcusedSince,
		})
	}
	return result
}

type statusSlotDirectory struct {
	repository scheduleModels.InstanceStudentRepository
}

func CarePlanStatusSlots(repository scheduleModels.InstanceStudentRepository) carePlanCompose.StatusSlotDirectory {
	if repository == nil {
		return nil
	}
	return statusSlotDirectory{repository: repository}
}

func (d statusSlotDirectory) ApplyStatusDay(ctx context.Context, studentID int64, date careplan.Date, statusDayID int64, substatus string) (int, error) {
	return d.repository.ApplyStatusDay(ctx, studentID, scheduleModels.Date(date), statusDayID, substatus)
}

func (d statusSlotDirectory) ReleaseStatusDay(ctx context.Context, statusDayID int64) (int, error) {
	return d.repository.ReleaseStatusDay(ctx, statusDayID)
}

// BindCarePlan replaces the bootstrap adapters with the observed Care Plan
// capability composed by the production root.
func (f *Factory) BindCarePlan(capability careplan.Capability) {
	if capability == nil {
		panic("repository factory: care plan capability is required")
	}
	if f.carePlanBound {
		return
	}
	f.carePlanBound = true
	f.bindCarePlanAdapters(capability)
}

// CarePlan returns the capability used by the legacy repository adapters.
func (f *Factory) CarePlan() careplan.Capability { return f.carePlan }

func (f *Factory) bindCarePlanAdapters(capability careplan.Capability) {
	f.carePlan = capability
	f.StudentArrivalSchedule = NewArrivalScheduleRepository(capability)
	f.StudentArrivalException = NewArrivalExceptionRepository(capability)
	f.StudentArrivalNote = NewArrivalNoteRepository(capability)
	f.StudentPickupSchedule = NewPickupScheduleRepository(capability)
	f.StudentPickupException = NewPickupExceptionRepository(capability)
	f.StudentPickupNote = NewPickupNoteRepository(capability)
	f.ExcusedAbsenceRequest = NewExcusedAbsenceRequestRepository(capability)
	f.CareScheduleChangeRequest = NewCareScheduleChangeRequestRepository(capability)
	f.StudentDataChangeRequest = NewStudentDataChangeRequestRepository(capability)
	f.StudentStatusDay = NewStudentStatusDayRepository(capability)
	f.bindCarePlanAuditDirectory()
}

type pickupExceptionDirectory struct {
	query carePlanCompose.ExceptionQueries
}

func (d pickupExceptionDirectory) FindPickupException(ctx context.Context, id int64) (*timetableCompose.PickupExceptionProjection, error) {
	value, err := d.query.FindPickupException(ctx, id, false)
	if errors.Is(err, careplan.ErrStudentScheduleNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &timetableCompose.PickupExceptionProjection{ID: value.ID, StudentID: value.StudentID, ExceptionDate: value.ExceptionDate.String(), ExcusedFrom: value.ExcusedFrom, ExcusedAuto: value.ExcusedAuto}, nil
}

func (d pickupExceptionDirectory) ListPickupExceptions(ctx context.Context, filter timetableCompose.PickupExceptionFilter) ([]timetableCompose.PickupExceptionProjection, error) {
	ownerFilter := careplan.StudentScheduleFilter{IDs: filter.IDs, StudentIDs: filter.StudentIDs}
	if filter.Date != "" {
		ownerFilter.Date = careplan.Date(filter.Date)
	}
	if filter.From != "" {
		ownerFilter.From = careplan.Date(filter.From)
	}
	values, err := d.query.ListPickupExceptions(ctx, ownerFilter)
	if err != nil {
		return nil, err
	}
	result := make([]timetableCompose.PickupExceptionProjection, 0, len(values))
	for _, value := range values {
		result = append(result, timetableCompose.PickupExceptionProjection{ID: value.ID, StudentID: value.StudentID, ExceptionDate: value.ExceptionDate.String(), ExcusedFrom: value.ExcusedFrom, ExcusedAuto: value.ExcusedAuto})
	}
	return result, nil
}

func (d pickupExceptionDirectory) FindStudentStatusDay(ctx context.Context, id int64, activeOnly bool) (*timetableCompose.StudentStatusDayProjection, error) {
	value, err := d.query.FindStudentStatusDay(ctx, id, activeOnly)
	if errors.Is(err, careplan.ErrStudentStatusDayNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &timetableCompose.StudentStatusDayProjection{ID: value.ID, StudentID: value.StudentID, Date: value.Date.String(), Status: value.Status}, nil
}

func (d pickupExceptionDirectory) ListStudentStatusDays(ctx context.Context, filter timetableCompose.StudentStatusDayFilter) ([]timetableCompose.StudentStatusDayProjection, error) {
	values, err := d.query.ListStudentStatusDays(ctx, careplan.StudentStatusDayFilter{
		IDs: filter.IDs, StudentIDs: filter.StudentIDs, Date: careplan.Date(filter.Date),
		From: careplan.Date(filter.From), ActiveOnly: filter.ActiveOnly, LatestOnly: filter.LatestOnly,
	})
	if err != nil {
		return nil, err
	}
	result := make([]timetableCompose.StudentStatusDayProjection, 0, len(values))
	for _, value := range values {
		result = append(result, timetableCompose.StudentStatusDayProjection{ID: value.ID, StudentID: value.StudentID, Date: value.Date.String(), Status: value.Status})
	}
	return result, nil
}

func (f *Factory) bindCarePlanAuditDirectory() {
	if f.carePlan == nil {
		return
	}
	if repository, ok := f.BookingConsistency.(interface {
		BindCarePlan(audit.CareOfferingDirectory)
	}); ok {
		repository.BindCarePlan(auditCarePlanDirectory{query: f.carePlan})
	}
}

type auditCarePlanDirectory struct{ query careplan.Query }

func (d auditCarePlanDirectory) ListCareOfferings(ctx context.Context) ([]audit.CareOfferingProjection, error) {
	values, err := d.query.ListCareOfferings(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderID})
	if err != nil {
		return nil, err
	}
	result := make([]audit.CareOfferingProjection, 0, len(values))
	for _, value := range values {
		result = append(result, audit.CareOfferingProjection{
			ID: value.ID, TenantID: value.TenantID, PhaseID: value.PhaseID,
			DaysOfWeekMode: value.DaysOfWeekMode, AvailableDays: value.AvailableDays,
			IsActive: value.IsActive, IsRequired: value.IsRequired,
			CountsAsCare: value.CountsAsCare, PickupTimes: value.PickupTimes,
		})
	}
	return result, nil
}
