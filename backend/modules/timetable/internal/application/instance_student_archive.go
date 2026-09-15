package application

import (
	"context"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) ArchivePlannedInstanceStudents(ctx context.Context, transitionID int64, studentIDs []int64, from, currentDate, currentClock string) (rows int, err error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	err = s.runWrite(ctx, "archive_planned_instance_students", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		updated, queryStats, archiveErr := s.store.ArchivePlannedInstanceStudents(txCtx, transitionID, studentIDs, from, currentDate, currentClock)
		stats.Add(queryStats)
		rows = int(updated)
		return archiveErr
	})
	return rows, err
}

func (s *Service) RestoreArchivedInstanceStudents(ctx context.Context, transitionID int64, studentIDs []int64, from string) (rows int, err error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	err = s.runWrite(ctx, "restore_archived_instance_students", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, restoreErr := s.restoreArchivedInstanceStudents(txCtx, transitionID, studentIDs, from, stats)
		rows = int(value)
		return restoreErr
	})
	return rows, err
}

func (s *Service) restoreArchivedInstanceStudents(ctx context.Context, transitionID int64, studentIDs []int64, from string, stats *domain.OperationStats) (int64, error) {
	carePlan, err := s.carePlanDirectory()
	if err != nil {
		return 0, err
	}
	if err = s.lockRestoreCareDays(ctx, transitionID, studentIDs, from, stats); err != nil {
		return 0, err
	}
	exceptions, err := carePlan.ListPickupExceptions(ctx, domain.PickupExceptionFilter{StudentIDs: studentIDs, From: from})
	if err != nil {
		return 0, err
	}
	statusDays, err := carePlan.ListStudentStatusDays(ctx, domain.StudentStatusDayFilter{StudentIDs: studentIDs, From: from, ActiveOnly: true, LatestOnly: true})
	if err != nil {
		return 0, err
	}
	removals, queryStats, err := s.store.ConsumeRosterRemovals(ctx, transitionID, studentIDs)
	stats.Add(queryStats)
	if err != nil || len(removals) == 0 {
		return 0, err
	}
	fields, err := s.restoredInstanceStudentFields(ctx, removals, statusDays, exceptions, from, stats)
	if err != nil {
		return 0, err
	}
	inserted, insertStats, err := s.store.InsertRestoredInstanceStudents(ctx, fields)
	stats.Add(insertStats)
	return inserted, err
}

func (s *Service) lockRestoreCareDays(ctx context.Context, transitionID int64, studentIDs []int64, from string, stats *domain.OperationStats) error {
	days, queryStats, err := s.store.ListRosterRemovalCareDays(ctx, transitionID, studentIDs, from)
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	for _, day := range days {
		if err = s.locks.LockStudentAndExceptionDay(ctx, day.StudentID, day.Date); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) restoredInstanceStudentFields(ctx context.Context, removals []domain.RosterRemoval, statusDays []domain.StudentStatusDay, exceptions []domain.PickupException, from string, stats *domain.OperationStats) ([]domain.InstanceStudentFields, error) {
	rooms, roomStats, err := s.rooms.LockRoomsByID(ctx, rosterRoomIDs(removals))
	stats.Add(roomStats)
	if err != nil {
		return nil, err
	}
	instances, instanceStats, err := s.store.ListActivityInstances(ctx, domain.ActivityInstanceFilter{IDs: rosterInstanceIDs(removals)})
	stats.Add(instanceStats)
	if err != nil {
		return nil, err
	}
	return buildRestoredInstanceStudents(removals, instances, rooms, statusDays, exceptions, from), nil
}

type studentDay struct {
	studentID int64
	date      string
}

func buildRestoredInstanceStudents(removals []domain.RosterRemoval, instances []domain.ActivityInstance, rooms []domain.RoomRef, statusDays []domain.StudentStatusDay, exceptions []domain.PickupException, from string) []domain.InstanceStudentFields {
	instanceByID := make(map[int64]domain.ActivityInstance, len(instances))
	for _, instance := range instances {
		instanceByID[instance.ID] = instance
	}
	validRooms := validRosterRooms(rooms)
	statuses := statusDaysByStudentDate(statusDays)
	pickups := pickupExceptionsByStudentDate(exceptions)
	result := make([]domain.InstanceStudentFields, 0, len(removals))
	for _, removal := range removals {
		instance, ok := instanceByID[removal.InstanceID]
		if !ok || instance.Date < from || instance.Status == "completed" || instance.Status == "cancelled" {
			continue
		}
		result = append(result, restoredInstanceStudent(removal, instance, validRooms, statuses, pickups))
	}
	return result
}

func restoredInstanceStudent(removal domain.RosterRemoval, instance domain.ActivityInstance, validRooms map[int64]int64, statuses map[studentDay]domain.StudentStatusDay, pickups map[studentDay][]domain.PickupException) domain.InstanceStudentFields {
	fields := domain.InstanceStudentFields{InstanceID: removal.InstanceID, StudentID: removal.StudentID,
		Status: removal.Status, Substatus: removal.Substatus, Note: removal.Note, IsUnplanned: removal.IsUnplanned,
		NotScheduled: removal.NotScheduled, ManualStatusAt: removal.ManualStatusAt}
	if removal.RoomID != nil && validRooms[*removal.RoomID] == removal.TenantID {
		fields.RoomID = removal.RoomID
	}
	if removal.NotScheduled || removal.ManualStatusAt != nil && removal.Status != domain.AttendanceExpected {
		return fields
	}
	key := studentDay{studentID: removal.StudentID, date: instance.Date}
	if status, ok := statuses[key]; ok {
		fields.Status = domain.AttendanceAbsent
		fields.Substatus = attendanceSubstatus(status.Status)
		fields.StudentStatusDayID = &status.ID
		return fields
	}
	if pickup := activePickupException(pickups[key], instance.StartTime); pickup != nil {
		fields.Status = domain.AttendanceAbsent
		substatus := domain.AttendanceSubstatusExcused
		fields.Substatus = &substatus
		fields.PickupExceptionID = &pickup.ID
		return fields
	}
	fields.Status, fields.Substatus = domain.AttendanceExpected, nil
	return fields
}

func attendanceSubstatus(status string) *string {
	var value string
	switch status {
	case "sick":
		value = domain.AttendanceSubstatusSick
	case "excused":
		value = domain.AttendanceSubstatusExcused
	case "class_trip":
		value = domain.AttendanceSubstatusTrip
	default:
		return nil
	}
	return &value
}

func activePickupException(values []domain.PickupException, startTime string) *domain.PickupException {
	for i := range values {
		if values[i].ExcusedFrom != nil && startTime >= values[i].ExcusedFrom.Format("15:04:05") {
			return &values[i]
		}
	}
	return nil
}

func validRosterRooms(values []domain.RoomRef) map[int64]int64 {
	result := make(map[int64]int64, len(values))
	for _, value := range values {
		result[value.ID] = value.TenantID
	}
	return result
}

func statusDaysByStudentDate(values []domain.StudentStatusDay) map[studentDay]domain.StudentStatusDay {
	result := make(map[studentDay]domain.StudentStatusDay, len(values))
	for _, value := range values {
		result[studentDay{studentID: value.StudentID, date: value.Date}] = value
	}
	return result
}

func pickupExceptionsByStudentDate(values []domain.PickupException) map[studentDay][]domain.PickupException {
	result := make(map[studentDay][]domain.PickupException)
	for _, value := range values {
		key := studentDay{studentID: value.StudentID, date: value.ExceptionDate}
		result[key] = append(result[key], value)
	}
	return result
}

func rosterRoomIDs(values []domain.RosterRemoval) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value.RoomID != nil {
			result = append(result, *value.RoomID)
		}
	}
	return sortedUniqueIDs(result)
}

func rosterInstanceIDs(values []domain.RosterRemoval) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		result = append(result, value.InstanceID)
	}
	return sortedUniqueIDs(result)
}

func sortedUniqueIDs(values []int64) []int64 {
	slices.Sort(values)
	return slices.Compact(values)
}
