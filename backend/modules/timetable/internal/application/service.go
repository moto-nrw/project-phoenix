package application

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
)

type Service struct {
	store      ports.Store
	tx         ports.Transaction
	students   ports.StudentDirectory
	rooms      ports.RoomDirectory
	locks      ports.CareDayLocker
	today      func() string
	observe    ports.Observer
	carePlanMu sync.RWMutex
	carePlan   ports.CarePlanDirectory
}

func New(store ports.Store, tx ports.Transaction, students ports.StudentDirectory, rooms ports.RoomDirectory, locks ports.CareDayLocker, today func() string, observe ports.Observer) *Service {
	if store == nil || tx == nil || students == nil || rooms == nil || locks == nil || today == nil || observe == nil {
		panic("timetable application: all dependencies are required")
	}
	return &Service{store: store, tx: tx, students: students, rooms: rooms, locks: locks, today: today, observe: observe}
}

func (s *Service) BindCarePlan(directory ports.CarePlanDirectory) {
	if directory == nil {
		panic("timetable application: care plan directory is required")
	}
	s.carePlanMu.Lock()
	defer s.carePlanMu.Unlock()
	s.carePlan = directory
}

func (s *Service) carePlanDirectory() (ports.CarePlanDirectory, error) {
	s.carePlanMu.RLock()
	defer s.carePlanMu.RUnlock()
	if s.carePlan == nil {
		return nil, errors.New("timetable application: care plan directory is not bound")
	}
	return s.carePlan, nil
}

func (s *Service) FindCategory(ctx context.Context, id int64) (result domain.Category, err error) {
	err = s.run("find_category", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindCategory(ctx, id, "")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrCategoryNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) FindCategoryForAssignment(ctx context.Context, id int64) (result domain.Category, err error) {
	err = s.run("find_category_for_assignment", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindCategory(ctx, id, "SHARE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrCategoryNotFound
		}
		if value.IsArchived() {
			return domain.ErrCategoryArchived
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) FindCategoryForShare(ctx context.Context, id int64) (result domain.Category, err error) {
	err = s.run("find_category_for_share", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindCategory(ctx, id, "SHARE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrCategoryNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) FindCategoryByName(ctx context.Context, name string) (result domain.Category, err error) {
	err = s.run("find_category_by_name", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindCategoryByName(ctx, name, false, "")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrCategoryNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) FindCategoryByNameForAssignment(ctx context.Context, name string) (result domain.Category, err error) {
	err = s.run("find_category_by_name_for_assignment", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindCategoryByName(ctx, name, true, "SHARE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrCategoryNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListCategories(ctx context.Context) (result []domain.Category, err error) {
	err = s.run("list_categories", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListCategories(ctx)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CountCategoryUsage(ctx context.Context) (result map[int64]int, err error) {
	err = s.run("count_category_usage", func(stats *domain.OperationStats) error {
		counts, queryStats, countErr := s.store.CountCategoryUsage(ctx)
		stats.Add(queryStats)
		result = counts
		return countErr
	})
	return result, err
}

func (s *Service) FindGroup(ctx context.Context, id int64) (result domain.Group, err error) {
	err = s.run("find_group", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindGroup(ctx, id, "")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrGroupNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) FindGroupForUpdate(ctx context.Context, id int64) (result domain.Group, err error) {
	err = s.run("find_group_for_update", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindGroup(ctx, id, "UPDATE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrGroupNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) FindGroupByName(ctx context.Context, name string) (result domain.Group, err error) {
	err = s.run("find_group_by_name", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindGroupByName(ctx, name)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrGroupNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListGroups(ctx context.Context, filter domain.GroupFilter) (result []domain.Group, err error) {
	err = s.run("list_groups", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListGroups(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ListGroupTargets(ctx context.Context, ids []int64) (result map[int64][]domain.GroupTarget, err error) {
	err = s.run("list_group_targets", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListGroupTargets(ctx, ids)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ListTargetStudentIDs(ctx context.Context, ids []int64) (result map[int64][]int64, err error) {
	err = s.run("list_target_student_ids", func(stats *domain.OperationStats) error {
		targets, targetStats, listErr := s.store.ListGroupTargets(ctx, ids)
		stats.Add(targetStats)
		if listErr != nil || len(targets) == 0 {
			result = make(map[int64][]int64, len(ids))
			return listErr
		}
		students, studentStats, studentErr := s.students.ListEnrolledStudents(ctx)
		stats.Add(studentStats)
		if studentErr != nil {
			return studentErr
		}
		result = matchTargetStudents(targets, students, s.today())
		return nil
	})
	return result, err
}

func (s *Service) ReplaceGroupTargets(ctx context.Context, groupID int64, targets []domain.GroupTargetFields) error {
	return s.runWrite(ctx, "replace_group_targets", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.ReplaceGroupTargets(txCtx, groupID, targets)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) CreateGroup(ctx context.Context, fields domain.GroupFields) (result domain.Group, err error) {
	err = s.runWrite(ctx, "create_group", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateGroup(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdateGroup(ctx context.Context, id int64, fields domain.GroupFields) (result domain.Group, err error) {
	err = s.runWrite(ctx, "update_group", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, updated, queryStats, updateErr := s.store.UpdateGroup(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !updated {
			return domain.ErrGroupNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) DeleteGroup(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_group", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteGroup(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) UpdateTemplate(ctx context.Context, id int64, fields domain.TemplateFields) (result int64, err error) {
	err = s.runWrite(ctx, "update_template", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, updateErr := s.store.UpdateTemplate(txCtx, id, fields)
		stats.Add(queryStats)
		result = rows
		return updateErr
	})
	return result, err
}

func (s *Service) ArchiveTemplate(ctx context.Context, id int64) (result int64, err error) {
	err = s.runWrite(ctx, "archive_template", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, archiveErr := s.store.ArchiveTemplate(txCtx, id)
		stats.Add(queryStats)
		result = rows
		return archiveErr
	})
	return result, err
}

func (s *Service) UpdateGroupOfferingSource(ctx context.Context, id int64, fields domain.OfferingSourceFields) error {
	return s.runWrite(ctx, "update_group_offering_source", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.UpdateGroupOfferingSource(txCtx, id, fields)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) CreateCategory(ctx context.Context, fields domain.CategoryFields) (result domain.Category, err error) {
	err = s.runWrite(ctx, "create_category", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateCategory(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdateCategory(ctx context.Context, id int64, fields domain.CategoryFields) (result domain.Category, err error) {
	err = s.runWrite(ctx, "update_category", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		existing, err := s.editableCategory(txCtx, id, stats)
		if err != nil {
			return err
		}
		if existing.IsArchived() {
			return domain.ErrCategoryArchived
		}
		value, updated, queryStats, updateErr := s.store.UpdateCategoryIfActive(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !updated {
			return domain.ErrCategoryArchived
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ArchiveCategory(ctx context.Context, id int64) (result domain.Category, err error) {
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

func (s *Service) RestoreCategory(ctx context.Context, id int64) (result domain.Category, err error) {
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
	return s.runWrite(ctx, "set_category_shift_type_links", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.SetCategoryShiftTypeLinks(txCtx, shiftTypeID, categoryIDs)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) SetCategoryShiftTypeID(ctx context.Context, id int64, shiftTypeID *int64) error {
	return s.runWrite(ctx, "set_category_shift_type_id", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.SetCategoryShiftTypeID(txCtx, id, shiftTypeID)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_category", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteCategory(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) LockStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string) error {
	return s.run("lock_student_enrollments_for_care_exit", func(stats *domain.OperationStats) error {
		queryStats, err := s.store.LockStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) EndStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string) (result domain.CareExitEnrollmentChanges, err error) {
	err = s.runWrite(ctx, "end_student_enrollments_for_care_exit", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		changes, queryStats, endErr := s.store.EndStudentEnrollmentsForCareExit(txCtx, studentIDs, validUntil)
		stats.Add(queryStats)
		result = changes
		return endErr
	})
	return result, err
}

func (s *Service) RestoreStudentEnrollmentsForCareExit(ctx context.Context, studentIDs, periodIDs []int64, removals []domain.CareExitEnrollmentRemoval) (result int, err error) {
	err = s.runWrite(ctx, "restore_student_enrollments_for_care_exit", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, restoreErr := s.store.RestoreStudentEnrollmentsForCareExit(txCtx, studentIDs, periodIDs, removals)
		stats.Add(queryStats)
		result = int(rows)
		return restoreErr
	})
	return result, err
}

func (s *Service) editableCategory(ctx context.Context, id int64, stats *domain.OperationStats) (domain.Category, error) {
	category, found, queryStats, err := s.store.FindCategory(ctx, id, "UPDATE")
	stats.Add(queryStats)
	if err != nil {
		return domain.Category{}, err
	}
	if !found {
		return domain.Category{}, domain.ErrCategoryNotFound
	}
	if category.IsSystem {
		return domain.Category{}, domain.ErrSystemCategoryProtected
	}
	return category, nil
}

func (s *Service) setArchivedAt(ctx context.Context, id int64, value *time.Time, stats *domain.OperationStats, result *domain.Category) error {
	category, updated, queryStats, err := s.store.SetCategoryArchivedAt(ctx, id, value)
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	if !updated {
		return domain.ErrCategoryNotFound
	}
	*result = category
	return nil
}

func (s *Service) run(operation string, callback func(*domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	err = callback(&stats)
	s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	return err
}

func (s *Service) runWrite(ctx context.Context, operation string, retry bool, callback func(context.Context, *domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	err = s.tx.RunWrite(ctx, retry, func(txCtx context.Context) error {
		return callback(txCtx, &stats)
	})
	s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	return err
}
