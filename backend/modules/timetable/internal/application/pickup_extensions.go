package application

import (
	"context"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

const expectedAttendanceStatus = "expected"

// RecordPickupDayExtension stores the later day pickup. Past days never open
// a task; the call also prunes past day tasks of the school, so stale rows
// cannot pile up.
func (s *Service) RecordPickupDayExtension(ctx context.Context, task domain.PickupExtensionTask) error {
	return s.runWrite(ctx, "record_pickup_day_extension", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		today := s.today()
		pruneStats, err := s.store.DeletePastPickupExtensionTasks(txCtx, today)
		stats.Add(pruneStats)
		if err != nil {
			return err
		}
		if task.Date < today {
			return nil
		}
		upsertStats, err := s.store.UpsertPickupExtensionTask(txCtx, task)
		stats.Add(upsertStats)
		return err
	})
}

func (s *Service) ClearPickupDayExtension(ctx context.Context, studentID int64, date string) error {
	return s.runWrite(ctx, "clear_pickup_day_extension", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeletePickupDayExtensionTask(txCtx, studentID, date)
		stats.Add(queryStats)
		return err
	})
}

// RecordPickupWeekdayExtension stores a lasting weekday change. Several
// changes in a row keep the earliest previous time: 14:45 → 16:00 → 15:30
// still needs a block from 14:45 on. A change back to that time or earlier
// closes the task.
func (s *Service) RecordPickupWeekdayExtension(ctx context.Context, task domain.PickupExtensionTask) error {
	return s.runWrite(ctx, "record_pickup_weekday_extension", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		tasks, listStats, err := s.store.ListPickupExtensionTasks(txCtx, task.StudentID, s.today())
		stats.Add(listStats)
		if err != nil {
			return err
		}
		for _, existing := range tasks {
			if !existing.IsDay() && existing.Weekday == task.Weekday && existing.PreviousPickup < task.PreviousPickup {
				task.PreviousPickup = existing.PreviousPickup
				task.EffectiveFrom = min(existing.EffectiveFrom, task.EffectiveFrom)
			}
		}
		if task.Pickup <= task.PreviousPickup {
			deleteStats, deleteErr := s.store.DeletePickupWeekdayExtensionTask(txCtx, task.StudentID, task.Weekday)
			stats.Add(deleteStats)
			return deleteErr
		}
		upsertStats, err := s.store.UpsertPickupExtensionTask(txCtx, task)
		stats.Add(upsertStats)
		return err
	})
}

func (s *Service) ClearPickupWeekdayExtension(ctx context.Context, studentID int64, weekday int) error {
	return s.runWrite(ctx, "clear_pickup_weekday_extension", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeletePickupWeekdayExtensionTask(txCtx, studentID, weekday)
		stats.Add(queryStats)
		return err
	})
}

// OpenPickupExtension pairs a task with the blocks that can still be chosen.
type OpenPickupExtension struct {
	Task   domain.PickupExtensionTask
	Blocks []domain.PickupExtensionBlock
}

// ListOpenPickupExtensions returns tasks that still have a choice. Openness
// is derived from the current plan, so a child added by hand, a cancelled
// block or a past date simply drops the task from the list.
func (s *Service) ListOpenPickupExtensions(ctx context.Context, studentID int64) (result []OpenPickupExtension, err error) {
	err = s.run("list_open_pickup_extensions", func(stats *domain.OperationStats) error {
		tasks, queryStats, listErr := s.store.ListPickupExtensionTasks(ctx, studentID, s.today())
		stats.Add(queryStats)
		if listErr != nil {
			return listErr
		}
		blocks, blockErr := s.pickupExtensionBlocks(ctx, tasks, stats)
		if blockErr != nil {
			return blockErr
		}
		result = make([]OpenPickupExtension, 0, len(tasks))
		for _, task := range tasks {
			open := domain.OpenPickupExtensionBlocks(blocks[task.ID], task.Pickup)
			if len(open) > 0 {
				result = append(result, OpenPickupExtension{Task: task, Blocks: open})
			}
		}
		stats.Rows = int64(len(result))
		return nil
	})
	return result, err
}

// PickupExtensionResolution is what ResolvePickupExtension changed.
type PickupExtensionResolution struct {
	Task        domain.PickupExtensionTask
	Assigned    []domain.PickupExtensionBlock
	InstanceIDs []int64
}

// ResolvePickupExtension adds the child to the chosen blocks and removes the
// task. Every chosen block must still be a choice; otherwise nothing changes.
func (s *Service) ResolvePickupExtension(ctx context.Context, taskID int64, blockIDs []int64) (result PickupExtensionResolution, err error) {
	err = s.runWrite(ctx, "resolve_pickup_extension", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		task, found, findStats, findErr := s.store.FindPickupExtensionTaskForUpdate(txCtx, taskID)
		stats.Add(findStats)
		if findErr != nil {
			return findErr
		}
		if !found || (task.IsDay() && task.Date < s.today()) {
			return domain.ErrPickupExtensionNotFound
		}
		result.Task = task
		chosen, chooseErr := s.chosenPickupExtensionBlocks(txCtx, task, blockIDs, stats)
		if chooseErr != nil {
			return chooseErr
		}
		for _, block := range chosen {
			instanceIDs, assignErr := s.assignPickupExtensionBlock(txCtx, task, block, stats)
			if assignErr != nil {
				return assignErr
			}
			result.InstanceIDs = append(result.InstanceIDs, instanceIDs...)
		}
		result.Assigned = chosen
		deleteStats, deleteErr := s.store.DeletePickupExtensionTask(txCtx, task.ID)
		stats.Add(deleteStats)
		return deleteErr
	})
	return result, err
}

func (s *Service) chosenPickupExtensionBlocks(ctx context.Context, task domain.PickupExtensionTask, blockIDs []int64, stats *domain.OperationStats) ([]domain.PickupExtensionBlock, error) {
	if len(blockIDs) == 0 {
		return nil, nil
	}
	blocks, err := s.pickupExtensionBlocks(ctx, []domain.PickupExtensionTask{task}, stats)
	if err != nil {
		return nil, err
	}
	open := domain.OpenPickupExtensionBlocks(blocks[task.ID], task.Pickup)
	chosen := make([]domain.PickupExtensionBlock, 0, len(blockIDs))
	for _, id := range uniquePositiveIDs(blockIDs) {
		index := slices.IndexFunc(open, func(block domain.PickupExtensionBlock) bool { return block.ID == id })
		if index < 0 {
			return nil, domain.ErrPickupExtensionBlockGone
		}
		chosen = append(chosen, open[index])
	}
	return chosen, nil
}

func (s *Service) assignPickupExtensionBlock(ctx context.Context, task domain.PickupExtensionTask, block domain.PickupExtensionBlock, stats *domain.OperationStats) ([]int64, error) {
	if task.IsDay() {
		if err := s.addPlannedStudent(ctx, block.ID, task.StudentID, task.Date, stats); err != nil {
			return nil, err
		}
		return []int64{block.ID}, nil
	}
	validFrom := max(task.EffectiveFrom, s.today(), block.ValidFrom)
	weekday := task.Weekday
	if err := s.ensurePickupWeekdayEnrollment(ctx, task.StudentID, block, weekday, validFrom, stats); err != nil {
		return nil, err
	}
	// The generator only fills new blocks. Blocks already planned for this
	// weekday get the child here, without re-planning the week, so edits made
	// to single dates stay untouched.
	instances, listStats, err := s.store.ListPickupExtensionTemplateInstances(ctx, block.ID, task.StudentID, task.Weekday, validFrom)
	stats.Add(listStats)
	if err != nil {
		return nil, err
	}
	instanceIDs := make([]int64, 0, len(instances))
	for _, instance := range instances {
		if err := s.addPlannedStudent(ctx, instance.ID, task.StudentID, instance.Date, stats); err != nil {
			return nil, err
		}
		instanceIDs = append(instanceIDs, instance.ID)
	}
	return instanceIDs, nil
}

func (s *Service) ensurePickupWeekdayEnrollment(
	ctx context.Context,
	studentID int64,
	block domain.PickupExtensionBlock,
	weekday int,
	validFrom string,
	stats *domain.OperationStats,
) error {
	enrollments, listStats, err := s.store.ListStudentEnrollments(ctx, domain.StudentEnrollmentFilter{
		StudentIDs: []int64{studentID}, ActivityGroupIDs: []int64{block.ID},
	})
	stats.Add(listStats)
	if err != nil {
		return err
	}
	for _, enrollment := range enrollments {
		if enrollment.ValidUntil != nil || enrollment.Weekday == nil || *enrollment.Weekday != weekday ||
			!samePickupExtensionPeriod(enrollment.CalendarPeriodID, block.CalendarPeriodID) {
			continue
		}
		fields := domain.StudentEnrollmentFields{
			StudentID:                enrollment.StudentID,
			ActivityGroupID:          enrollment.ActivityGroupID,
			ValidFrom:                min(enrollment.ValidFrom, validFrom),
			ValidUntil:               enrollment.ValidUntil,
			CalendarPeriodID:         enrollment.CalendarPeriodID,
			EnrollmentRequestChildID: enrollment.EnrollmentRequestChildID,
			SelectedWeekdays:         enrollment.SelectedWeekdays,
			AttendanceStatus:         enrollment.AttendanceStatus,
			Weekday:                  enrollment.Weekday,
		}
		if len(fields.SelectedWeekdays) > 0 && !slices.Contains(fields.SelectedWeekdays, weekday) {
			fields.SelectedWeekdays = append(fields.SelectedWeekdays, weekday)
		}
		_, found, updateStats, updateErr := s.store.UpdateStudentEnrollment(ctx, enrollment.ID, fields)
		stats.Add(updateStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return domain.ErrStudentEnrollmentNotFound
		}
		return nil
	}
	_, enrollStats, err := s.store.CreateStudentEnrollment(ctx, domain.StudentEnrollmentFields{
		StudentID:        studentID,
		ActivityGroupID:  block.ID,
		ValidFrom:        validFrom,
		CalendarPeriodID: block.CalendarPeriodID,
		Weekday:          &weekday,
	})
	stats.Add(enrollStats)
	if err != nil {
		return err
	}
	return nil
}

func samePickupExtensionPeriod(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// addPlannedStudent adds the child as expected, like the generator does, and
// re-applies sick days and partial absences of that date.
func (s *Service) addPlannedStudent(ctx context.Context, instanceID, studentID int64, date string, stats *domain.OperationStats) error {
	_, createStats, err := s.store.CreateInstanceStudent(ctx, domain.InstanceStudentFields{
		InstanceID: instanceID,
		StudentID:  studentID,
		Status:     expectedAttendanceStatus,
	})
	stats.Add(createStats)
	if err != nil {
		return err
	}
	if _, err := s.ApplyActiveStatusDaysForInstance(ctx, instanceID, date); err != nil {
		return err
	}
	_, err = s.ApplyActivePartialAbsencesForInstance(ctx, instanceID, date)
	return err
}

// pickupExtensionBlocks loads the overlapping blocks for all tasks, keyed by
// task ID. A template whose dynamic target group already contains the child
// counts as a block the child is on.
func (s *Service) pickupExtensionBlocks(ctx context.Context, tasks []domain.PickupExtensionTask, stats *domain.OperationStats) (map[int64][]domain.PickupExtensionBlock, error) {
	dayTasks := make([]domain.PickupExtensionTask, 0, len(tasks))
	weekdayTasks := make([]domain.PickupExtensionTask, 0, len(tasks))
	for _, task := range tasks {
		if task.IsDay() {
			dayTasks = append(dayTasks, task)
		} else {
			weekdayTasks = append(weekdayTasks, task)
		}
	}
	byTask := make(map[int64][]domain.PickupExtensionBlock, len(tasks))
	if len(dayTasks) > 0 {
		blocks, queryStats, err := s.store.ListPickupExtensionDayBlocks(ctx, dayTasks)
		stats.Add(queryStats)
		if err != nil {
			return nil, err
		}
		for _, block := range blocks {
			byTask[block.TaskID] = append(byTask[block.TaskID], block)
		}
	}
	if len(weekdayTasks) == 0 {
		return byTask, nil
	}
	blocks, queryStats, err := s.store.ListPickupExtensionWeekdayBlocks(ctx, weekdayTasks)
	stats.Add(queryStats)
	if err != nil {
		return nil, err
	}
	tasksByID := make(map[int64]domain.PickupExtensionTask, len(weekdayTasks))
	for _, task := range weekdayTasks {
		tasksByID[task.ID] = task
	}
	targets, err := s.pickupExtensionTargets(ctx, blocks, tasksByID, stats)
	if err != nil {
		return nil, err
	}
	for _, block := range blocks {
		if !block.Member && targets[block.TaskID][block.ID] {
			block.Member = true
		}
		byTask[block.TaskID] = append(byTask[block.TaskID], block)
	}
	return byTask, nil
}

func (s *Service) pickupExtensionTargets(
	ctx context.Context,
	blocks []domain.PickupExtensionBlock,
	tasks map[int64]domain.PickupExtensionTask,
	stats *domain.OperationStats,
) (map[int64]map[int64]bool, error) {
	templateIDs := make([]int64, 0, len(blocks))
	for _, block := range blocks {
		if !block.Member {
			templateIDs = append(templateIDs, block.ID)
		}
	}
	templateIDs = uniquePositiveIDs(templateIDs)
	if len(templateIDs) == 0 {
		return map[int64]map[int64]bool{}, nil
	}
	targetRules, students, err := s.loadTargetStudentCandidates(ctx, templateIDs, stats)
	if err != nil {
		return nil, err
	}
	targetsByDate := make(map[string]map[int64][]int64, len(tasks))
	result := make(map[int64]map[int64]bool, len(tasks))
	for _, block := range blocks {
		if block.Member {
			continue
		}
		task := tasks[block.TaskID]
		targets, found := targetsByDate[task.EffectiveFrom]
		if !found {
			targets = matchTargetStudents(targetRules, students, task.EffectiveFrom)
			targetsByDate[task.EffectiveFrom] = targets
		}
		if !slices.Contains(targets[block.ID], task.StudentID) {
			continue
		}
		if result[block.TaskID] == nil {
			result[block.TaskID] = make(map[int64]bool)
		}
		result[block.TaskID][block.ID] = true
	}
	return result, nil
}

func uniquePositiveIDs(values []int64) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value > 0 {
			result = append(result, value)
		}
	}
	slices.Sort(result)
	return slices.Compact(result)
}
