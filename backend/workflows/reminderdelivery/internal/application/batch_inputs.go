package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

func (s *service) loadBatchPresence(ctx context.Context, cfg batchConfig, recipients []reminder.BatchScope, caregivers bool, in *batchInputs) error {
	needsAttendance := cfg.anyPickup() && (!caregivers || cfg.binaryPresence || hasAdmin(recipients))
	if needsAttendance {
		ids, err := s.presentStudentIDsFromAttendance(ctx, in.today)
		if err != nil {
			return err
		}
		in.attendanceIDs = ids
	}

	if caregivers && cfg.anyPickup() && !cfg.binaryPresence {
		byRoom, err := s.Visits.ListOpenVisitStudentIDsByRoom(ctx)
		if err != nil {
			return fmt.Errorf("load open visits by room: %w", err)
		}
		in.visitsByRoom = byRoom
	}

	return nil
}

func (s *service) loadBatchSupervision(ctx context.Context, cfg batchConfig, caregivers bool, in *batchInputs) error {
	needsSupervision := cfg.anyActivity() || (cfg.anyPickup() && !cfg.binaryPresence)
	if !caregivers || !needsSupervision || s.BulkSupervision == nil {
		return nil
	}
	rows, err := s.BulkSupervision.ListActiveSupervisedRooms(ctx)
	if err != nil {
		return fmt.Errorf("load supervised rooms: %w", err)
	}
	for _, row := range rows {
		if _, wanted := in.roomsByStaff[row.StaffID]; wanted {
			in.roomsByStaff[row.StaffID] = append(in.roomsByStaff[row.StaffID], row.RoomID)
		}
	}
	return nil
}

func (s *service) loadBatchPickups(ctx context.Context, cfg batchConfig, recipients []reminder.BatchScope, in *batchInputs) error {
	// Presence per person, reproducing pickupScopeStudentIDs branch for branch.
	for _, sc := range recipients {
		in.presenceByStaff[sc.StaffID] = s.batchPresence(sc, cfg, in)
	}

	union := unionOf(in.presenceByStaff)
	if len(union) > 0 {
		if s.Student != nil {
			students, err := s.Student.FindReadScopeByIDs(ctx, union)
			if err != nil {
				return fmt.Errorf("load students: %w", err)
			}
			in.students = students
		}
		if s.Pickup != nil {
			times, err := s.Pickup.GetBulkEffectivePickupTimesForDate(ctx, union, in.today)
			if err != nil {
				return err
			}
			in.pickupTimes = times
		}
	}
	return nil
}

func (s *service) loadBatchActivities(ctx context.Context, cfg batchConfig, recipients []reminder.BatchScope, in *batchInputs) error {
	instances, err := s.Instance.FindByTenantAndDate(ctx, in.today)
	if err != nil {
		return err
	}
	in.instances = instances
	if cfg.runningOver {
		active, activeErr := s.Instance.FindActiveByTenant(ctx)
		if activeErr != nil {
			return activeErr
		}
		in.runningInstances = active
	}
	if err := s.loadBatchActivityRooms(ctx, cfg, in); err != nil {
		return err
	}
	if err := s.loadBatchRunningOver(ctx, cfg, recipients, in); err != nil {
		return err
	}
	return s.loadBatchAssignments(ctx, recipients, in)
}

func (s *service) loadBatchActivityRooms(ctx context.Context, cfg batchConfig, in *batchInputs) error {
	if !cfg.activityOverdue {
		return nil
	}
	if s.Room == nil {
		return errActivityRoomReaderMissing
	}
	wanted := plannedInstanceRoomIDs(in.instances)
	rooms, roomErr := s.Room.FindByIDs(ctx, sortedIDs(wanted))
	if roomErr != nil {
		return fmt.Errorf("load activity reminder rooms: %w", roomErr)
	}
	if resolveErr := resolveActivityRooms(rooms, wanted); resolveErr != nil {
		return resolveErr
	}
	return nil
}

func (s *service) loadBatchAssignments(ctx context.Context, recipients []reminder.BatchScope, in *batchInputs) error {
	if s.BulkInstanceStaff == nil {
		return nil
	}
	recipientStaffIDs := make(map[int64]struct{}, len(recipients))
	for _, sc := range recipients {
		if sc.StaffID > 0 {
			recipientStaffIDs[sc.StaffID] = struct{}{}
		}
	}

	// Planned assignments, one query for the whole day. Absent rows are
	// skipped: someone who called in sick must not be pinged about the
	// slot they were relieved of. Substitutes are kept — they are the ones
	// who have to show up.
	instanceIDs := plannedBatchInstanceIDs(in.instances)
	rows, staffErr := s.BulkInstanceStaff.FindByInstanceIDs(ctx, instanceIDs)
	if staffErr != nil {
		return fmt.Errorf("load planned activity staff: %w", staffErr)
	}
	for _, row := range rows {
		if row == nil || row.IsAbsent {
			continue
		}
		if _, wanted := recipientStaffIDs[row.StaffID]; !wanted {
			continue
		}
		if in.assignedInstances[row.StaffID] == nil {
			in.assignedInstances[row.StaffID] = map[int64]struct{}{}
		}
		in.assignedInstances[row.StaffID][row.InstanceID] = struct{}{}
	}
	return nil
}

func plannedBatchInstanceIDs(instances []*ports.ActivityInstance) []int64 {
	instanceIDs := make([]int64, 0, len(instances))
	for _, inst := range instances {
		if inst != nil && inst.Status == ports.InstanceStatusPlanned {
			instanceIDs = append(instanceIDs, inst.ID)
		}
	}
	return instanceIDs
}

// loadBatchRunningOver reads facts only for rooms at least one recipient can
// see. An admin sees every room; otherwise the room union preserves one set of
// shared reads without allowing an unrelated room to fail the whole batch.
func (s *service) loadBatchRunningOver(ctx context.Context, cfg batchConfig, recipients []reminder.BatchScope, in *batchInputs) error {
	if !cfg.runningOver {
		return nil
	}
	roomFilter := batchRunningOverRoomFilter(recipients, in.roomsByStaff)
	due, _ := dueRunningOver(in.runningInstances, activityScopeFilter(roomFilter, nil), runningOverWindow{nowMin: in.nowMin, grace: cfg.runningOverGrace, today: calendar.Date(in.today)})
	facts, err := s.loadRunningOverFacts(ctx, in.today, due)
	if err != nil {
		return err
	}
	in.runningOver = facts
	return nil
}

func batchRunningOverRoomFilter(recipients []reminder.BatchScope, roomsByStaff map[int64][]int64) map[int64]struct{} {
	if hasAdmin(recipients) {
		return nil
	}
	rooms := make(map[int64]struct{})
	for _, sc := range recipients {
		for _, roomID := range roomsByStaff[sc.StaffID] {
			rooms[roomID] = struct{}{}
		}
	}
	return rooms
}
