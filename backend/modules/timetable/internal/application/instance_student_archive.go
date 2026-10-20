package application

import (
	"context"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

// ArchivePlannedInstanceStudents parks the given participants of a grade
// transition together with the attendance snapshots the caller read from
// Student Presence. Which participants may go is the caller's decision; a
// participant with observed presence is refused here because a transition
// must never erase a recorded day.
func (s *Service) ArchivePlannedInstanceStudents(ctx context.Context, transitionID int64, entries []timetable.RosterArchiveEntry) (rows int, err error) {
	if transitionID <= 0 || !validRosterArchiveEntries(entries) {
		return 0, s.reject("archive_planned_instance_students", timetable.ErrInvalidInstanceStudent)
	}

	if len(entries) == 0 {
		return 0, nil
	}
	err = s.runWrite(ctx, "archive_planned_instance_students", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		ids := make([]int64, 0, len(entries))
		for _, entry := range entries {
			ids = append(ids, entry.ParticipantID)
		}
		observed, err := s.observedParticipantIDs(txCtx, ids)
		if err != nil {
			return err
		}
		eligible := make([]timetable.RosterArchiveEntry, 0, len(entries))
		for _, entry := range entries {
			if !observed[entry.ParticipantID] {
				eligible = append(eligible, entry)
			}
		}
		updated, queryStats, archiveErr := s.store.ArchivePlannedInstanceStudents(txCtx, transitionID, eligible)
		stats.Add(queryStats)
		rows = int(updated)
		return archiveErr
	})
	return rows, err
}

// RestoreArchivedInstanceStudents puts the archived participants back onto
// the rosters from the date on. Instances that ended, were cancelled, or
// are dated before the restore stay untouched, as does a roster that lists
// the child again. The archived attendance travels back with each restored
// participant for Student Presence to apply.
func (s *Service) RestoreArchivedInstanceStudents(ctx context.Context, transitionID int64, studentIDs []int64, from string) (result []timetable.RestoredInstanceStudent, err error) {
	if transitionID <= 0 || hasInvalidID(studentIDs) || !timetable.ValidDate(from) {
		return nil, s.reject("restore_archived_instance_students", timetable.ErrInvalidInstanceStudent)
	}

	result = []timetable.RestoredInstanceStudent{}
	if len(studentIDs) == 0 {
		return result, nil
	}
	err = s.runWrite(ctx, "restore_archived_instance_students", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		removals, queryStats, consumeErr := s.store.ConsumeRosterRemovals(txCtx, transitionID, studentIDs)
		stats.Add(queryStats)
		if consumeErr != nil || len(removals) == 0 {
			return consumeErr
		}
		rooms, roomStats, roomErr := s.rooms.LockRoomsByID(txCtx, rosterRoomIDs(removals))
		stats.Add(roomStats)
		if roomErr != nil {
			return roomErr
		}
		instances, instanceStats, listErr := s.store.ListActivityInstances(txCtx, timetable.ActivityInstanceFilter{IDs: rosterInstanceIDs(removals)})
		stats.Add(instanceStats)
		if listErr != nil {
			return listErr
		}
		completed, factsErr := s.completedInstanceIDs(txCtx, rosterInstanceIDs(removals))
		if factsErr != nil {
			return factsErr
		}
		fields, snapshots := restorableInstanceStudents(removals, instances, validRosterRooms(rooms), completed, from)
		inserted, insertStats, insertErr := s.store.InsertRestoredInstanceStudents(txCtx, fields)
		stats.Add(insertStats)
		if insertErr != nil {
			return insertErr
		}
		result = restoredInstanceStudents(inserted, instances, snapshots)
		return nil
	})
	return result, err
}

// restoredInstanceStudents pairs every restored row with its block and the
// attendance snapshot the archive kept for it.
func restoredInstanceStudents(inserted []timetable.InstanceStudent, instances []timetable.ActivityInstance, snapshots map[timetable.InstanceStudentKey]timetable.ArchivedAttendance) []timetable.RestoredInstanceStudent {
	instanceByID := make(map[int64]timetable.ActivityInstance, len(instances))
	for _, instance := range instances {
		instanceByID[instance.ID] = instance
	}
	result := make([]timetable.RestoredInstanceStudent, 0, len(inserted))
	for _, participant := range inserted {
		instance := instanceByID[participant.InstanceID]
		result = append(result, timetable.RestoredInstanceStudent{
			ID: participant.ID, InstanceID: participant.InstanceID, StudentID: participant.StudentID, RoomID: participant.RoomID,
			Date: instance.Date, StartTime: instance.StartTime,
			Attendance: snapshots[timetable.InstanceStudentKey{InstanceID: participant.InstanceID, StudentID: participant.StudentID}],
		})
	}
	return result
}

func restorableInstanceStudents(removals []domain.RosterRemoval, instances []timetable.ActivityInstance, validRooms map[int64]int64, completed map[int64]bool, from string) ([]timetable.InstanceStudentInput, map[timetable.InstanceStudentKey]timetable.ArchivedAttendance) {
	instanceByID := make(map[int64]timetable.ActivityInstance, len(instances))
	for _, instance := range instances {
		instanceByID[instance.ID] = instance
	}
	fields := make([]timetable.InstanceStudentInput, 0, len(removals))
	snapshots := make(map[timetable.InstanceStudentKey]timetable.ArchivedAttendance, len(removals))
	for _, removal := range removals {
		instance, ok := instanceByID[removal.InstanceID]
		if !ok || instance.Date < from || instance.Status == domain.InstanceStatusCancelled || completed[removal.InstanceID] {
			continue
		}
		field := timetable.InstanceStudentInput{InstanceID: removal.InstanceID, StudentID: removal.StudentID}
		if removal.RoomID != nil && validRooms[*removal.RoomID] == removal.TenantID {
			field.RoomID = removal.RoomID
		}
		fields = append(fields, field)
		snapshots[timetable.InstanceStudentKey{InstanceID: removal.InstanceID, StudentID: removal.StudentID}] = timetable.ArchivedAttendance(removal.Attendance)
	}
	return fields, snapshots
}

func validRosterRooms(values []timetable.RoomRef) map[int64]int64 {
	result := make(map[int64]int64, len(values))
	for _, value := range values {
		result[value.ID] = value.TenantID
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
