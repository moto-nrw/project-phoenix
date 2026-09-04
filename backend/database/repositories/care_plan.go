package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	"github.com/moto-nrw/project-phoenix/database/repositories/audit"
	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	carePlanLegacy "github.com/moto-nrw/project-phoenix/modules/careplan/legacy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/uptrace/bun"
)

func NewCarePlan(db *bun.DB, students peopledirectory.Capability, slots scheduleModels.InstanceStudentRepository) (careplan.Capability, error) {
	if students == nil || slots == nil {
		return nil, errors.New("compose Care Plan: People Directory and instance-student repository are required")
	}
	statusStudents, err := CarePlanStatusStudents(students)
	if err != nil {
		return nil, err
	}
	studentLock, studentNotFound := CareStudentLock(students)
	capability, err := carePlanCompose.New(carePlanCompose.Dependencies{
		DB: db, Observe: func(carePlanCompose.Observation) {}, AmbientDB: carePlanLegacy.NewAmbientDatabase(db),
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
	if repository, ok := slots.(*scheduleRepo.InstanceStudentRepository); ok {
		repository.BindCarePlan(pickupExceptionDirectory{query: capability})
	}
	return capability, nil
}

type statusStudentDirectory struct {
	students peopledirectory.Capability
	flags    peopledirectory.StudentStatusFlagCapability
}

func CarePlanStatusStudents(students peopledirectory.Capability) (carePlanCompose.StatusStudentDirectory, error) {
	statusFlags, ok := students.(peopledirectory.StudentStatusFlagCapability)
	if !ok {
		return nil, errors.New("compose Care Plan: People Directory status-flag capability is required")
	}
	return statusStudentDirectory{students: students, flags: statusFlags}, nil
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
	return d.flags.ClearStudentStatusFlags(ctx, ids, status)
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
	return d.repository.ApplyStatusDay(ctx, studentID, carePlanLegacy.ScheduleDate(date), statusDayID, substatus)
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
	f.CareOffering = carePlanLegacy.NewCareOfferingRepository(capability)
	f.OfferingChangeRequest = carePlanLegacy.NewOfferingChangeRepository(capability, f.students)
	companion := carePlanLegacy.NewCompanionRepository(capability)
	f.StudentCompanion = companion
	f.StudentDocument = carePlanLegacy.NewCareDocumentRepository(capability)
	if repository, ok := f.Student.(*usersRepo.StudentRepository); ok {
		repository.BindCompanionRepository(companion)
	}
	f.bindCarePlanAuditDirectory()
	if repository, ok := f.PhaseExpiry.(*enrollmentRepo.PhaseExpiryRepository); ok {
		repository.BindCarePlan(phaseExpiryCarePlanDirectory{query: capability})
	}
	if repository, ok := f.CareExitCleanup.(*usersRepo.CareExitCleanupRepository); ok {
		repository.BindCarePlan(careExitCarePlanDirectory{capability: capability})
	}
	if repository, ok := f.CareExit.(*usersRepo.CareExitRepository); ok {
		repository.BindCarePlan(careExitDirectory{capability: capability})
	}
	if repository, ok := f.StudentDeletion.(*usersRepo.StudentDeletionRepository); ok {
		repository.BindCarePlan(studentDeletionCarePlanDirectory{capability: capability})
	}
	if repository, ok := f.InstanceStudent.(*scheduleRepo.InstanceStudentRepository); ok {
		repository.BindCarePlan(pickupExceptionDirectory{query: capability})
	}
	if repository, ok := f.Statistics.(*activeRepo.StatisticsRepository); ok {
		repository.BindCarePlan(statisticsCarePlanDirectory{query: capability})
	}
}

type statisticsCarePlanDirectory struct {
	query careplan.StudentStatusDaysQuery
}

func (d statisticsCarePlanDirectory) ListStatusDaySummaries(ctx context.Context, from, to string) ([]activeRepo.StatusDaySummary, error) {
	values, err := d.query.ListStatusDaySummaries(ctx, careplan.Date(from), careplan.Date(to))
	if err != nil {
		return nil, err
	}
	result := make([]activeRepo.StatusDaySummary, 0, len(values))
	for _, value := range values {
		result = append(result, activeRepo.StatusDaySummary{StudentID: value.StudentID, Date: value.Date.String(), Status: value.Status})
	}
	return result, nil
}

type studentDeletionCarePlanDirectory struct{ capability careplan.Capability }

func (d studentDeletionCarePlanDirectory) CountCompanionLinks(ctx context.Context, studentID int64) (int, error) {
	return d.capability.CountCompanionLinks(ctx, studentID)
}

func (d studentDeletionCarePlanDirectory) CountStudentScheduleRows(ctx context.Context, studentID int64) (int, error) {
	return d.capability.CountStudentScheduleRows(ctx, studentID)
}

func (d studentDeletionCarePlanDirectory) CountCarePlanDeletionRecords(ctx context.Context, studentID int64) (usersRepo.CarePlanDeletionCounts, error) {
	counts, err := d.capability.CountCarePlanDeletionRecords(ctx, studentID)
	if err != nil {
		return usersRepo.CarePlanDeletionCounts{}, err
	}
	return usersRepo.CarePlanDeletionCounts{
		StatusDays: counts.StatusDays, ExcusedRequests: counts.ExcusedRequests,
		CareRequests: counts.CareRequests, DataRequests: counts.DataRequests,
	}, nil
}

type pickupExceptionDirectory struct {
	query careplan.Query
}

func (d pickupExceptionDirectory) FindPickupException(ctx context.Context, id int64) (*scheduleRepo.PickupExceptionProjection, error) {
	value, err := d.query.FindPickupException(ctx, id, false)
	if errors.Is(err, careplan.ErrStudentScheduleNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &scheduleRepo.PickupExceptionProjection{ID: value.ID, StudentID: value.StudentID, ExceptionDate: value.ExceptionDate.String(), ExcusedFrom: value.ExcusedFrom, ExcusedAuto: value.ExcusedAuto}, nil
}

func (d pickupExceptionDirectory) ListPickupExceptions(ctx context.Context, filter scheduleRepo.PickupExceptionFilter) ([]scheduleRepo.PickupExceptionProjection, error) {
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
	result := make([]scheduleRepo.PickupExceptionProjection, 0, len(values))
	for _, value := range values {
		result = append(result, scheduleRepo.PickupExceptionProjection{ID: value.ID, StudentID: value.StudentID, ExceptionDate: value.ExceptionDate.String(), ExcusedFrom: value.ExcusedFrom, ExcusedAuto: value.ExcusedAuto})
	}
	return result, nil
}

func (d pickupExceptionDirectory) FindStudentStatusDay(ctx context.Context, id int64, activeOnly bool) (*scheduleRepo.StudentStatusDayProjection, error) {
	value, err := d.query.FindStudentStatusDay(ctx, id, activeOnly)
	if errors.Is(err, careplan.ErrStudentStatusDayNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &scheduleRepo.StudentStatusDayProjection{ID: value.ID, StudentID: value.StudentID, Date: value.Date.String(), Status: value.Status}, nil
}

func (d pickupExceptionDirectory) ListStudentStatusDays(ctx context.Context, filter scheduleRepo.StudentStatusDayFilter) ([]scheduleRepo.StudentStatusDayProjection, error) {
	values, err := d.query.ListStudentStatusDays(ctx, careplan.StudentStatusDayFilter{
		IDs: filter.IDs, StudentIDs: filter.StudentIDs, Date: careplan.Date(filter.Date),
		From: careplan.Date(filter.From), ActiveOnly: filter.ActiveOnly, LatestOnly: filter.LatestOnly,
	})
	if err != nil {
		return nil, err
	}
	result := make([]scheduleRepo.StudentStatusDayProjection, 0, len(values))
	for _, value := range values {
		result = append(result, scheduleRepo.StudentStatusDayProjection{ID: value.ID, StudentID: value.StudentID, Date: value.Date.String(), Status: value.Status})
	}
	return result, nil
}

type careExitDirectory struct{ capability careplan.Capability }

func (d careExitDirectory) FindCareExits(ctx context.Context, ids []int64) (map[int64]*usersRepo.CareExit, error) {
	values, err := d.capability.FindCareExits(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]*usersRepo.CareExit, len(values))
	for id, value := range values {
		row := &usersRepo.CareExit{StudentID: value.StudentID, Reason: value.Reason, ReasonNote: value.ReasonNote, RecordedBy: value.RecordedBy, WithdrawalCompletionID: value.WithdrawalCompletionID}
		row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
		if value.PreviousEnrolledUntil != nil {
			row.PreviousEnrolledUntil = usersRepo.CareExitDate(value.PreviousEnrolledUntil.String())
		}
		result[id] = row
	}
	return result, nil
}

func (d careExitDirectory) UpsertCareExit(ctx context.Context, row *usersRepo.CareExit) error {
	value := careplan.CareExit{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, Reason: row.Reason, ReasonNote: row.ReasonNote, RecordedBy: row.RecordedBy, WithdrawalCompletionID: row.WithdrawalCompletionID}
	if row.PreviousEnrolledUntil != nil {
		converted := careplan.Date(row.PreviousEnrolledUntil.String())
		value.PreviousEnrolledUntil = &converted
	}
	return d.capability.UpsertCareExit(ctx, value)
}

func (d careExitDirectory) DeleteCareExits(ctx context.Context, ids []int64) error {
	return d.capability.DeleteCareExits(ctx, ids)
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

type phaseExpiryCarePlanDirectory struct{ query careplan.Query }

func (d phaseExpiryCarePlanDirectory) ListCareOfferings(ctx context.Context) ([]enrollmentRepo.CareOfferingProjection, error) {
	values, err := d.query.ListCareOfferings(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderID})
	if err != nil {
		return nil, err
	}
	result := make([]enrollmentRepo.CareOfferingProjection, 0, len(values))
	for _, value := range values {
		result = append(result, enrollmentRepo.CareOfferingProjection{
			ID: value.ID, TenantID: value.TenantID, PhaseID: value.PhaseID,
			DaysOfWeekMode: value.DaysOfWeekMode, AvailableDays: value.AvailableDays, IsActive: value.IsActive,
		})
	}
	return result, nil
}

type careExitCarePlanDirectory struct{ capability careplan.Capability }

func (d careExitCarePlanDirectory) ListCareOfferings(ctx context.Context) ([]usersRepo.CareOfferingProjection, error) {
	values, err := d.capability.ListCareOfferings(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderID})
	if err != nil {
		return nil, err
	}
	result := make([]usersRepo.CareOfferingProjection, 0, len(values))
	for _, value := range values {
		result = append(result, usersRepo.CareOfferingProjection{
			ID: value.ID, TenantID: value.TenantID, Name: value.Name,
			DaysOfWeekMode: value.DaysOfWeekMode, AvailableDays: value.AvailableDays,
			CountsAsCare: value.CountsAsCare, SortOrder: value.SortOrder,
		})
	}
	return result, nil
}

func (d careExitCarePlanDirectory) LockCareOfferings(ctx context.Context, ids []int64) error {
	_, err := d.capability.ListCareOfferings(ctx, careplan.CareOfferingFilter{
		IDs: ids, LockForUpdate: true, Order: careplan.OfferingOrderID,
	})
	return err
}

func (d careExitCarePlanDirectory) ListPendingOfferingChanges(ctx context.Context, studentIDs []int64, lock bool) ([]usersRepo.PendingOfferingChange, error) {
	values, err := d.capability.ListOfferingChanges(ctx, careplan.OfferingChangeFilter{
		StudentIDs: studentIDs, Statuses: []string{careplan.OfferingChangePending},
		LockForUpdate: lock, Order: careplan.ChangeOrderCreated,
	})
	if err != nil {
		return nil, err
	}
	result := make([]usersRepo.PendingOfferingChange, 0, len(values))
	for _, value := range values {
		result = append(result, usersRepo.PendingOfferingChange{StudentID: value.StudentID})
	}
	return result, nil
}

func (d careExitCarePlanDirectory) ClosePendingOfferingChanges(ctx context.Context, studentIDs []int64, reason string, reviewedBy *int64, at time.Time) (int64, error) {
	return d.capability.ClosePendingOfferingChanges(ctx, studentIDs, reason, reviewedBy, at)
}

func (d careExitCarePlanDirectory) ListCareExitRemovals(ctx context.Context, studentIDs []int64) ([]usersRepo.CareExitRemoval, error) {
	values, err := d.capability.ListCareExitRemovals(ctx, studentIDs)
	return convertCareExitRecords[[]careplan.CareExitRemoval, []usersRepo.CareExitRemoval](values, err)
}

func (d careExitCarePlanDirectory) ListCareExitSourceRemovals(ctx context.Context, studentIDs []int64) ([]usersRepo.CareExitSourceRemoval, error) {
	values, err := d.capability.ListCareExitSourceRemovals(ctx, studentIDs)
	return convertCareExitRecords[[]careplan.CareExitSourceRemoval, []usersRepo.CareExitSourceRemoval](values, err)
}

func (d careExitCarePlanDirectory) RecordCareExitRemovals(ctx context.Context, values []usersRepo.CareExitRemoval) error {
	converted, err := convertCareExitRecords[[]usersRepo.CareExitRemoval, []careplan.CareExitRemoval](values, nil)
	if err != nil {
		return err
	}
	return d.capability.RecordCareExitRemovals(ctx, converted)
}

func (d careExitCarePlanDirectory) RecordCareExitSourceRemovals(ctx context.Context, values []usersRepo.CareExitSourceRemoval) error {
	converted, err := convertCareExitRecords[[]usersRepo.CareExitSourceRemoval, []careplan.CareExitSourceRemoval](values, nil)
	if err != nil {
		return err
	}
	return d.capability.RecordCareExitSourceRemovals(ctx, converted)
}

func convertCareExitRecords[From any, To any](values From, err error) (To, error) {
	var result To
	if err != nil {
		return result, err
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return result, err
	}
	return result, json.Unmarshal(encoded, &result)
}

func (d careExitCarePlanDirectory) DiscardCareExitRemovals(ctx context.Context, studentIDs []int64) error {
	return d.capability.DiscardCareExitRemovals(ctx, studentIDs)
}

func (d careExitCarePlanDirectory) LockStudentSchedulesForCareExit(ctx context.Context, studentIDs []int64, from string) error {
	filter := careplan.StudentScheduleFilter{StudentIDs: studentIDs, LockForUpdate: true}
	if _, err := d.capability.ListPickupSchedules(ctx, filter); err != nil {
		return err
	}
	if _, err := d.capability.ListArrivalSchedules(ctx, filter); err != nil {
		return err
	}
	filter.From = careplan.Date(from)
	if _, err := d.capability.ListPickupExceptions(ctx, filter); err != nil {
		return err
	}
	_, err := d.capability.ListArrivalExceptions(ctx, filter)
	return err
}

func (d careExitCarePlanDirectory) ListWeeklyPlanPatterns(ctx context.Context, studentIDs []int64) (map[int64][]string, error) {
	arrivals, err := d.capability.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: studentIDs})
	if err != nil {
		return nil, err
	}
	pickups, err := d.capability.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: studentIDs})
	if err != nil {
		return nil, err
	}
	weekdays := [...]string{"", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag"}
	patterns := make(map[int64][]string, len(studentIDs))
	for _, value := range arrivals {
		pattern := "Ankunft am " + weekdays[value.Weekday]
		if !value.ExpectedArrival.IsZero() {
			pattern += ": " + value.ExpectedArrival.Format("15:04")
		}
		patterns[value.StudentID] = append(patterns[value.StudentID], pattern)
	}
	for _, value := range pickups {
		patterns[value.StudentID] = append(patterns[value.StudentID], "Abholung am "+weekdays[value.Weekday]+": "+value.PickupTime.Format("15:04"))
	}
	for studentID := range patterns {
		sort.Strings(patterns[studentID])
	}
	return patterns, nil
}

func (d careExitCarePlanDirectory) EndStudentSchedulesForCareExit(ctx context.Context, studentIDs []int64, validUntil string) (int64, error) {
	return d.capability.EndStudentSchedulesForCareExit(ctx, studentIDs, careplan.Date(validUntil))
}

func (d careExitCarePlanDirectory) RestoreStudentSchedulesForCareExit(ctx context.Context, studentIDs []int64) (int64, error) {
	return d.capability.RestoreStudentSchedulesForCareExit(ctx, studentIDs)
}

func (d careExitCarePlanDirectory) ExistingPickupExceptionIDs(ctx context.Context, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return []int64{}, nil
	}
	values, err := d.capability.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{IDs: ids})
	if err != nil {
		return nil, err
	}
	result := make([]int64, 0, len(values))
	for _, value := range values {
		result = append(result, value.ID)
	}
	return result, nil
}

func (d careExitCarePlanDirectory) ExistingStudentStatusDayIDs(ctx context.Context, ids []int64) ([]int64, error) {
	return d.capability.ExistingStudentStatusDayIDs(ctx, ids)
}

func (d careExitCarePlanDirectory) CountOpenCareRequests(ctx context.Context, studentIDs []int64) (map[int64]int, error) {
	return d.capability.CountOpenCareRequests(ctx, studentIDs)
}

func (d careExitCarePlanDirectory) LockOpenCareRequests(ctx context.Context, studentIDs []int64) error {
	return d.capability.LockOpenCareRequests(ctx, studentIDs)
}

func (d careExitCarePlanDirectory) CloseOpenCareRequests(ctx context.Context, studentIDs []int64, reason string, reviewedBy *int64, at time.Time) (int64, error) {
	return d.capability.CloseOpenCareRequests(ctx, studentIDs, reason, reviewedBy, at)
}
