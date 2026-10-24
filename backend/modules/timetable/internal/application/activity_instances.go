package application

import (
	"context"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) FindActivityInstance(ctx context.Context, id int64) (result timetable.ActivityInstance, err error) {
	if id <= 0 {
		return timetable.ActivityInstance{}, s.reject("find_activity_instance", timetable.ErrInvalidActivityInstanceQuery)
	}

	err = s.run(ctx, "find_activity_instance", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindActivityInstance(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrActivityInstanceNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListActivityInstances(ctx context.Context, filter timetable.ActivityInstanceFilter) (result []timetable.ActivityInstance, err error) {
	if !validActivityInstanceFilter(filter) {
		return nil, s.reject("list_activity_instances", timetable.ErrInvalidActivityInstanceQuery)
	}

	err = s.run(ctx, "list_activity_instances", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListActivityInstances(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) MaxActivityInstanceID(ctx context.Context) (result int64, err error) {
	err = s.run(ctx, "max_activity_instance_id", func(ctx context.Context, stats *domain.OperationStats) error {
		value, queryStats, findErr := s.store.MaxActivityInstanceID(ctx)
		stats.Add(queryStats)
		result = value
		return findErr
	})
	return result, err
}

func (s *Service) CountActivityInstances(ctx context.Context, before *string) (result int, err error) {
	if !validOptionalDate(before) {
		return 0, s.reject("count_activity_instances", timetable.ErrInvalidActivityInstanceQuery)
	}

	err = s.run(ctx, "count_activity_instances", func(ctx context.Context, stats *domain.OperationStats) error {
		count, queryStats, countErr := s.store.CountActivityInstances(ctx, before)
		stats.Add(queryStats)
		result = count
		return countErr
	})
	return result, err
}

func (s *Service) OldestActivityInstanceBefore(ctx context.Context, before *string) (result *string, err error) {
	if !validOptionalDate(before) {
		return nil, s.reject("oldest_activity_instance", timetable.ErrInvalidActivityInstanceQuery)
	}

	err = s.run(ctx, "oldest_activity_instance", func(ctx context.Context, stats *domain.OperationStats) error {
		value, queryStats, findErr := s.store.OldestActivityInstanceBefore(ctx, before)
		stats.Add(queryStats)
		result = value
		return findErr
	})
	return result, err
}

func (s *Service) CreateActivityInstance(ctx context.Context, fields timetable.ActivityInstanceInput) (result timetable.ActivityInstance, err error) {
	fields, ok := normalizeActivityInstance(fields)
	if !ok {
		return timetable.ActivityInstance{}, s.reject("create_activity_instance", timetable.ErrInvalidActivityInstance)
	}

	err = s.runWrite(ctx, "create_activity_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateActivityInstance(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) CreateTemplateBackedActivityInstanceIfAbsent(ctx context.Context, fields timetable.ActivityInstanceInput) (result timetable.ActivityInstance, inserted bool, err error) {
	fields, ok := normalizeActivityInstance(fields)
	if !ok || fields.ActivityGroupID == nil {
		return timetable.ActivityInstance{}, false, s.reject("create_template_backed_activity_instance", timetable.ErrInvalidActivityInstance)
	}

	err = s.runWrite(ctx, "create_template_backed_activity_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, created, queryStats, createErr := s.store.CreateTemplateBackedActivityInstanceIfAbsent(txCtx, fields)
		stats.Add(queryStats)
		result, inserted = value, created
		return createErr
	})
	return result, inserted, err
}

func (s *Service) CreateIdempotentActivityInstance(ctx context.Context, fields timetable.ActivityInstanceInput) (result timetable.ActivityInstance, inserted bool, err error) {
	fields, ok := normalizeActivityInstance(fields)
	if !ok || fields.IdempotencyKey == nil {
		return timetable.ActivityInstance{}, false, s.reject("create_idempotent_activity_instance", timetable.ErrInvalidActivityInstance)
	}

	err = s.runWrite(ctx, "create_idempotent_activity_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, created, queryStats, createErr := s.store.CreateIdempotentActivityInstance(txCtx, fields)
		stats.Add(queryStats)
		result, inserted = value, created
		return createErr
	})
	return result, inserted, err
}

func (s *Service) UpdateActivityInstance(ctx context.Context, id int64, fields timetable.ActivityInstanceInput) (result timetable.ActivityInstance, err error) {
	fields, ok := normalizeActivityInstance(fields)
	if id <= 0 || !ok {
		return timetable.ActivityInstance{}, s.reject("update_activity_instance", timetable.ErrInvalidActivityInstance)
	}

	err = s.runWrite(ctx, "update_activity_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.lockOperationalInstanceStaff(txCtx, id, fields.Date, fields.Status, nil, stats); err != nil {
			return err
		}
		value, found, queryStats, updateErr := s.store.UpdateActivityInstance(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return timetable.ErrActivityInstanceNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) PatchActivityInstance(ctx context.Context, id int64, fields timetable.ActivityInstanceInput, columns []string) (result int64, err error) {
	if id <= 0 || len(columns) == 0 || !validActivityInstanceColumns(columns) {
		return 0, s.reject("patch_activity_instance", timetable.ErrInvalidActivityInstance)
	}

	err = s.runWrite(ctx, "patch_activity_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.guardActivityInstancePatch(txCtx, id, fields, columns, stats); err != nil {
			return err
		}
		rows, queryStats, updateErr := s.store.PatchActivityInstance(txCtx, id, fields, columns)
		stats.Add(queryStats)
		result = rows
		return updateErr
	})
	return result, err
}

// guardActivityInstancePatch locks what a date or planning-status change
// touches: the staff assignments of an operational block, and the block
// itself. A cancelled block has no session, so a planning-status change of a
// running block has to end its execution first.
func (s *Service) guardActivityInstancePatch(ctx context.Context, id int64, fields timetable.ActivityInstanceInput, columns []string, stats *domain.OperationStats) error {
	if !slices.Contains(columns, "date") && !slices.Contains(columns, "status") {
		return nil
	}
	current, found, queryStats, err := s.store.FindActivityInstance(ctx, id)
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	if !found {
		return timetable.ErrActivityInstanceNotFound
	}
	date, status := current.Date, current.Status
	if slices.Contains(columns, "date") {
		date = fields.Date
	}
	if slices.Contains(columns, "status") {
		status = fields.Status
		if err := s.requireUnstartedInstance(ctx, id); err != nil {
			return err
		}
	}
	return s.lockOperationalInstanceStaff(ctx, id, date, status, &current, stats)
}

func (s *Service) requireUnstartedInstance(ctx context.Context, id int64) error {
	started, err := s.startedInstanceIDs(ctx, []int64{id})
	if err != nil {
		return err
	}
	if started[id] {
		return domain.ErrActivityInstanceStarted
	}
	return nil
}

// Moving retained history back into the operational timetable must not revive
// assignments for staff who have since retired.
func (s *Service) lockOperationalInstanceStaff(ctx context.Context, id int64, date, status string, expected *timetable.ActivityInstance, stats *domain.OperationStats) error {
	operational := date > s.today() || (date == s.today() && status == "planned")
	assignments, queryStats, err := s.store.ListInstanceStaff(ctx, timetable.InstanceStaffFilter{InstanceIDs: []int64{id}})
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	ids := sortedInstanceStaffIDs(assignments)
	if operational {
		if err := s.lockInstanceStaffAssignments(ctx, ids); err != nil {
			return err
		}
	}
	current, found, queryStats, err := s.store.LockActivityInstance(ctx, id, true)
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	if !found {
		return timetable.ErrActivityInstanceNotFound
	}
	if expected != nil && (current.Date != expected.Date || current.Status != expected.Status) {
		return timetable.ErrOffboardingConflict
	}
	if !operational {
		return nil
	}
	return s.requireInstanceStaffLocked(ctx, id, ids, stats)
}

// sortedInstanceStaffIDs returns the distinct staff IDs of the assignments in
// ascending order: the order offboarding takes staff locks in.
func sortedInstanceStaffIDs(assignments []timetable.InstanceStaff) []int64 {
	ids := make([]int64, 0, len(assignments))
	for _, assignment := range assignments {
		ids = append(ids, assignment.StaffID)
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

func (s *Service) lockInstanceStaffAssignments(ctx context.Context, staffIDs []int64) error {
	for _, staffID := range staffIDs {
		if err := s.lockStaffAssignment(ctx, staffID); err != nil {
			return err
		}
	}
	return nil
}

// requireInstanceStaffLocked re-reads the instance staff below the instance
// lock. Do not acquire newly discovered staff locks there: that would invert
// offboarding's order. Abort and let the caller retry.
func (s *Service) requireInstanceStaffLocked(ctx context.Context, id int64, lockedIDs []int64, stats *domain.OperationStats) error {
	assignments, queryStats, err := s.store.ListInstanceStaff(ctx, timetable.InstanceStaffFilter{InstanceIDs: []int64{id}})
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	for _, assignment := range assignments {
		if _, locked := slices.BinarySearch(lockedIDs, assignment.StaffID); !locked {
			return timetable.ErrOffboardingConflict
		}
	}
	return nil
}

func (s *Service) DeleteActivityInstance(ctx context.Context, id int64) error {
	if id <= 0 {
		return s.reject("delete_activity_instance", timetable.ErrInvalidActivityInstance)
	}

	return s.runWrite(ctx, "delete_activity_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteActivityInstance(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeleteActivityInstancesBefore(ctx context.Context, before string) (result int64, err error) {
	if !timetable.ValidDate(before) {
		return 0, s.reject("delete_activity_instances_before", timetable.ErrInvalidActivityInstance)
	}

	err = s.runWrite(ctx, "delete_activity_instances_before", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, deleteErr := s.store.DeleteActivityInstancesBefore(txCtx, before)
		stats.Add(queryStats)
		result = rows
		return deleteErr
	})
	return result, err
}

// DeletePlannedActivityInstances removes the template-backed occurrences of
// the window a replan may replace. A block that started or ended keeps its
// row: Student Presence records the execution, so the planner asks it first.
func (s *Service) DeletePlannedActivityInstances(ctx context.Context, from string, to *string, groupID *int64, preserveDeviations bool) (result int64, err error) {
	if !timetable.ValidDate(from) || !validOptionalDate(to) || (groupID != nil && *groupID <= 0) {
		return 0, s.reject("delete_planned_activity_instances", timetable.ErrInvalidActivityInstance)
	}

	err = s.runWrite(ctx, "delete_planned_activity_instances", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		ids, queryStats, listErr := s.store.ListReplannableActivityInstanceIDs(txCtx, from, to, groupID, preserveDeviations)
		stats.Add(queryStats)
		if listErr != nil {
			return listErr
		}
		return s.deleteUnstartedInstances(txCtx, ids, stats, &result)
	})
	return result, err
}

func (s *Service) deleteUnstartedInstances(ctx context.Context, ids []int64, stats *domain.OperationStats, result *int64) error {
	started, err := s.startedInstanceIDs(ctx, ids)
	if err != nil {
		return err
	}
	rows, deleteStats, err := s.store.DeleteActivityInstancesByID(ctx, withoutIDs(ids, started))
	stats.Add(deleteStats)
	*result = rows
	return err
}

func (s *Service) DeleteRemovedWeekendActivityInstances(ctx context.Context, groupID int64, weekdays []int) (result int64, err error) {
	if groupID <= 0 || invalidActivityInstanceWeekdays(weekdays) {
		return 0, s.reject("delete_removed_weekend_activity_instances", timetable.ErrInvalidActivityInstance)
	}

	err = s.runWrite(ctx, "delete_removed_weekend_activity_instances", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		ids, queryStats, listErr := s.store.ListRemovedWeekendActivityInstanceIDs(txCtx, groupID, weekdays, s.today())
		stats.Add(queryStats)
		if listErr != nil {
			return listErr
		}
		return s.deleteUnstartedInstances(txCtx, ids, stats, &result)
	})
	return result, err
}

func (s *Service) PropagateActivityInstanceListKind(ctx context.Context, groupID int64, previousKind, newKind *string, after string) (result int64, err error) {
	if groupID <= 0 || !timetable.ValidDate(after) {
		return 0, s.reject("propagate_activity_instance_list_kind", timetable.ErrInvalidActivityInstance)
	}

	err = s.runWrite(ctx, "propagate_activity_instance_list_kind", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		ids, queryStats, listErr := s.store.ListFutureTemplateActivityInstanceIDs(txCtx, groupID, previousKind, after)
		stats.Add(queryStats)
		if listErr != nil {
			return listErr
		}
		started, factsErr := s.startedInstanceIDs(txCtx, ids)
		if factsErr != nil {
			return factsErr
		}
		rows, updateStats, updateErr := s.store.SetActivityInstanceListKind(txCtx, withoutIDs(ids, started), newKind, time.Now())
		stats.Add(updateStats)
		result = rows
		return updateErr
	})
	return result, err
}
