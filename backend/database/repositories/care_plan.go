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
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	carePlanLegacy "github.com/moto-nrw/project-phoenix/modules/careplan/legacy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/uptrace/bun"
)

func NewCarePlan(db *bun.DB, students peopledirectory.Capability) (careplan.Capability, error) {
	if students == nil {
		return nil, errors.New("compose Care Plan: People Directory capability is required")
	}
	studentLock, studentNotFound := CareStudentLock(students)
	return carePlanCompose.New(carePlanCompose.Dependencies{
		DB: db, Observe: func(carePlanCompose.Observation) {}, AmbientDB: carePlanLegacy.NewAmbientDatabase(db),
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
		repository.BindCarePlan(capability)
	}
	if repository, ok := f.InstanceStudent.(*scheduleRepo.InstanceStudentRepository); ok {
		repository.BindCarePlan(pickupExceptionDirectory{query: capability})
	}
	if repository, ok := f.StudentStatusDay.(*activeRepo.StudentStatusDayRepository); ok {
		repository.BindCarePlan(pickupExceptionDirectory{query: capability})
	}
}

type pickupExceptionDirectory struct {
	query careplan.StudentSchedulesQuery
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
