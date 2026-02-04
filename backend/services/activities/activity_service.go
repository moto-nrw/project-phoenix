package activities

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Service implements the ActivityService interface
type Service struct {
	categoryRepo   activities.CategoryRepository
	groupRepo      activities.GroupRepository
	scheduleRepo   activities.ScheduleRepository
	supervisorRepo activities.SupervisorPlannedRepository
	enrollmentRepo activities.StudentEnrollmentRepository
	db             *bun.DB
	txHandler      *base.TxHandler
}

// NewService creates a new activity service
func NewService(
	categoryRepo activities.CategoryRepository,
	groupRepo activities.GroupRepository,
	scheduleRepo activities.ScheduleRepository,
	supervisorRepo activities.SupervisorPlannedRepository,
	enrollmentRepo activities.StudentEnrollmentRepository,
	db *bun.DB) (*Service, error) {
	return &Service{
		categoryRepo:   categoryRepo,
		groupRepo:      groupRepo,
		scheduleRepo:   scheduleRepo,
		supervisorRepo: supervisorRepo,
		enrollmentRepo: enrollmentRepo,
		db:             db,
		txHandler:      base.NewTxHandler(db),
	}, nil
}

// Operation names for error context
const (
	opGetCategory          = "get category"
	opValidateSupervisor   = "validate supervisor"
	opCreateSupervisor     = "create supervisor"
	opValidateSchedule     = "validate schedule"
	opGetGroup             = "get group"
	opFindByCategory       = "find by category"
	opFindGroup            = "find group"
	opGetSchedule          = "get schedule"
	opUpdateSchedule       = "update schedule"
	opGetSupervisor        = "get supervisor"
	opFindSupervisor       = "find supervisor"
	opFindGroupSupervisors = "find group supervisors"
	opUpdateSupervisor     = "update supervisor"
	opDeleteSupervisor     = "delete supervisor"
	opCheckPermissions     = "check permissions"
)

// WithTx returns a new service that uses the provided transaction
func (s *Service) WithTx(tx bun.Tx) interface{} {
	// Get repositories with transaction if they implement the TransactionalRepository interface
	var categoryRepo = s.categoryRepo
	var groupRepo = s.groupRepo
	var scheduleRepo = s.scheduleRepo
	var supervisorRepo = s.supervisorRepo
	var enrollmentRepo = s.enrollmentRepo

	// Try to cast repositories to TransactionalRepository and apply the transaction
	if txRepo, ok := s.categoryRepo.(base.TransactionalRepository); ok {
		categoryRepo = txRepo.WithTx(tx).(activities.CategoryRepository)
	}
	if txRepo, ok := s.groupRepo.(base.TransactionalRepository); ok {
		groupRepo = txRepo.WithTx(tx).(activities.GroupRepository)
	}
	if txRepo, ok := s.scheduleRepo.(base.TransactionalRepository); ok {
		scheduleRepo = txRepo.WithTx(tx).(activities.ScheduleRepository)
	}
	if txRepo, ok := s.supervisorRepo.(base.TransactionalRepository); ok {
		supervisorRepo = txRepo.WithTx(tx).(activities.SupervisorPlannedRepository)
	}
	if txRepo, ok := s.enrollmentRepo.(base.TransactionalRepository); ok {
		enrollmentRepo = txRepo.WithTx(tx).(activities.StudentEnrollmentRepository)
	}

	// Return a new service with the transaction
	return &Service{
		categoryRepo:   categoryRepo,
		groupRepo:      groupRepo,
		scheduleRepo:   scheduleRepo,
		supervisorRepo: supervisorRepo,
		enrollmentRepo: enrollmentRepo,
		db:             s.db,
		txHandler:      s.txHandler.WithTx(tx),
	}
}

// ======== Category Methods ========

// CreateCategory creates a new activity category
func (s *Service) CreateCategory(ctx context.Context, category *activities.Category) (*activities.Category, error) {
	if err := category.Validate(); err != nil {
		return nil, &ActivityError{Op: "create category", Err: err}
	}

	// Use queryOptions with ModelTableExpr for schema qualification
	if err := s.categoryRepo.Create(ctx, category); err != nil {
		return nil, &ActivityError{Op: "create category", Err: err}
	}

	return category, nil
}

// GetCategory retrieves a category by ID
func (s *Service) GetCategory(ctx context.Context, id int64) (*activities.Category, error) {
	category, err := s.categoryRepo.FindByID(ctx, id)
	if err != nil {
		// Check for "no rows" error and convert to our own error
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: opGetCategory, Err: ErrCategoryNotFound}
		}
		// Check if the wrapped database error contains sql.ErrNoRows
		if dbErr, ok := err.(*base.DatabaseError); ok && errors.Is(dbErr.Err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: opGetCategory, Err: ErrCategoryNotFound}
		}
		return nil, &ActivityError{Op: opGetCategory, Err: err}
	}

	return category, nil
}

// UpdateCategory updates an activity category
func (s *Service) UpdateCategory(ctx context.Context, category *activities.Category) (*activities.Category, error) {
	if err := category.Validate(); err != nil {
		return nil, &ActivityError{Op: "update category", Err: err}
	}

	if err := s.categoryRepo.Update(ctx, category); err != nil {
		return nil, &ActivityError{Op: "update category", Err: err}
	}

	return category, nil
}

// DeleteCategory deletes a category
func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	// Check if the category is in use by any group
	groupsWithCategory, err := s.groupRepo.FindByCategory(ctx, id)
	if err != nil {
		return &ActivityError{Op: "check category usage", Err: err}
	}

	if len(groupsWithCategory) > 0 {
		return &ActivityError{Op: "delete category", Err: errors.New("category is in use by one or more activity groups")}
	}

	if err := s.categoryRepo.Delete(ctx, id); err != nil {
		return &ActivityError{Op: "delete category", Err: err}
	}

	return nil
}

// ListCategories lists all activity categories
func (s *Service) ListCategories(ctx context.Context) ([]*activities.Category, error) {
	categories, err := s.categoryRepo.ListAll(ctx)
	if err != nil {
		return nil, &ActivityError{Op: "list categories", Err: err}
	}

	return categories, nil
}

// ======== Activity Group Methods ========

// CreateGroup creates a new activity group with supervisors and schedules
func (s *Service) CreateGroup(ctx context.Context, group *activities.Group, supervisorIDs []int64, schedules []*activities.Schedule) (*activities.Group, error) {
	if err := group.Validate(); err != nil {
		return nil, &ActivityError{Op: "validate group", Err: err}
	}

	if err := s.validateAndSetCategory(ctx, group); err != nil {
		return nil, err
	}

	var result *activities.Group
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(ActivityService)

		if err := txService.(*Service).groupRepo.Create(ctx, group); err != nil {
			return &ActivityError{Op: "create group", Err: err}
		}

		if err := s.createSupervisorsInTx(ctx, txService, group.ID, supervisorIDs); err != nil {
			return err
		}

		if err := s.createSchedulesInTx(ctx, txService, group.ID, schedules); err != nil {
			return err
		}

		var err error
		result, err = txService.(*Service).groupRepo.FindByID(ctx, group.ID)
		if err != nil {
			return &ActivityError{Op: "retrieve created group", Err: err}
		}

		return nil
	})

	if err != nil {
		return nil, &ActivityError{Op: "create group", Err: err}
	}

	return result, nil
}

// validateAndSetCategory validates and sets the category if provided
func (s *Service) validateAndSetCategory(ctx context.Context, group *activities.Group) error {
	if group.CategoryID <= 0 {
		return nil
	}

	category, err := s.categoryRepo.FindByID(ctx, group.CategoryID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &ActivityError{Op: "validate category", Err: ErrCategoryNotFound}
		}
		return &ActivityError{Op: "validate category", Err: err}
	}

	group.Category = category
	return nil
}

// createSupervisorsInTx creates supervisors for a group within a transaction
func (s *Service) createSupervisorsInTx(ctx context.Context, txService ActivityService, groupID int64, supervisorIDs []int64) error {
	for i, staffID := range supervisorIDs {
		supervisor := &activities.SupervisorPlanned{
			StaffID:   staffID,
			GroupID:   groupID,
			IsPrimary: i == 0, // First supervisor is primary
		}

		if err := supervisor.Validate(); err != nil {
			return &ActivityError{Op: opValidateSupervisor, Err: err}
		}

		if err := txService.(*Service).supervisorRepo.Create(ctx, supervisor); err != nil {
			return &ActivityError{Op: opCreateSupervisor, Err: err}
		}
	}
	return nil
}

// createSchedulesInTx creates schedules for a group within a transaction
func (s *Service) createSchedulesInTx(ctx context.Context, txService ActivityService, groupID int64, schedules []*activities.Schedule) error {
	for _, schedule := range schedules {
		schedule.ActivityGroupID = groupID

		if err := schedule.Validate(); err != nil {
			return &ActivityError{Op: opValidateSchedule, Err: err}
		}

		if err := txService.(*Service).scheduleRepo.Create(ctx, schedule); err != nil {
			return &ActivityError{Op: "create schedule", Err: err}
		}
	}
	return nil
}

// GetGroup retrieves an activity group by ID
func (s *Service) GetGroup(ctx context.Context, id int64) (*activities.Group, error) {
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		// Check for "no rows" error and convert to our own error
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: opGetGroup, Err: ErrGroupNotFound}
		}
		// Check if the wrapped database error contains sql.ErrNoRows
		if dbErr, ok := err.(*base.DatabaseError); ok && errors.Is(dbErr.Err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: opGetGroup, Err: ErrGroupNotFound}
		}
		return nil, &ActivityError{Op: opGetGroup, Err: err}
	}

	return group, nil
}

// UpdateGroup updates an activity group with ownership verification
// Only the creator, supervisors, or users with manage permission can update
func (s *Service) UpdateGroup(ctx context.Context, group *activities.Group, requestingStaffID int64, hasManagePermission bool) (*activities.Group, error) {
	if err := group.Validate(); err != nil {
		return nil, &ActivityError{Op: "validate group", Err: err}
	}

	// Check if user can modify this activity
	canModify, err := s.CanModifyActivity(ctx, group.ID, requestingStaffID, hasManagePermission)
	if err != nil {
		return nil, &ActivityError{Op: opCheckPermissions, Err: err}
	}
	if !canModify {
		return nil, &ActivityError{Op: "update group", Err: ErrNotOwner}
	}

	if err := s.groupRepo.Update(ctx, group); err != nil {
		return nil, &ActivityError{Op: "update group", Err: err}
	}

	return group, nil
}

// DeleteGroup deletes an activity group and all related records with ownership verification
// Only the creator, supervisors, or users with manage permission can delete
func (s *Service) DeleteGroup(ctx context.Context, id int64, requestingStaffID int64, hasManagePermission bool) error {
	// Check if user can modify this activity before starting transaction
	canModify, err := s.CanModifyActivity(ctx, id, requestingStaffID, hasManagePermission)
	if err != nil {
		return &ActivityError{Op: opCheckPermissions, Err: err}
	}
	if !canModify {
		return &ActivityError{Op: "delete group", Err: ErrNotOwner}
	}

	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*Service)

		// Delete all related records
		if err := deleteGroupEnrollments(ctx, txService, id); err != nil {
			return err
		}

		if err := deleteGroupSupervisors(ctx, txService, id); err != nil {
			return err
		}

		if err := deleteGroupSchedules(ctx, txService, id); err != nil {
			return err
		}

		// Finally delete the group
		return txService.groupRepo.Delete(ctx, id)
	})

	if err != nil {
		return &ActivityError{Op: "delete group transaction", Err: err}
	}

	return nil
}

// deleteGroupEnrollments deletes all enrollments for a group
func deleteGroupEnrollments(ctx context.Context, service *Service, groupID int64) error {
	enrollments, err := service.enrollmentRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return err
	}

	for _, enrollment := range enrollments {
		if err := service.enrollmentRepo.Delete(ctx, enrollment.ID); err != nil {
			return err
		}
	}

	return nil
}

// deleteGroupSupervisors deletes all supervisors for a group
func deleteGroupSupervisors(ctx context.Context, service *Service, groupID int64) error {
	supervisors, err := service.supervisorRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return err
	}

	for _, supervisor := range supervisors {
		if err := service.supervisorRepo.Delete(ctx, supervisor.ID); err != nil {
			return err
		}
	}

	return nil
}

// deleteGroupSchedules deletes all schedules for a group
func deleteGroupSchedules(ctx context.Context, service *Service, groupID int64) error {
	schedules, err := service.scheduleRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return err
	}

	for _, schedule := range schedules {
		if err := service.scheduleRepo.Delete(ctx, schedule.ID); err != nil {
			return err
		}
	}

	return nil
}

// ListGroups lists activity groups with optional filters
func (s *Service) ListGroups(ctx context.Context, queryOptions *base.QueryOptions) ([]*activities.Group, error) {
	// Use the repository's List method instead since ListWithOptions is not defined
	groups, err := s.groupRepo.List(ctx, queryOptions)
	if err != nil {
		return nil, &ActivityError{Op: "list groups", Err: err}
	}

	return groups, nil
}

// FindByCategory finds all activity groups in a specific category
func (s *Service) FindByCategory(ctx context.Context, categoryID int64) ([]*activities.Group, error) {
	// First verify the category exists
	_, err := s.categoryRepo.FindByID(ctx, categoryID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: opFindByCategory, Err: ErrCategoryNotFound}
		}
		return nil, &ActivityError{Op: opFindByCategory, Err: err}
	}

	// Use the repository method
	groups, err := s.groupRepo.FindByCategory(ctx, categoryID)
	if err != nil {
		return nil, &ActivityError{Op: opFindByCategory, Err: err}
	}

	return groups, nil
}

// GetGroupWithDetails retrieves a group with its supervisors and schedules
func (s *Service) GetGroupWithDetails(ctx context.Context, id int64) (*activities.Group, []*activities.SupervisorPlanned, []*activities.Schedule, error) {
	// Get the group
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		// Check for "no rows" error and convert to our own error
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, nil, &ActivityError{Op: opGetGroup, Err: ErrGroupNotFound}
		}
		// Check if the wrapped database error contains sql.ErrNoRows
		if dbErr, ok := err.(*base.DatabaseError); ok && errors.Is(dbErr.Err, sql.ErrNoRows) {
			return nil, nil, nil, &ActivityError{Op: opGetGroup, Err: ErrGroupNotFound}
		}
		return nil, nil, nil, &ActivityError{Op: opGetGroup, Err: err}
	}

	// Load the category if not already loaded
	if group.Category == nil && group.CategoryID > 0 {
		category, err := s.categoryRepo.FindByID(ctx, group.CategoryID)
		if err != nil {
			slog.Default().WarnContext(ctx, "failed to load category for group",
				slog.Int64("group_id", id),
				slog.String("error", err.Error()))
		} else {
			group.Category = category
		}
	}

	// Get supervisors - handle errors gracefully
	var supervisors []*activities.SupervisorPlanned
	var supervisorErr error
	supervisors, supervisorErr = s.supervisorRepo.FindByGroupID(ctx, id)
	if supervisorErr != nil {
		// Log the error but continue - we'll return an error at the end
		// so the caller can decide whether to use the partial data
		slog.Default().WarnContext(ctx, "failed to load supervisors for group",
			slog.Int64("group_id", id),
			slog.String("error", supervisorErr.Error()))
	}

	// Get schedules
	schedules, err := s.scheduleRepo.FindByGroupID(ctx, id)
	if err != nil {
		return nil, nil, nil, &ActivityError{Op: "get schedules", Err: err}
	}

	// If we had a supervisor error, return it after loading everything else
	if supervisorErr != nil {
		return group, nil, schedules, &ActivityError{Op: "get supervisors", Err: supervisorErr}
	}

	return group, supervisors, schedules, nil
}

// GetGroupsWithEnrollmentCounts returns groups with their enrollment counts
func (s *Service) GetGroupsWithEnrollmentCounts(ctx context.Context) ([]*activities.Group, map[int64]int, error) {
	// Use the repository method that does this
	return s.groupRepo.FindWithEnrollmentCounts(ctx)
}

// CanModifyActivity checks if a user can modify (edit/delete) an activity
// Returns true if:
// 1. User has manage permission (admin)
// 2. User created the activity (group.CreatedBy == staffID)
// 3. User is a supervisor of the activity
func (s *Service) CanModifyActivity(ctx context.Context, groupID int64, staffID int64, hasManagePermission bool) (bool, error) {
	// Admins with manage permission can always modify
	if hasManagePermission {
		return true, nil
	}

	// Get the group with supervisors
	group, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, &ActivityError{Op: opCheckPermissions, Err: ErrGroupNotFound}
		}
		return false, &ActivityError{Op: opCheckPermissions, Err: err}
	}

	// Check if user is the creator
	if group.CreatedBy == staffID {
		return true, nil
	}

	// Check if user is a supervisor
	supervisors, err := s.supervisorRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		slog.Default().WarnContext(ctx, "failed to load supervisors for permission check",
			slog.String("error", err.Error()))
		// Continue without supervisor check if we can't load them
	} else {
		for _, supervisor := range supervisors {
			if supervisor != nil && supervisor.StaffID == staffID {
				return true, nil
			}
		}
	}

	return false, nil
}

// validateGroupExists checks if a group exists
func (s *Service) validateGroupExists(ctx context.Context, txService ActivityService, groupID int64) error {
	_, err := txService.(*Service).groupRepo.FindByID(ctx, groupID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrGroupNotFound
		}
		return &ActivityError{Op: opFindGroup, Err: err}
	}
	return nil
}
