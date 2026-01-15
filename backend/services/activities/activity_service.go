package activities

import (
	"context"
	"database/sql"
	"errors"
	"log"

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
)

// WithTx returns a new service that uses the provided transaction
func (s *Service) WithTx(tx bun.Tx) any {
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

// UpdateGroup updates an activity group
func (s *Service) UpdateGroup(ctx context.Context, group *activities.Group) (*activities.Group, error) {
	if err := group.Validate(); err != nil {
		return nil, &ActivityError{Op: "validate group", Err: err}
	}

	if err := s.groupRepo.Update(ctx, group); err != nil {
		return nil, &ActivityError{Op: "update group", Err: err}
	}

	return group, nil
}

// DeleteGroup deletes an activity group and all related records
func (s *Service) DeleteGroup(ctx context.Context, id int64) error {
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
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
			log.Printf("Warning: Failed to load category for group ID %d: %v", id, err)
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
		log.Printf("Warning: Failed to load supervisors for group ID %d: %v", id, supervisorErr)
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

// ======== Public Methods ========

// GetPublicGroups retrieves groups for public display, optionally filtered by category
func (s *Service) GetPublicGroups(ctx context.Context, categoryID *int64) ([]*activities.Group, map[int64]int, error) {
	var groups []*activities.Group
	var err error

	// Retrieve groups by category or all public groups
	if categoryID != nil {
		groups, err = s.groupRepo.FindByCategory(ctx, *categoryID)
	} else {
		groups, err = s.groupRepo.FindOpenGroups(ctx)
	}

	if err != nil {
		return nil, nil, &ActivityError{Op: "get public groups", Err: err}
	}

	// Get enrollment counts
	counts := make(map[int64]int)
	for _, group := range groups {
		count, err := s.enrollmentRepo.CountByGroupID(ctx, group.ID)
		if err != nil {
			return nil, nil, &ActivityError{Op: "get enrollments", Err: err}
		}
		counts[group.ID] = count
	}

	return groups, counts, nil
}

// GetPublicCategories retrieves categories for public display
func (s *Service) GetPublicCategories(ctx context.Context) ([]*activities.Category, error) {
	categories, err := s.categoryRepo.ListAll(ctx)
	if err != nil {
		return nil, &ActivityError{Op: "get public categories", Err: err}
	}

	return categories, nil
}

// GetOpenGroups retrieves activity groups that are open for enrollment
func (s *Service) GetOpenGroups(ctx context.Context) ([]*activities.Group, error) {
	// This is likely a wrapper around a repository method
	groups, err := s.groupRepo.FindOpenGroups(ctx)
	if err != nil {
		return nil, &ActivityError{Op: "get open groups", Err: err}
	}

	return groups, nil
}

// GetTeacherTodaysActivities returns activities available for teacher selection on devices
func (s *Service) GetTeacherTodaysActivities(ctx context.Context, staffID int64) ([]*activities.Group, error) {
	groups, err := s.groupRepo.FindByStaffSupervisorToday(ctx, staffID)
	if err != nil {
		return nil, &ActivityError{Op: "get teacher today activities", Err: err}
	}
	return groups, nil
}
