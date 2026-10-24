package application

import (
	"context"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	internalports "github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
)

type Service struct {
	store               internalports.Store
	tx                  internalports.Transaction
	students            internalports.StudentDirectory
	rooms               internalports.RoomDirectory
	sessions            internalports.SessionFacts
	today               func() string
	observe             internalports.Observer
	lockStaffAssignment func(context.Context, int64) error
	provenance          internalports.EnrollmentProvenanceStore
}

func New(store internalports.Store, tx internalports.Transaction, students internalports.StudentDirectory, rooms internalports.RoomDirectory, sessions internalports.SessionFacts, lockStaffAssignment func(context.Context, int64) error, today func() string, observe internalports.Observer, provenance internalports.EnrollmentProvenanceStore) *Service {
	if store == nil || tx == nil || students == nil || rooms == nil || sessions == nil || lockStaffAssignment == nil || today == nil || observe == nil || provenance == nil {
		panic("timetable application: all dependencies are required")
	}
	return &Service{store: store, tx: tx, students: students, rooms: rooms, sessions: sessions, lockStaffAssignment: lockStaffAssignment, today: today, observe: observe, provenance: provenance}
}

// idSet answers membership questions the session facts port returns as lists.
func idSet(ids []int64, err error) (map[int64]bool, error) {
	if err != nil {
		return nil, err
	}
	result := make(map[int64]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result, nil
}

// withoutIDs returns the ids that are not in the excluded set.
func withoutIDs(ids []int64, excluded map[int64]bool) []int64 {
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !excluded[id] {
			result = append(result, id)
		}
	}
	return result
}

// startedInstanceIDs asks Student Presence which of the instances already
// have an execution.
func (s *Service) startedInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]bool, error) {
	if len(instanceIDs) == 0 {
		return map[int64]bool{}, nil
	}
	return idSet(s.sessions.StartedInstanceIDs(ctx, instanceIDs))
}

func (s *Service) completedInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]bool, error) {
	if len(instanceIDs) == 0 {
		return map[int64]bool{}, nil
	}
	return idSet(s.sessions.CompletedInstanceIDs(ctx, instanceIDs))
}

func (s *Service) observedParticipantIDs(ctx context.Context, participantIDs []int64) (map[int64]bool, error) {
	if len(participantIDs) == 0 {
		return map[int64]bool{}, nil
	}
	return idSet(s.sessions.ObservedParticipantIDs(ctx, participantIDs))
}

func (s *Service) FindCategory(ctx context.Context, id int64) (result timetable.Category, err error) {
	if id <= 0 {
		return timetable.Category{}, s.reject("find_category", timetable.ErrInvalidCategory)
	}

	err = s.run(ctx, "find_category", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindCategory(ctx, id, "")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrCategoryNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) FindCategoryForAssignment(ctx context.Context, id int64) (result timetable.Category, err error) {
	if id <= 0 {
		return timetable.Category{}, s.reject("find_category_for_assignment", timetable.ErrInvalidCategory)
	}

	err = s.observeOperation(ctx, "find_category_for_assignment", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindCategory(ctx, id, "SHARE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrCategoryNotFound
		}
		if value.IsArchived() {
			return timetable.ErrCategoryArchived
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) FindCategoryForShare(ctx context.Context, id int64) (result timetable.Category, err error) {
	if id <= 0 {
		return timetable.Category{}, s.reject("find_category_for_share", timetable.ErrInvalidCategory)
	}

	err = s.observeOperation(ctx, "find_category_for_share", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindCategory(ctx, id, "SHARE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrCategoryNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) FindCategoryByName(ctx context.Context, name string) (result timetable.Category, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return timetable.Category{}, s.reject("find_category_by_name", timetable.ErrInvalidCategory)
	}

	err = s.run(ctx, "find_category_by_name", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindCategoryByName(ctx, name, false, "")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrCategoryNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) FindCategoryByNameForAssignment(ctx context.Context, name string) (result timetable.Category, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return timetable.Category{}, s.reject("find_category_by_name_for_assignment", timetable.ErrInvalidCategory)
	}

	err = s.observeOperation(ctx, "find_category_by_name_for_assignment", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindCategoryByName(ctx, name, true, "SHARE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrCategoryNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListCategories(ctx context.Context) (result []timetable.Category, err error) {
	err = s.run(ctx, "list_categories", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListCategories(ctx)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CountCategoryUsage(ctx context.Context) (result map[int64]int, err error) {
	err = s.run(ctx, "count_category_usage", func(ctx context.Context, stats *domain.OperationStats) error {
		counts, queryStats, countErr := s.store.CountCategoryUsage(ctx)
		stats.Add(queryStats)
		result = counts
		return countErr
	})
	return result, err
}

func (s *Service) FindGroup(ctx context.Context, id int64) (result timetable.Group, err error) {
	if id <= 0 {
		return timetable.Group{}, s.reject("find_group", timetable.ErrInvalidGroupQuery)
	}

	err = s.run(ctx, "find_group", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindGroup(ctx, id, "")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrGroupNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) FindGroupForUpdate(ctx context.Context, id int64) (result timetable.Group, err error) {
	if id <= 0 {
		return timetable.Group{}, s.reject("find_group_for_update", timetable.ErrInvalidGroupQuery)
	}

	err = s.observeOperation(ctx, "find_group_for_update", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindGroup(ctx, id, "UPDATE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrGroupNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) FindGroupByName(ctx context.Context, name string) (result timetable.Group, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return timetable.Group{}, s.reject("find_group_by_name", timetable.ErrInvalidGroupQuery)
	}

	err = s.run(ctx, "find_group_by_name", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindGroupByName(ctx, name)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrGroupNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListGroups(ctx context.Context, filter timetable.GroupFilter) (result []timetable.Group, err error) {
	if hasInvalidID(filter.IDs) || hasInvalidID(filter.SourceOfferingIDs) ||
		(filter.CategoryID != nil && *filter.CategoryID <= 0) ||
		(filter.SeriesForGroupID != nil && *filter.SeriesForGroupID <= 0) {
		return nil, s.reject("list_groups", timetable.ErrInvalidGroupQuery)
	}

	err = s.run(ctx, "list_groups", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListGroups(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ListCourseGroups(ctx context.Context, filter timetable.CourseGroupFilter) (result []timetable.CourseGroup, err error) {
	if hasInvalidID(filter.LegacyGroupIDs) || hasInvalidID(filter.SourceOfferingIDs) {
		return nil, s.reject("list_course_groups", timetable.ErrInvalidGroupQuery)
	}
	if len(filter.LegacyGroupIDs) == 0 && len(filter.SourceOfferingIDs) == 0 {
		return []timetable.CourseGroup{}, nil
	}

	err = s.run(ctx, "list_course_groups", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListCourseGroups(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ListTemplateRows(ctx context.Context, templateID *int64) (result []timetable.TemplateListRow, err error) {
	if templateID != nil && *templateID <= 0 {
		return nil, s.reject("list_template_rows", timetable.ErrInvalidGroupQuery)
	}

	err = s.run(ctx, "list_template_rows", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListTemplateRows(ctx, templateID, s.today())
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ListTemplateRowsForTemplatePeriod(ctx context.Context, templateID, periodID int64) (result []timetable.TemplateListRow, err error) {
	if templateID <= 0 || periodID <= 0 {
		return nil, s.reject("list_template_rows_for_template_period", timetable.ErrInvalidGroupQuery)
	}

	err = s.run(ctx, "list_template_rows_for_template_period", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListTemplateRowsForTemplatePeriod(ctx, templateID, periodID, s.today())
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ListTemplateRowsForPeriod(ctx context.Context, periodID *int64) (result []timetable.TemplateListRow, err error) {
	if periodID != nil && *periodID <= 0 {
		return nil, s.reject("list_template_rows_for_period", timetable.ErrInvalidGroupQuery)
	}

	err = s.run(ctx, "list_template_rows_for_period", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListTemplateRowsForPeriod(ctx, periodID, s.today())
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ListTemplateWeekdayRoster(ctx context.Context, templateID, periodID *int64) (result []timetable.TemplateWeekdayRosterRow, err error) {
	if (templateID != nil && *templateID <= 0) || (periodID != nil && *periodID <= 0) {
		return nil, s.reject("list_template_weekday_roster", timetable.ErrInvalidGroupQuery)
	}

	err = s.run(ctx, "list_template_weekday_roster", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListTemplateWeekdayRoster(ctx, templateID, periodID, s.today())
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ListTemplateCapacityOccurrences(ctx context.Context, periodID *int64, templateIDs []int64, periods []timetable.TemplateCapacityPeriod) (result []timetable.TemplateCapacityOccurrence, err error) {
	if (periodID != nil && *periodID <= 0) || hasInvalidID(templateIDs) {
		return nil, s.reject("list_template_capacity_occurrences", timetable.ErrInvalidGroupQuery)
	}
	if len(templateIDs) == 0 {
		return []timetable.TemplateCapacityOccurrence{}, nil
	}

	dynamicStudents, err := s.ListTargetStudentIDs(ctx, templateIDs)
	if err != nil {
		return nil, err
	}
	err = s.run(ctx, "list_template_capacity_occurrences", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListTemplateCapacityOccurrences(ctx, periodID, templateIDs, periods, dynamicStudents)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ListGroupTargets(ctx context.Context, ids []int64) (result map[int64][]timetable.GroupTarget, err error) {
	if hasInvalidID(ids) {
		return nil, s.reject("list_group_targets", timetable.ErrInvalidGroupQuery)
	}

	err = s.run(ctx, "list_group_targets", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListGroupTargets(ctx, ids)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ListTargetStudentIDs(ctx context.Context, ids []int64) (result map[int64][]int64, err error) {
	if hasInvalidID(ids) {
		return nil, s.reject("list_target_student_ids", timetable.ErrInvalidGroupQuery)
	}

	err = s.run(ctx, "list_target_student_ids", func(ctx context.Context, stats *domain.OperationStats) error {
		targets, students, listErr := s.loadTargetStudentCandidates(ctx, ids, stats)
		if listErr != nil {
			return listErr
		}
		result = matchTargetStudents(targets, students, s.today())
		return nil
	})
	return result, err
}

func (s *Service) loadTargetStudentCandidates(
	ctx context.Context, ids []int64, stats *domain.OperationStats,
) (map[int64][]timetable.GroupTarget, []domain.TargetStudent, error) {
	targets, targetStats, err := s.store.ListGroupTargets(ctx, ids)
	stats.Add(targetStats)
	if err != nil || len(targets) == 0 {
		return targets, nil, err
	}
	students, studentStats, err := s.students.ListEnrolledStudents(ctx)
	stats.Add(studentStats)
	if err != nil {
		return nil, nil, err
	}
	return targets, students, nil
}

func (s *Service) ReplaceGroupTargets(ctx context.Context, groupID int64, targets []timetable.GroupTargetInput) error {
	targets, err := normalizeGroupTargets(groupID, targets)
	if err != nil {
		return s.reject("replace_group_targets", err)
	}
	return s.runWrite(ctx, "replace_group_targets", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.ReplaceGroupTargets(txCtx, groupID, targets)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) CreateGroup(ctx context.Context, fields timetable.GroupInput) (result timetable.Group, err error) {
	if err := normalizeGroup(&fields); err != nil {
		return timetable.Group{}, s.reject("create_group", err)
	}

	err = s.runWrite(ctx, "create_group", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateGroup(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdateGroup(ctx context.Context, id int64, fields timetable.GroupInput) (result timetable.Group, err error) {
	if id <= 0 {
		return timetable.Group{}, s.reject("update_group", timetable.ErrInvalidGroup)
	}
	if err := normalizeGroup(&fields); err != nil {
		return timetable.Group{}, s.reject("update_group", err)
	}

	err = s.runWrite(ctx, "update_group", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, updated, queryStats, updateErr := s.store.UpdateGroup(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !updated {
			return timetable.ErrGroupNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) DeleteGroup(ctx context.Context, id int64) error {
	if id <= 0 {
		return s.reject("delete_group", timetable.ErrInvalidGroup)
	}

	return s.runWrite(ctx, "delete_group", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteGroup(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) UpdateTemplate(ctx context.Context, id int64, fields timetable.TemplateUpdate) (result int64, err error) {
	if id <= 0 || !normalizeTemplateUpdate(&fields) {
		return 0, s.reject("update_template", timetable.ErrInvalidGroup)
	}

	err = s.runWrite(ctx, "update_template", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, updateErr := s.store.UpdateTemplate(txCtx, id, fields)
		stats.Add(queryStats)
		result = rows
		if updateErr != nil || rows != 1 {
			return updateErr
		}
		return s.dropOccurrencesAfterSeriesEnd(txCtx, id, fields, stats)
	})
	return result, err
}

func (s *Service) ArchiveTemplate(ctx context.Context, id int64) (result int64, err error) {
	if id <= 0 {
		return 0, s.reject("archive_template", timetable.ErrInvalidGroup)
	}

	err = s.runWrite(ctx, "archive_template", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, archiveErr := s.store.ArchiveTemplate(txCtx, id)
		stats.Add(queryStats)
		result = rows
		return archiveErr
	})
	return result, err
}

func (s *Service) UpdateGroupOfferingSource(ctx context.Context, id int64, fields timetable.OfferingSourceInput) error {
	source := timetable.GroupInput{TargetGroupType: timetable.TargetGroupTypeOffering,
		SourceCareOfferingIDs: fields.CareOfferingIDs, SourceGradeLevels: fields.GradeLevels,
		SourceSchoolClasses: fields.SchoolClasses}
	if id <= 0 || !normalizeOfferingSource(&source) {
		return s.reject("update_group_offering_source", timetable.ErrInvalidGroup)
	}
	fields.CareOfferingIDs = source.SourceCareOfferingIDs
	fields.GradeLevels = source.SourceGradeLevels
	fields.SchoolClasses = source.SourceSchoolClasses

	return s.runWrite(ctx, "update_group_offering_source", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.UpdateGroupOfferingSource(txCtx, id, fields)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) CreateCategory(ctx context.Context, fields timetable.CreateCategory) (result timetable.Category, err error) {
	started := time.Now()
	if err := normalizeCategory(&fields.Name, &fields.Description, &fields.Color); err != nil {
		s.observeRejection("create_category", time.Since(started), err)
		return timetable.Category{}, err
	}
	if !fields.IsSystem && reservedCategoryName(fields.Name) {
		s.observeRejection("create_category", time.Since(started), timetable.ErrSystemCategoryName)
		return timetable.Category{}, timetable.ErrSystemCategoryName
	}

	err = s.runWrite(ctx, "create_category", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateCategory(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdateCategory(ctx context.Context, input timetable.UpdateCategory) (result timetable.Category, err error) {
	started := time.Now()
	if input.ID <= 0 {
		s.observeRejection("update_category", time.Since(started), timetable.ErrInvalidCategory)
		return timetable.Category{}, timetable.ErrInvalidCategory
	}
	if err := normalizeCategory(&input.Name, &input.Description, &input.Color); err != nil {
		s.observeRejection("update_category", time.Since(started), err)
		return timetable.Category{}, err
	}
	if reservedCategoryName(input.Name) {
		s.observeRejection("update_category", time.Since(started), timetable.ErrSystemCategoryName)
		return timetable.Category{}, timetable.ErrSystemCategoryName
	}
	id := input.ID
	fields := timetable.CreateCategory{Name: input.Name, Description: input.Description, Color: input.Color}
	err = s.runWrite(ctx, "update_category", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		existing, err := s.editableCategory(txCtx, id, stats)
		if err != nil {
			return err
		}
		if existing.IsArchived() {
			return timetable.ErrCategoryArchived
		}
		value, updated, queryStats, updateErr := s.store.UpdateCategoryIfActive(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !updated {
			return timetable.ErrCategoryArchived
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ArchiveCategory(ctx context.Context, id int64) (result timetable.Category, err error) {
	if id <= 0 {
		return timetable.Category{}, s.reject("archive_category", timetable.ErrInvalidCategory)
	}

	err = s.runWrite(ctx, "archive_category", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		existing, editErr := s.editableCategory(txCtx, id, stats)
		if editErr != nil {
			return editErr
		}
		if existing.IsArchived() {
			result = existing
			return nil
		}
		now := time.Now()
		return s.setArchivedAt(txCtx, id, &now, stats, &result)
	})
	return result, err
}

func (s *Service) RestoreCategory(ctx context.Context, id int64) (result timetable.Category, err error) {
	if id <= 0 {
		return timetable.Category{}, s.reject("restore_category", timetable.ErrInvalidCategory)
	}

	err = s.runWrite(ctx, "restore_category", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		existing, editErr := s.editableCategory(txCtx, id, stats)
		if editErr != nil {
			return editErr
		}
		if !existing.IsArchived() {
			result = existing
			return nil
		}
		return s.setArchivedAt(txCtx, id, nil, stats, &result)
	})
	return result, err
}

func (s *Service) SetCategoryShiftTypeLinks(ctx context.Context, shiftTypeID int64, categoryIDs []int64) error {
	if shiftTypeID <= 0 || hasInvalidID(categoryIDs) {
		return s.reject("set_category_shift_type_links", timetable.ErrInvalidCategory)
	}

	return s.runWrite(ctx, "set_category_shift_type_links", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.SetCategoryShiftTypeLinks(txCtx, shiftTypeID, categoryIDs)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) SetCategoryShiftTypeID(ctx context.Context, id int64, shiftTypeID *int64) error {
	if id <= 0 || (shiftTypeID != nil && *shiftTypeID <= 0) {
		return s.reject("set_category_shift_type_id", timetable.ErrInvalidCategory)
	}

	return s.runWrite(ctx, "set_category_shift_type_id", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.SetCategoryShiftTypeID(txCtx, id, shiftTypeID)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	if id <= 0 {
		return s.reject("delete_category", timetable.ErrInvalidCategory)
	}

	return s.runWrite(ctx, "delete_category", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteCategory(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) LockStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string) error {
	if len(studentIDs) == 0 || hasInvalidID(studentIDs) || !timetable.ValidDate(validUntil) {
		return s.reject("lock_student_enrollments_for_care_exit", timetable.ErrInvalidCareExitEnrollment)
	}

	return s.observeOperation(ctx, "lock_student_enrollments_for_care_exit", func(ctx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.LockStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) EndStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string) (result timetable.CareExitEnrollmentChanges, err error) {
	if len(studentIDs) == 0 || hasInvalidID(studentIDs) || !timetable.ValidDate(validUntil) {
		return timetable.CareExitEnrollmentChanges{}, s.reject("end_student_enrollments_for_care_exit", timetable.ErrInvalidCareExitEnrollment)
	}

	err = s.runWrite(ctx, "end_student_enrollments_for_care_exit", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		changes, queryStats, endErr := s.store.EndStudentEnrollmentsForCareExit(txCtx, studentIDs, validUntil)
		stats.Add(queryStats)
		result = changes
		return endErr
	})
	return result, err
}

// RestoreStudentEnrollmentsForCareExit restores a cancelled care exit's
// enrollments through the provenance store.
func (s *Service) RestoreStudentEnrollmentsForCareExit(ctx context.Context, studentIDs, periodIDs []int64, removals []timetable.CareExitEnrollmentRemoval) (result int, err error) {
	if len(studentIDs) == 0 || hasInvalidID(studentIDs) || hasInvalidID(periodIDs) || invalidCareExitRemovals(removals) {
		return 0, s.reject("restore_student_enrollments_for_care_exit", timetable.ErrInvalidCareExitEnrollment)
	}
	err = s.runWrite(ctx, "restore_student_enrollments_for_care_exit", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, restoreErr := s.provenance.RestoreStudentEnrollmentsForCareExit(txCtx, studentIDs, periodIDs, removals)
		stats.Add(queryStats)
		result = int(rows)
		return restoreErr
	})
	return result, err
}

func (s *Service) editableCategory(ctx context.Context, id int64, stats *domain.OperationStats) (timetable.Category, error) {
	category, found, queryStats, err := s.store.FindCategory(ctx, id, "UPDATE")
	stats.Add(queryStats)
	if err != nil {
		return timetable.Category{}, err
	}
	if !found {
		return timetable.Category{}, timetable.ErrCategoryNotFound
	}
	if category.IsSystem {
		return timetable.Category{}, timetable.ErrSystemCategoryProtected
	}
	return category, nil
}

func (s *Service) setArchivedAt(ctx context.Context, id int64, value *time.Time, stats *domain.OperationStats, result *timetable.Category) error {
	category, updated, queryStats, err := s.store.SetCategoryArchivedAt(ctx, id, value)
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	if !updated {
		return timetable.ErrCategoryNotFound
	}
	*result = category
	return nil
}

// reject observes a request the owner edge refused before any operation ran.
func (s *Service) reject(operation string, err error) error {
	s.observeRejection(operation, 0, err)
	return err
}

func (s *Service) observeRejection(operation string, duration time.Duration, err error) {
	s.observe(internalports.Observation{Operation: operation, Duration: duration, Err: err})
}

func (s *Service) run(ctx context.Context, operation string, callback func(context.Context, *domain.OperationStats) error) error {
	return s.runWrite(ctx, operation, false, callback)
}

func (s *Service) observeOperation(ctx context.Context, operation string, callback func(context.Context, *domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	err = callback(ctx, &stats)
	s.observe(internalports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	return err
}

func (s *Service) runWrite(ctx context.Context, operation string, retry bool, callback func(context.Context, *domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	err = s.tx.RunWrite(ctx, retry, func(txCtx context.Context) error {
		return callback(txCtx, &stats)
	})
	s.observe(internalports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	return err
}
