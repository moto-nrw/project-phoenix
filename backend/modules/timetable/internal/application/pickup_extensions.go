package application

import (
	"context"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) pickupExtensionToday() domain.Date { return domain.Date(s.today()) }

// RecordPickupDayExtension stores the later day pickup. Past days never open
// a task; the call also prunes past day tasks of the school, so stale rows
// cannot pile up.
func (s *Service) RecordPickupDayExtension(ctx context.Context, input timetable.PickupDayExtension) error {
	if input.StudentID <= 0 || input.PickupExceptionID <= 0 || !timetable.ValidDate(input.Date) ||
		!validPickupClock(input.PreviousPickup) || !validPickupClock(input.Pickup) || input.Pickup <= input.PreviousPickup {
		return s.reject("record_pickup_day_extension", timetable.ErrInvalidPickupExtension)
	}
	exceptionID := input.PickupExceptionID
	task := domain.PickupExtensionTask{
		StudentID: input.StudentID, PickupExceptionID: &exceptionID, Date: domain.Date(input.Date),
		PreviousPickup: input.PreviousPickup, Pickup: input.Pickup,
	}
	return s.runWrite(ctx, "record_pickup_day_extension", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		today := s.pickupExtensionToday()
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
	if studentID <= 0 || !timetable.ValidDate(date) {
		return s.reject("clear_pickup_day_extension", timetable.ErrInvalidPickupExtension)
	}
	return s.runWrite(ctx, "clear_pickup_day_extension", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeletePickupDayExtensionTask(txCtx, studentID, domain.Date(date))
		stats.Add(queryStats)
		return err
	})
}

// RecordPickupWeekdayExtension stores a lasting weekday change. Several
// changes in a row keep the earliest previous time: 14:45 → 16:00 → 15:30
// still needs a block from 14:45 on. A change back to that time or earlier
// closes the task.
func (s *Service) RecordPickupWeekdayExtension(ctx context.Context, input timetable.PickupWeekdayExtension) error {
	if input.StudentID <= 0 || input.Weekday < 1 || input.Weekday > 5 || !timetable.ValidDate(input.EffectiveFrom) ||
		!validPickupClock(input.PreviousPickup) || !validPickupClock(input.Pickup) || input.Pickup == input.PreviousPickup {
		return s.reject("record_pickup_weekday_extension", timetable.ErrInvalidPickupExtension)
	}
	task := domain.PickupExtensionTask{
		StudentID: input.StudentID, Weekday: input.Weekday, EffectiveFrom: domain.Date(input.EffectiveFrom),
		PreviousPickup: input.PreviousPickup, Pickup: input.Pickup,
	}
	return s.runWrite(ctx, "record_pickup_weekday_extension", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		tasks, listStats, err := s.store.ListPickupExtensionTasks(txCtx, task.StudentID, s.pickupExtensionToday())
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
	if studentID <= 0 || weekday < 1 || weekday > 5 {
		return s.reject("clear_pickup_weekday_extension", timetable.ErrInvalidPickupExtension)
	}

	return s.runWrite(ctx, "clear_pickup_weekday_extension", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeletePickupWeekdayExtensionTask(txCtx, studentID, weekday)
		stats.Add(queryStats)
		return err
	})
}

// ListOpenPickupExtensions returns unresolved tasks with their available
// choices. A task without a matching block remains resolvable with "no
// block"; only a child already covered until pickup drops the task.
func (s *Service) ListOpenPickupExtensions(ctx context.Context, studentID int64) (result []timetable.PickupExtensionTask, err error) {
	if studentID < 0 {
		return nil, s.reject("list_open_pickup_extensions", timetable.ErrInvalidPickupExtension)
	}
	err = s.run(ctx, "list_open_pickup_extensions", func(ctx context.Context, stats *domain.OperationStats) error {
		tasks, queryStats, listErr := s.store.ListPickupExtensionTasks(ctx, studentID, s.pickupExtensionToday())
		stats.Add(queryStats)
		if listErr != nil {
			return listErr
		}
		blocks, blockErr := s.pickupExtensionBlocks(ctx, tasks, stats)
		if blockErr != nil {
			return blockErr
		}
		result = make([]timetable.PickupExtensionTask, 0, len(tasks))
		for _, task := range tasks {
			open := domain.OpenPickupExtensionBlocks(blocks[task.ID], task.Pickup)
			if open != nil {
				result = append(result, pickupExtensionTaskToPublic(task, open))
			}
		}
		stats.Rows = int64(len(result))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// FindPickupExtensionStudent reads the affected child before a caller starts
// the resolve command. The command still locks and re-reads the task to keep
// its write-side validation race-safe.
func (s *Service) FindPickupExtensionStudent(ctx context.Context, taskID int64) (studentID int64, err error) {
	if taskID <= 0 {
		return 0, s.reject("find_pickup_extension_student", timetable.ErrInvalidPickupExtension)
	}

	err = s.run(ctx, "find_pickup_extension_student", func(ctx context.Context, stats *domain.OperationStats) error {
		task, found, queryStats, queryErr := s.store.FindPickupExtensionTask(ctx, taskID)
		stats.Add(queryStats)
		if queryErr != nil {
			return queryErr
		}
		if !found {
			return timetable.ErrPickupExtensionNotFound
		}
		studentID = task.StudentID
		return nil
	})
	return studentID, err
}

// pickupExtensionResolution is what ResolvePickupExtension changed.
type pickupExtensionResolution struct {
	Task        domain.PickupExtensionTask
	Assigned    []domain.PickupExtensionBlock
	InstanceIDs []int64
	// Instances are the concrete blocks the child was added to, with their
	// dates, so the caller can have Student Presence re-apply the day's
	// reported statuses and partial absences.
	Instances []domain.PickupExtensionInstance
}

// ResolvePickupExtension adds the child to the chosen blocks and removes the
// task. Every chosen block must still be a choice; otherwise nothing changes.
func (s *Service) ResolvePickupExtension(ctx context.Context, taskID int64, blockIDs []int64) (timetable.PickupExtensionResolution, error) {
	if taskID <= 0 || len(blockIDs) > 20 || slices.ContainsFunc(blockIDs, func(id int64) bool { return id <= 0 }) {
		return timetable.PickupExtensionResolution{}, s.reject("resolve_pickup_extension", timetable.ErrInvalidPickupExtension)
	}
	var result pickupExtensionResolution
	err := s.runWrite(ctx, "resolve_pickup_extension", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		task, found, findStats, findErr := s.store.FindPickupExtensionTaskForUpdate(txCtx, taskID)
		stats.Add(findStats)
		if findErr != nil {
			return findErr
		}
		if !found || (task.IsDay() && task.Date < s.pickupExtensionToday()) {
			return timetable.ErrPickupExtensionNotFound
		}
		result.Task = task
		chosen, chooseErr := s.chosenPickupExtensionBlocks(txCtx, task, blockIDs, stats)
		if chooseErr != nil {
			return chooseErr
		}
		instances, assignErr := s.assignPickupExtensionBlocks(txCtx, task, chosen, stats)
		if assignErr != nil {
			return assignErr
		}
		result.Assigned, result.Instances = chosen, instances
		for _, instance := range instances {
			result.InstanceIDs = append(result.InstanceIDs, instance.ID)
		}
		deleteStats, deleteErr := s.store.DeletePickupExtensionTask(txCtx, task.ID)
		stats.Add(deleteStats)
		return deleteErr
	})
	if err != nil {
		return timetable.PickupExtensionResolution{}, err
	}
	return timetable.PickupExtensionResolution{
		StudentID:      result.Task.StudentID,
		Kind:           pickupExtensionKind(result.Task),
		AssignedBlocks: pickupExtensionBlocksToPublic(result.Assigned),
		InstanceIDs:    append([]int64{}, result.InstanceIDs...),
		Instances:      pickupExtensionInstancesToPublic(result.Instances),
	}, nil
}

func pickupExtensionTaskToPublic(task domain.PickupExtensionTask, blocks []domain.PickupExtensionBlock) timetable.PickupExtensionTask {
	return timetable.PickupExtensionTask{
		ID:             task.ID,
		StudentID:      task.StudentID,
		Kind:           pickupExtensionKind(task),
		Date:           task.Date.String(),
		Weekday:        task.Weekday,
		EffectiveFrom:  task.EffectiveFrom.String(),
		PreviousPickup: task.PreviousPickup,
		Pickup:         task.Pickup,
		Blocks:         pickupExtensionBlocksToPublic(blocks),
	}
}

func pickupExtensionKind(task domain.PickupExtensionTask) string {
	if task.IsDay() {
		return timetable.PickupExtensionKindDay
	}
	return timetable.PickupExtensionKindWeekday
}

func pickupExtensionBlocksToPublic(values []domain.PickupExtensionBlock) []timetable.PickupExtensionBlock {
	result := make([]timetable.PickupExtensionBlock, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.PickupExtensionBlock{
			ID: value.ID, Title: value.Title, StartTime: value.StartTime, EndTime: value.EndTime,
		})
	}
	return result
}

func pickupExtensionInstancesToPublic(values []domain.PickupExtensionInstance) []timetable.PickupExtensionInstance {
	result := make([]timetable.PickupExtensionInstance, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.PickupExtensionInstance{ID: value.ID, Date: value.Date.String()})
	}
	return result
}

func (s *Service) assignPickupExtensionBlocks(ctx context.Context, task domain.PickupExtensionTask, chosen []domain.PickupExtensionBlock, stats *domain.OperationStats) ([]domain.PickupExtensionInstance, error) {
	var instances []domain.PickupExtensionInstance
	for _, block := range chosen {
		assigned, err := s.assignPickupExtensionBlock(ctx, task, block, stats)
		if err != nil {
			return nil, err
		}
		instances = append(instances, assigned...)
	}
	return instances, nil
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
			return nil, timetable.ErrPickupExtensionBlockGone
		}
		chosen = append(chosen, open[index])
	}
	return chosen, nil
}

func (s *Service) assignPickupExtensionBlock(ctx context.Context, task domain.PickupExtensionTask, block domain.PickupExtensionBlock, stats *domain.OperationStats) ([]domain.PickupExtensionInstance, error) {
	if task.IsDay() {
		instance := domain.PickupExtensionInstance{ID: block.ID, Date: task.Date}
		if err := s.lockPickupExtensionInstance(ctx, task, instance, 0, stats); err != nil {
			return nil, err
		}
		if err := s.addPlannedStudent(ctx, block.ID, task.StudentID, stats); err != nil {
			return nil, err
		}
		return []domain.PickupExtensionInstance{instance}, nil
	}
	validFrom := max(task.EffectiveFrom, s.pickupExtensionToday(), block.ValidFrom)
	weekday := task.Weekday
	// The generator only fills new blocks. Blocks already planned for this
	// weekday get the child here, without re-planning the week, so edits made
	// to single dates stay untouched.
	instances, err := s.pickupExtensionTemplateInstances(ctx, block, task, validFrom, stats)
	if err != nil {
		return nil, err
	}
	for _, instance := range instances {
		if err := s.lockPickupExtensionInstance(ctx, task, instance, block.ID, stats); err != nil {
			return nil, err
		}
	}
	if err := s.ensurePickupWeekdayEnrollment(ctx, task.StudentID, block, weekday, validFrom, stats); err != nil {
		return nil, err
	}
	for _, instance := range instances {
		if err := s.addPlannedStudent(ctx, instance.ID, task.StudentID, stats); err != nil {
			return nil, err
		}
	}
	return instances, nil
}

// pickupExtensionTemplateInstances lists the template's planned blocks on the
// weekday that do not list the child as scheduled: a block that already
// ended is not a choice, and a child whose row carries the non-booking
// marker is not scheduled on it.
func (s *Service) pickupExtensionTemplateInstances(ctx context.Context, block domain.PickupExtensionBlock, task domain.PickupExtensionTask, validFrom domain.Date, stats *domain.OperationStats) ([]domain.PickupExtensionInstance, error) {
	candidates, listStats, err := s.store.ListPickupExtensionTemplateInstances(ctx, block.ID, task.StudentID, task.Weekday, validFrom, block.CalendarPeriodID, block.ValidUntil)
	stats.Add(listStats)
	if err != nil || len(candidates) == 0 {
		return nil, err
	}
	instanceIDs := make([]int64, 0, len(candidates))
	ownIDs := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		instanceIDs = append(instanceIDs, candidate.ID)
		if candidate.OwnParticipantID != nil {
			ownIDs = append(ownIDs, *candidate.OwnParticipantID)
		}
	}
	completedIDs, notScheduledIDs, err := s.sessions.ExecutionFacts(ctx, instanceIDs, ownIDs)
	if err != nil {
		return nil, err
	}
	completed, notScheduled := idLookup(completedIDs), idLookup(notScheduledIDs)
	result := make([]domain.PickupExtensionInstance, 0, len(candidates))
	for _, candidate := range candidates {
		if completed[candidate.ID] {
			continue
		}
		if candidate.OwnParticipantID != nil && !notScheduled[*candidate.OwnParticipantID] {
			continue
		}
		result = append(result, candidate)
	}
	return result, nil
}

// pickupExtensionDayBlocks lists the blocks of the day tasks that can still
// be chosen: not ended, with at least one scheduled child, and not carrying
// the child as explicitly not booked. The roster comes with the block; the
// execution and the non-booking markers are one Student Presence read.
func (s *Service) pickupExtensionDayBlocks(ctx context.Context, tasks []domain.PickupExtensionTask, stats *domain.OperationStats) ([]domain.PickupExtensionBlock, error) {
	blocks, queryStats, err := s.store.ListPickupExtensionDayBlocks(ctx, tasks)
	stats.Add(queryStats)
	if err != nil || len(blocks) == 0 {
		return nil, err
	}
	instanceIDs, participantIDs := pickupExtensionBlockIDs(blocks)
	completed, notScheduled, err := s.sessions.ExecutionFacts(ctx, instanceIDs, participantIDs)
	if err != nil {
		return nil, err
	}
	return openPickupExtensionDayBlocks(blocks, idLookup(completed), idLookup(notScheduled)), nil
}

func pickupExtensionBlockIDs(blocks []domain.PickupExtensionBlock) (instanceIDs, participantIDs []int64) {
	for _, block := range blocks {
		instanceIDs = append(instanceIDs, block.ID)
		participantIDs = append(participantIDs, block.ParticipantIDs...)
	}
	return uniquePositiveIDs(instanceIDs), uniquePositiveIDs(participantIDs)
}

func idLookup(ids []int64) map[int64]bool {
	result := make(map[int64]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result
}

func openPickupExtensionDayBlocks(blocks []domain.PickupExtensionBlock, completed, notScheduled map[int64]bool) []domain.PickupExtensionBlock {
	result := make([]domain.PickupExtensionBlock, 0, len(blocks))
	for _, block := range blocks {
		if completed[block.ID] || !pickupExtensionBlockScheduled(block, notScheduled) {
			continue
		}
		if block.OwnParticipantID != nil && notScheduled[*block.OwnParticipantID] {
			continue
		}
		result = append(result, block)
	}
	return result
}

// pickupExtensionBlockScheduled reports whether at least one child of the
// block is still booked on it.
func pickupExtensionBlockScheduled(block domain.PickupExtensionBlock, notScheduled map[int64]bool) bool {
	for _, id := range block.ParticipantIDs {
		if !notScheduled[id] {
			return true
		}
	}
	return false
}

// lockPickupExtensionInstance keeps a selected instance selectable until the
// resolution commits. Lifecycle writers take the same row exclusively, so a
// cancellation, spontaneous conversion or time change that wins the race is
// observed before adding the child.
func (s *Service) lockPickupExtensionInstance(
	ctx context.Context,
	task domain.PickupExtensionTask,
	instance domain.PickupExtensionInstance,
	templateID int64,
	stats *domain.OperationStats,
) error {
	current, found, queryStats, err := s.store.LockActivityInstance(ctx, instance.ID, true)
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	if !found || current.Date != instance.Date.String() || current.IsSpontaneous ||
		current.Status != domain.InstanceStatusPlanned ||
		!pickupExtensionTimesOverlap(current.StartTime, current.EndTime, task.PreviousPickup, task.Pickup) {
		return timetable.ErrPickupExtensionBlockGone
	}
	if templateID > 0 && (current.ActivityGroupID == nil || *current.ActivityGroupID != templateID) {
		return timetable.ErrPickupExtensionBlockGone
	}
	return nil
}

func pickupExtensionTimesOverlap(start, end, from, to string) bool {
	if len(start) < 5 || len(end) < 5 {
		return false
	}
	return start[:5] < to && end[:5] > from
}

func (s *Service) ensurePickupWeekdayEnrollment(
	ctx context.Context,
	studentID int64,
	block domain.PickupExtensionBlock,
	weekday int,
	validFrom domain.Date,
	stats *domain.OperationStats,
) error {
	enrollments, listStats, err := s.store.ListStudentEnrollments(ctx, timetable.StudentEnrollmentFilter{
		StudentIDs: []int64{studentID}, ActivityGroupIDs: []int64{block.ID},
	})
	stats.Add(listStats)
	if err != nil {
		return err
	}
	for _, enrollment := range enrollments {
		if !pickupEnrollmentExtendable(enrollment, block, weekday, validFrom) {
			continue
		}
		fields := extendedPickupEnrollmentFields(enrollment, weekday, validFrom)
		_, found, updateStats, updateErr := s.store.UpdateStudentEnrollment(ctx, enrollment.ID, fields)
		stats.Add(updateStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return timetable.ErrStudentEnrollmentNotFound
		}
		return nil
	}
	_, enrollStats, err := s.store.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID:        studentID,
		ActivityGroupID:  block.ID,
		ValidFrom:        validFrom.String(),
		CalendarPeriodID: block.CalendarPeriodID,
		Weekday:          &weekday,
	})
	stats.Add(enrollStats)
	if err != nil {
		return err
	}
	return nil
}

// pickupEnrollmentExtendable reports whether an existing enrollment of the
// child in the block can carry the weekday from validFrom on: it is still
// valid then, not bound to another weekday, and in the block's period.
func pickupEnrollmentExtendable(enrollment timetable.StudentEnrollment, block domain.PickupExtensionBlock, weekday int, validFrom domain.Date) bool {
	if enrollment.ValidUntil != nil && *enrollment.ValidUntil <= validFrom.String() {
		return false
	}
	if enrollment.Weekday != nil && *enrollment.Weekday != weekday {
		return false
	}
	return samePickupExtensionPeriod(enrollment.CalendarPeriodID, block.CalendarPeriodID)
}

// extendedPickupEnrollmentFields keeps the enrollment, starts it no later than
// validFrom and adds the weekday to a non-empty weekday selection.
func extendedPickupEnrollmentFields(enrollment timetable.StudentEnrollment, weekday int, validFrom domain.Date) timetable.StudentEnrollmentInput {
	fields := timetable.StudentEnrollmentInput{
		StudentID:                enrollment.StudentID,
		ActivityGroupID:          enrollment.ActivityGroupID,
		ValidFrom:                min(enrollment.ValidFrom, validFrom.String()),
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
	return fields
}

func samePickupExtensionPeriod(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// addPlannedStudent adds the child to the roster, like the generator does.
// Re-applying the day's sick days and partial absences is Student Presence's
// rule; the resolution reports the added blocks so the caller runs it.
func (s *Service) addPlannedStudent(ctx context.Context, instanceID, studentID int64, stats *domain.OperationStats) error {
	_, _, createStats, err := s.store.EnsureInstanceStudent(ctx, instanceID, studentID)
	stats.Add(createStats)
	return err
}

// pickupExtensionBlocks loads the overlapping blocks for all tasks, keyed by
// task ID. A template whose dynamic target group already contains the child
// counts as a block the child is on.
func (s *Service) pickupExtensionBlocks(ctx context.Context, tasks []domain.PickupExtensionTask, stats *domain.OperationStats) (map[int64][]domain.PickupExtensionBlock, error) {
	dayTasks, weekdayTasks := partitionPickupExtensionTasks(tasks)
	byTask := make(map[int64][]domain.PickupExtensionBlock, len(tasks))
	if len(dayTasks) > 0 {
		blocks, err := s.pickupExtensionDayBlocks(ctx, dayTasks, stats)
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
	if err := s.collectPickupExtensionWeekdayBlocks(ctx, weekdayTasks, byTask, stats); err != nil {
		return nil, err
	}
	return byTask, nil
}

// partitionPickupExtensionTasks splits the tasks into single-day and weekday
// tasks, keeping their order.
func partitionPickupExtensionTasks(tasks []domain.PickupExtensionTask) ([]domain.PickupExtensionTask, []domain.PickupExtensionTask) {
	dayTasks := make([]domain.PickupExtensionTask, 0, len(tasks))
	weekdayTasks := make([]domain.PickupExtensionTask, 0, len(tasks))
	for _, task := range tasks {
		if task.IsDay() {
			dayTasks = append(dayTasks, task)
		} else {
			weekdayTasks = append(weekdayTasks, task)
		}
	}
	return dayTasks, weekdayTasks
}

// collectPickupExtensionWeekdayBlocks adds the overlapping template blocks of
// the weekday tasks to byTask. A block the child is not enrolled in whose
// template's target group holds the child counts as a block the child is on;
// one whose target group leaves the child out is dropped.
func (s *Service) collectPickupExtensionWeekdayBlocks(ctx context.Context, weekdayTasks []domain.PickupExtensionTask, byTask map[int64][]domain.PickupExtensionBlock, stats *domain.OperationStats) error {
	blocks, queryStats, err := s.store.ListPickupExtensionWeekdayBlocks(ctx, weekdayTasks, s.pickupExtensionToday())
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	tasksByID := make(map[int64]domain.PickupExtensionTask, len(weekdayTasks))
	for _, task := range weekdayTasks {
		tasksByID[task.ID] = task
	}
	targets, err := s.pickupExtensionTargets(ctx, blocks, tasksByID, stats)
	if err != nil {
		return err
	}
	for _, block := range blocks {
		if !block.Member {
			targeted, hasTargetGroup := targets[block.TaskID][block.ID]
			if hasTargetGroup {
				if !targeted {
					continue
				}
				block.Member = true
			}
		}
		byTask[block.TaskID] = append(byTask[block.TaskID], block)
	}
	return nil
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
	targetsByOperationalDate := make(map[domain.Date]map[int64][]int64, len(tasks))
	result := make(map[int64]map[int64]bool, len(tasks))
	for _, block := range blocks {
		if block.Member {
			continue
		}
		if len(targetRules[block.ID]) == 0 {
			continue
		}
		task := tasks[block.TaskID]
		// Target membership must use the same operational date as the schedule:
		// a future task cannot assign a child whose care ends before it starts.
		operationalDate := max(task.EffectiveFrom, s.pickupExtensionToday())
		targets, found := targetsByOperationalDate[operationalDate]
		if !found {
			targets = matchTargetStudents(targetRules, students, operationalDate.String())
			targetsByOperationalDate[operationalDate] = targets
		}
		if result[block.TaskID] == nil {
			result[block.TaskID] = make(map[int64]bool)
		}
		result[block.TaskID][block.ID] = slices.Contains(targets[block.ID], task.StudentID)
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
