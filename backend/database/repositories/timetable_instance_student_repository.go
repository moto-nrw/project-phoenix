package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type timetableInstanceStudentRepository struct {
	timetable timetable.InstanceStudentCapability
}

type timetableInstanceStudentProxy struct {
	scheduleModels.InstanceStudentRepository
	carePlan scheduleRepo.PickupExceptionDirectory
}

func newTimetableInstanceStudentProxy() *timetableInstanceStudentProxy {
	return &timetableInstanceStudentProxy{}
}

func (p *timetableInstanceStudentProxy) Bind(capability timetable.InstanceStudentCapability) {
	if capability == nil {
		panic("instance student compatibility: timetable owner is required")
	}
	repository := timetableInstanceStudentRepository{timetable: capability}
	if p.carePlan != nil {
		repository.BindCarePlan(p.carePlan)
	}
	p.InstanceStudentRepository = repository
}

func (p *timetableInstanceStudentProxy) BindCarePlan(query scheduleRepo.PickupExceptionDirectory) {
	if query == nil {
		panic("instance student compatibility: Care Plan is required")
	}
	p.carePlan = query
	if p.InstanceStudentRepository != nil {
		p.InstanceStudentRepository.(interface {
			BindCarePlan(scheduleRepo.PickupExceptionDirectory)
		}).BindCarePlan(query)
	}
}

func (p *timetableInstanceStudentProxy) FindPartialAbsenceBlocks(ctx context.Context, studentID int64, date scheduleRepo.InstanceStudentDate, cutoff time.Time) ([]scheduleModels.PartialAbsenceBlock, error) {
	repository, ok := p.InstanceStudentRepository.(interface {
		FindPartialAbsenceBlocks(context.Context, int64, scheduleRepo.InstanceStudentDate, time.Time) ([]scheduleModels.PartialAbsenceBlock, error)
	})
	if !ok {
		panic("instance student compatibility: timetable owner is not bound")
	}
	return repository.FindPartialAbsenceBlocks(ctx, studentID, date, cutoff)
}

func (r timetableInstanceStudentRepository) BindCarePlan(query scheduleRepo.PickupExceptionDirectory) {
	binder, ok := r.timetable.(timetable.CarePlanBinder)
	if !ok {
		panic("instance student compatibility: timetable owner cannot bind Care Plan")
	}
	binder.BindCarePlan(timetableCarePlanDirectory{query: query})
}

type timetableCarePlanDirectory struct {
	query scheduleRepo.PickupExceptionDirectory
}

func (d timetableCarePlanDirectory) FindPickupException(ctx context.Context, id int64) (*timetable.PickupException, error) {
	value, err := d.query.FindPickupException(ctx, id)
	if value == nil || err != nil {
		return nil, err
	}
	result := timetable.PickupException(*value)
	return &result, nil
}

func (d timetableCarePlanDirectory) ListPickupExceptions(ctx context.Context, filter timetable.PickupExceptionFilter) ([]timetable.PickupException, error) {
	values, err := d.query.ListPickupExceptions(ctx, scheduleRepo.PickupExceptionFilter(filter))
	result := make([]timetable.PickupException, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.PickupException(value))
	}
	return result, err
}

func (d timetableCarePlanDirectory) FindStudentStatusDay(ctx context.Context, id int64, activeOnly bool) (*timetable.StudentStatusDay, error) {
	value, err := d.query.FindStudentStatusDay(ctx, id, activeOnly)
	if value == nil || err != nil {
		return nil, err
	}
	result := timetable.StudentStatusDay(*value)
	return &result, nil
}

func (d timetableCarePlanDirectory) ListStudentStatusDays(ctx context.Context, filter timetable.StudentStatusDayFilter) ([]timetable.StudentStatusDay, error) {
	values, err := d.query.ListStudentStatusDays(ctx, scheduleRepo.StudentStatusDayFilter(filter))
	result := make([]timetable.StudentStatusDay, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.StudentStatusDay(value))
	}
	return result, err
}

func (r timetableInstanceStudentRepository) Create(ctx context.Context, value *scheduleModels.InstanceStudent) error {
	if value == nil {
		return errors.New("instance student cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	created, err := r.timetable.CreateInstanceStudent(ctx, publicInstanceStudentInput(value))
	if err != nil {
		return scheduleRepo.WrapDatabaseError("create", err)
	}
	replaceLegacyInstanceStudent(value, created)
	return nil
}

func (r timetableInstanceStudentRepository) FindByID(ctx context.Context, id any) (*scheduleModels.InstanceStudent, error) {
	rowID, ok := legacyGroupID(id)
	if !ok {
		return nil, scheduleRepo.WrapDatabaseError("find by id", fmt.Errorf("invalid instance student id %T", id))
	}
	if rowID <= 0 {
		return nil, scheduleRepo.WrapNotFoundDatabaseError("find by id")
	}
	value, err := r.timetable.FindInstanceStudent(ctx, rowID)
	if err != nil {
		return nil, legacyInstanceStudentError("find by id", err)
	}
	return legacyInstanceStudent(value), nil
}

func (r timetableInstanceStudentRepository) Update(ctx context.Context, value *scheduleModels.InstanceStudent) error {
	if value == nil {
		return errors.New("instance student cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	updated, err := r.timetable.UpdateInstanceStudent(ctx, value.ID, publicInstanceStudentInput(value))
	if err != nil {
		return legacyInstanceStudentError("update", err)
	}
	replaceLegacyInstanceStudent(value, updated)
	return nil
}

func (r timetableInstanceStudentRepository) Delete(ctx context.Context, id any) error {
	rowID, ok := legacyGroupID(id)
	if !ok {
		return scheduleRepo.WrapDatabaseError("delete", fmt.Errorf("invalid instance student id %T", id))
	}
	if err := r.timetable.DeleteInstanceStudent(ctx, rowID); err != nil {
		return scheduleRepo.WrapDatabaseError("delete", err)
	}
	return nil
}

func (r timetableInstanceStudentRepository) List(ctx context.Context, options *scheduleRepo.InstanceStudentQueryOptions) ([]*scheduleModels.InstanceStudent, error) {
	filter, err := scheduleRepo.InstanceStudentListOptions(options)
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("list with options", err)
	}
	return r.list(ctx, timetable.InstanceStudentFilter{IDs: filter.IDs, InstanceIDs: filter.InstanceIDs,
		StudentIDs: filter.StudentIDs, Status: filter.Status, Limit: filter.Limit, Offset: filter.Offset}, "list with options")
}

func (r timetableInstanceStudentRepository) FindByInstanceID(ctx context.Context, instanceID int64) ([]*scheduleModels.InstanceStudent, error) {
	return r.list(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{instanceID}, OrderByCreated: true}, "find by instance id")
}

func (r timetableInstanceStudentRepository) FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error) {
	if len(instanceIDs) == 0 {
		return []*scheduleModels.InstanceStudent{}, nil
	}
	return r.list(ctx, timetable.InstanceStudentFilter{InstanceIDs: instanceIDs, OrderByInstanceStudent: true}, "find by instance ids")
}

func (r timetableInstanceStudentRepository) FindExpectedByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error) {
	if len(instanceIDs) == 0 {
		return []*scheduleModels.InstanceStudent{}, nil
	}
	status := timetable.InstanceAttendanceExpected
	return r.list(ctx, timetable.InstanceStudentFilter{InstanceIDs: instanceIDs, Status: &status, OrderByInstanceStudent: true}, "find expected by instance ids")
}

func (r timetableInstanceStudentRepository) FindNotScheduledCandidatesByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error) {
	if len(instanceIDs) == 0 {
		return []*scheduleModels.InstanceStudent{}, nil
	}
	return r.list(ctx, timetable.InstanceStudentFilter{InstanceIDs: instanceIDs,
		NotScheduledCandidatesOnly: true, OrderByInstanceStudent: true}, "find not scheduled candidates by instance ids")
}

func (r timetableInstanceStudentRepository) CountNonAbsentByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error) {
	instanceIDs = positiveInstanceStudentIDs(instanceIDs)
	result, err := r.timetable.CountNonAbsentInstanceStudents(ctx, instanceIDs)
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("count non-absent by instance ids", err)
	}
	return result, nil
}

func (r timetableInstanceStudentRepository) FindPresentInOtherActiveInstances(ctx context.Context, excludedID int64, date scheduleRepo.InstanceStudentDate, studentIDs []int64) ([]scheduleModels.ParallelPresence, error) {
	studentIDs = positiveInstanceStudentIDs(studentIDs)
	values, err := r.timetable.ListParallelStudentPresence(ctx, excludedID, date.String(), studentIDs)
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("find present in other active instances", err)
	}
	result := make([]scheduleModels.ParallelPresence, 0, len(values))
	for _, value := range values {
		result = append(result, scheduleModels.ParallelPresence(value))
	}
	return result, nil
}

func (r timetableInstanceStudentRepository) FindPartialAbsenceBlocks(ctx context.Context, studentID int64, date scheduleRepo.InstanceStudentDate, cutoff time.Time) ([]scheduleModels.PartialAbsenceBlock, error) {
	values, err := r.timetable.ListPartialAbsenceBlocks(ctx, studentID, date.String(), cutoff)
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("find partial absence blocks", err)
	}
	result := make([]scheduleModels.PartialAbsenceBlock, 0, len(values))
	for _, value := range values {
		result = append(result, scheduleModels.PartialAbsenceBlock(value))
	}
	return result, nil
}

func (r timetableInstanceStudentRepository) FindByStudentAndDateRange(ctx context.Context, studentID int64, from, to scheduleRepo.InstanceStudentDate) ([]*scheduleModels.InstanceStudent, error) {
	fromText, toText := from.String(), to.String()
	return r.list(ctx, timetable.InstanceStudentFilter{StudentIDs: []int64{studentID}, FromDate: &fromText,
		ToDate: &toText, OrderByActivityDateTime: true}, "find by student and date range")
}

func (r timetableInstanceStudentRepository) FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date scheduleRepo.InstanceStudentDate) ([]*scheduleModels.InstanceStudent, error) {
	if len(studentIDs) == 0 {
		return []*scheduleModels.InstanceStudent{}, nil
	}
	text := date.String()
	return r.list(ctx, timetable.InstanceStudentFilter{StudentIDs: studentIDs, Date: &text,
		OrderByStudentActivityTime: true}, "find by student ids and date")
}

func (r timetableInstanceStudentRepository) FindByInstanceAndStudent(ctx context.Context, instanceID, studentID int64) (*scheduleModels.InstanceStudent, error) {
	values, err := r.list(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{instanceID}, StudentIDs: []int64{studentID}}, "find by instance and student")
	if err != nil || len(values) == 0 {
		return nil, err
	}
	return values[0], nil
}

func (r timetableInstanceStudentRepository) FindCurrentCandidates(ctx context.Context, studentID int64, date scheduleRepo.InstanceStudentDate, at time.Time) ([]*scheduleModels.InstanceStudent, error) {
	return r.findCurrent(ctx, []int64{studentID}, date, at, "find current student slot candidates")
}

func (r timetableInstanceStudentRepository) FindCurrentCandidatesByStudentIDs(ctx context.Context, studentIDs []int64, date scheduleRepo.InstanceStudentDate, at time.Time) ([]*scheduleModels.InstanceStudent, error) {
	if len(studentIDs) == 0 {
		return []*scheduleModels.InstanceStudent{}, nil
	}
	return r.findCurrent(ctx, studentIDs, date, at, "find current student slot candidates batch")
}

func (r timetableInstanceStudentRepository) findCurrent(ctx context.Context, studentIDs []int64, date scheduleRepo.InstanceStudentDate, at time.Time, operation string) ([]*scheduleModels.InstanceStudent, error) {
	dateText, clock := date.String(), scheduleRepo.InstanceStudentWallClock(at)
	return r.list(ctx, timetable.InstanceStudentFilter{StudentIDs: studentIDs, Date: &dateText, CurrentTime: &clock,
		OrderByStudentActivityTime: true}, operation)
}

func (r timetableInstanceStudentRepository) DeleteByInstanceID(ctx context.Context, instanceID int64) error {
	if err := r.timetable.DeleteInstanceStudentsByInstance(ctx, instanceID); err != nil {
		return scheduleRepo.WrapDatabaseError("delete by instance id", err)
	}
	return nil
}

func (r timetableInstanceStudentRepository) UpdateAttendanceFromCheckin(ctx context.Context, instanceID, studentID int64, checkedInAt time.Time) (bool, error) {
	updated, err := r.timetable.UpdateAttendanceFromCheckin(ctx, instanceID, studentID, checkedInAt)
	if err != nil {
		return false, scheduleRepo.WrapDatabaseError("update attendance from checkin", err)
	}
	return updated, nil
}

func (r timetableInstanceStudentRepository) UpdateAttendanceFromCheckinBatch(ctx context.Context, keys []scheduleModels.InstanceStudentKey, checkedInAt time.Time) error {
	if len(keys) == 0 {
		return nil
	}
	if err := r.timetable.UpdateAttendanceFromCheckinBatch(ctx, publicInstanceStudentKeys(keys), checkedInAt); err != nil {
		return scheduleRepo.WrapDatabaseError("update attendance from checkin batch", err)
	}
	return nil
}

func (r timetableInstanceStudentRepository) UpdateAttendanceCheckout(ctx context.Context, instanceID, studentID int64, checkedOutAt time.Time) error {
	if err := r.timetable.UpdateAttendanceCheckout(ctx, instanceID, studentID, checkedOutAt); err != nil {
		return scheduleRepo.WrapDatabaseError("update slot attendance checkout", err)
	}
	return nil
}

func (r timetableInstanceStudentRepository) UpdateAttendanceCheckoutBatch(ctx context.Context, keys []scheduleModels.InstanceStudentKey, checkedOutAt time.Time) error {
	if len(keys) == 0 {
		return nil
	}
	if err := r.timetable.UpdateAttendanceCheckoutBatch(ctx, publicInstanceStudentKeys(keys), checkedOutAt); err != nil {
		return scheduleRepo.WrapDatabaseError("update slot attendance checkout batch", err)
	}
	return nil
}

func (r timetableInstanceStudentRepository) CreateUnplannedPresentIfAbsent(ctx context.Context, instanceID, studentID int64, checkedInAt time.Time) (*scheduleModels.InstanceStudent, error) {
	value, err := r.timetable.CreateUnplannedPresentIfAbsent(ctx, instanceID, studentID, checkedInAt)
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("create unplanned slot attendance", err)
	}
	return legacyInstanceStudent(value), nil
}

func (r timetableInstanceStudentRepository) ReconcileAttendanceInterval(ctx context.Context, instanceID, studentID int64, previousCheckIn time.Time, previousCheckOut *time.Time, updatedCheckIn time.Time, updatedCheckOut *time.Time) (bool, error) {
	updated, err := r.timetable.ReconcileAttendanceInterval(ctx, instanceID, studentID, previousCheckIn, previousCheckOut, updatedCheckIn, updatedCheckOut)
	if err != nil {
		return false, scheduleRepo.WrapDatabaseError("reconcile slot attendance interval", err)
	}
	return updated, nil
}

func (r timetableInstanceStudentRepository) UpdateAttendanceFields(ctx context.Context, id int64, patch scheduleModels.AttendanceFieldPatch) error {
	if !patch.HasChanges() {
		return nil
	}
	input := timetable.AttendanceFieldPatch{Status: patch.Status, Substatus: patch.Substatus,
		SubstatusClear: patch.SubstatusClear, Note: patch.Note, NoteClear: patch.NoteClear}
	if err := r.timetable.UpdateAttendanceFields(ctx, id, input); err != nil {
		return scheduleRepo.WrapDatabaseError("update attendance fields", err)
	}
	return nil
}

func (r timetableInstanceStudentRepository) BulkUpdateStatus(ctx context.Context, instanceID int64, fromStatus, toStatus string, excludedStudentIDs []int64) (int, error) {
	rows, err := r.timetable.BulkUpdateStatus(ctx, instanceID, fromStatus, toStatus, positiveInstanceStudentIDs(excludedStudentIDs))
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("bulk update status", err)
	}
	return rows, nil
}

func (r timetableInstanceStudentRepository) MarkNotScheduled(ctx context.Context, refs []scheduleModels.StudentInstanceRef) error {
	if err := r.timetable.MarkNotScheduled(ctx, publicStudentInstanceRefs(refs)); err != nil {
		return scheduleRepo.WrapDatabaseError("mark attendance rows not scheduled", err)
	}
	return nil
}

func (r timetableInstanceStudentRepository) MarkExpectedAbsentByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, updatedAt time.Time, exclusions []scheduleModels.StudentInstanceRef) error {
	if len(activeGroupIDs) == 0 {
		return nil
	}
	if err := r.timetable.MarkExpectedAbsentByActiveGroupIDs(ctx, positiveInstanceStudentIDs(activeGroupIDs), updatedAt, publicStudentInstanceRefs(exclusions)); err != nil {
		return scheduleRepo.WrapDatabaseError("mark expected absent by active group ids", err)
	}
	return nil
}

func (r timetableInstanceStudentRepository) CloseOpenCheckoutsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, checkedOutAt time.Time) (int, error) {
	if len(activeGroupIDs) == 0 {
		return 0, nil
	}
	rows, err := r.timetable.CloseOpenCheckoutsByActiveGroupIDs(ctx, positiveInstanceStudentIDs(activeGroupIDs), checkedOutAt)
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("close open checkouts by active group ids", err)
	}
	return rows, nil
}

func (r timetableInstanceStudentRepository) ListStudentInstanceRefsBefore(ctx context.Context, cutoff scheduleRepo.InstanceStudentDate) ([]scheduleModels.StudentInstanceRef, error) {
	values, err := r.timetable.ListStudentInstanceRefsBefore(ctx, cutoff.String())
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("list student instance refs before", err)
	}
	result := make([]scheduleModels.StudentInstanceRef, 0, len(values))
	for _, value := range values {
		result = append(result, scheduleModels.StudentInstanceRef(value))
	}
	return result, nil
}

func (r timetableInstanceStudentRepository) ApplyStatusDay(ctx context.Context, studentID int64, date scheduleRepo.InstanceStudentDate, statusDayID int64, substatus string) (int, error) {
	rows, err := r.timetable.ApplyStatusDay(ctx, studentID, date.String(), statusDayID, substatus)
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("apply student status day to slots", err)
	}
	return rows, nil
}

func (r timetableInstanceStudentRepository) ReleaseStatusDay(ctx context.Context, statusDayID int64) (int, error) {
	rows, err := r.timetable.ReleaseStatusDay(ctx, statusDayID)
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("release student status day from slots", err)
	}
	return rows, nil
}

func (r timetableInstanceStudentRepository) ApplyActiveStatusDaysForInstance(ctx context.Context, instanceID int64, date scheduleRepo.InstanceStudentDate) (int, error) {
	rows, err := r.timetable.ApplyActiveStatusDaysForInstance(ctx, instanceID, date.String())
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("apply active status days to instance", err)
	}
	return rows, nil
}

func (r timetableInstanceStudentRepository) ApplyPartialAbsence(ctx context.Context, pickupExceptionID int64) (int, error) {
	rows, err := r.timetable.ApplyPartialAbsence(ctx, pickupExceptionID)
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("apply partial absence to slots", err)
	}
	return rows, nil
}

func (r timetableInstanceStudentRepository) ReleasePartialAbsence(ctx context.Context, pickupExceptionID int64) (int, error) {
	rows, err := r.timetable.ReleasePartialAbsence(ctx, pickupExceptionID)
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("release partial absence from slots", err)
	}
	return rows, nil
}

func (r timetableInstanceStudentRepository) ApplyActivePartialAbsencesForInstance(ctx context.Context, instanceID int64, date scheduleRepo.InstanceStudentDate) (int, error) {
	rows, err := r.timetable.ApplyActivePartialAbsencesForInstance(ctx, instanceID, date.String())
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("apply active partial absences to instance", err)
	}
	return rows, nil
}

func (r timetableInstanceStudentRepository) FindInstancesWithAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, from, to scheduleRepo.InstanceStudentDate) ([]*scheduleModels.ScheduledInstanceRow, error) {
	if studentID <= 0 {
		return []*scheduleModels.ScheduledInstanceRow{}, nil
	}
	values, err := r.timetable.ListScheduledInstancesForStudent(ctx, studentID, from.String(), to.String())
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("find instances with attendance by student and date range", err)
	}
	result := make([]*scheduleModels.ScheduledInstanceRow, 0, len(values))
	for _, value := range values {
		instance, convertErr := legacyActivityInstance(value.Instance)
		if convertErr != nil {
			return nil, scheduleRepo.WrapDatabaseError("find instances with attendance by student and date range", convertErr)
		}
		result = append(result, &scheduleModels.ScheduledInstanceRow{
			Instance: instance, Attendance: legacyInstanceStudent(value.Attendance),
		})
	}
	return result, nil
}

func (r timetableInstanceStudentRepository) HasPlannedSlotsInRange(ctx context.Context, from, to scheduleRepo.InstanceStudentDate) (bool, error) {
	result, err := r.timetable.HasPlannedStudentSlots(ctx, from.String(), to.String())
	if err != nil {
		return false, scheduleRepo.WrapDatabaseError("check planned slots in range", err)
	}
	return result, nil
}

func (r timetableInstanceStudentRepository) FindPlannedStudentIDsByDate(ctx context.Context, studentIDs []int64, date scheduleRepo.InstanceStudentDate) ([]int64, error) {
	studentIDs = positiveInstanceStudentIDs(studentIDs)
	if len(studentIDs) == 0 {
		return []int64{}, nil
	}
	result, err := r.timetable.ListPlannedStudentIDs(ctx, studentIDs, date.String())
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("find planned student ids by date", err)
	}
	return result, nil
}

func (r timetableInstanceStudentRepository) ArchivePlannedByStudentIDsFrom(ctx context.Context, transitionID int64, studentIDs []int64, from scheduleRepo.InstanceStudentDate, at time.Time) (int, error) {
	rows, err := r.timetable.ArchivePlannedInstanceStudents(ctx, transitionID, positiveInstanceStudentIDs(studentIDs), from.String(), at)
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("archive planned by student ids from", err)
	}
	return rows, nil
}

func (r timetableInstanceStudentRepository) RestoreArchivedByTransition(ctx context.Context, transitionID int64, studentIDs []int64, from scheduleRepo.InstanceStudentDate) (int, error) {
	rows, err := r.timetable.RestoreArchivedInstanceStudents(ctx, transitionID, positiveInstanceStudentIDs(studentIDs), from.String())
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("restore archived rows by transition", err)
	}
	return rows, nil
}

func (r timetableInstanceStudentRepository) list(ctx context.Context, filter timetable.InstanceStudentFilter, operation string) ([]*scheduleModels.InstanceStudent, error) {
	requestedIDs, requestedInstances, requestedStudents := len(filter.IDs) > 0, len(filter.InstanceIDs) > 0, len(filter.StudentIDs) > 0
	filter.IDs = positiveInstanceStudentIDs(filter.IDs)
	filter.InstanceIDs = positiveInstanceStudentIDs(filter.InstanceIDs)
	filter.StudentIDs = positiveInstanceStudentIDs(filter.StudentIDs)
	if requestedIDs && len(filter.IDs) == 0 || requestedInstances && len(filter.InstanceIDs) == 0 || requestedStudents && len(filter.StudentIDs) == 0 {
		return []*scheduleModels.InstanceStudent{}, nil
	}
	values, err := r.timetable.ListInstanceStudents(ctx, filter)
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError(operation, err)
	}
	result := make([]*scheduleModels.InstanceStudent, 0, len(values))
	for _, value := range values {
		result = append(result, legacyInstanceStudent(value))
	}
	return result, nil
}

func positiveInstanceStudentIDs(ids []int64) []int64 {
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			result = append(result, id)
		}
	}
	return result
}

func legacyInstanceStudent(value timetable.InstanceStudent) *scheduleModels.InstanceStudent {
	result := &scheduleModels.InstanceStudent{}
	replaceLegacyInstanceStudent(result, value)
	return result
}

func replaceLegacyInstanceStudent(result *scheduleModels.InstanceStudent, value timetable.InstanceStudent) {
	*result = scheduleModels.InstanceStudent{InstanceID: value.InstanceID, StudentID: value.StudentID, RoomID: value.RoomID,
		Status: value.Status, Substatus: value.Substatus, Note: value.Note, CheckedInAt: value.CheckedInAt,
		CheckedOutAt: value.CheckedOutAt, IsUnplanned: value.IsUnplanned, NotScheduled: value.NotScheduled,
		ManualStatusAt: value.ManualStatusAt, StudentStatusDayID: value.StudentStatusDayID, PickupExceptionID: value.PickupExceptionID}
	result.ID, result.CreatedAt, result.UpdatedAt = value.ID, value.CreatedAt, value.UpdatedAt
	result.SetTenantID(value.TenantID)
}

func publicInstanceStudentInput(value *scheduleModels.InstanceStudent) timetable.InstanceStudentInput {
	return timetable.InstanceStudentInput{InstanceID: value.InstanceID, StudentID: value.StudentID, RoomID: value.RoomID,
		Status: value.Status, Substatus: value.Substatus, Note: value.Note, CheckedInAt: value.CheckedInAt,
		CheckedOutAt: value.CheckedOutAt, IsUnplanned: value.IsUnplanned, NotScheduled: value.NotScheduled,
		ManualStatusAt: value.ManualStatusAt, StudentStatusDayID: value.StudentStatusDayID, PickupExceptionID: value.PickupExceptionID}
}

func publicInstanceStudentKeys(keys []scheduleModels.InstanceStudentKey) []timetable.InstanceStudentKey {
	result := make([]timetable.InstanceStudentKey, 0, len(keys))
	for _, key := range keys {
		result = append(result, timetable.InstanceStudentKey(key))
	}
	return result
}

func publicStudentInstanceRefs(refs []scheduleModels.StudentInstanceRef) []timetable.StudentInstanceRef {
	result := make([]timetable.StudentInstanceRef, 0, len(refs))
	for _, ref := range refs {
		if ref.InstanceID > 0 && ref.StudentID > 0 {
			result = append(result, timetable.StudentInstanceRef(ref))
		}
	}
	return result
}

func legacyInstanceStudentError(operation string, err error) error {
	if errors.Is(err, timetable.ErrInstanceStudentNotFound) {
		return scheduleRepo.WrapNotFoundDatabaseError(operation)
	}
	return scheduleRepo.WrapDatabaseError(operation, err)
}
