package activities

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
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
	opGetCategory        = "get category"
	opValidateSupervisor = "validate supervisor"
	opCreateSupervisor   = "create supervisor"
	opValidateSchedule   = "validate schedule"
	opGetGroup           = "get group"
	opFindByCategory     = "find by category"
	opFindGroup          = "find group"
	opGetSchedule        = "get schedule"
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
	// Execute in transaction
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Delete all enrollments
		enrollments, err := txService.(*Service).enrollmentRepo.FindByGroupID(ctx, id)
		if err != nil {
			return err
		}

		for _, enrollment := range enrollments {
			if err := txService.(*Service).enrollmentRepo.Delete(ctx, enrollment.ID); err != nil {
				return err
			}
		}

		// Delete all supervisors
		supervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, id)
		if err != nil {
			return err
		}

		for _, supervisor := range supervisors {
			if err := txService.(*Service).supervisorRepo.Delete(ctx, supervisor.ID); err != nil {
				return err
			}
		}

		// Delete all schedules
		schedules, err := txService.(*Service).scheduleRepo.FindByGroupID(ctx, id)
		if err != nil {
			return err
		}

		for _, schedule := range schedules {
			if err := txService.(*Service).scheduleRepo.Delete(ctx, schedule.ID); err != nil {
				return err
			}
		}

		// Finally delete the group
		return txService.(*Service).groupRepo.Delete(ctx, id)
	})

	if err != nil {
		return &ActivityError{Op: "delete group transaction", Err: err}
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

// ======== Schedule Methods ========

// AddSchedule adds a schedule to an activity group
func (s *Service) AddSchedule(ctx context.Context, groupID int64, schedule *activities.Schedule) (*activities.Schedule, error) {
	// Check if group exists
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, &ActivityError{Op: opFindGroup, Err: err}
	}

	// Set group ID
	schedule.ActivityGroupID = groupID

	// Validate the schedule
	if err := schedule.Validate(); err != nil {
		return nil, &ActivityError{Op: opValidateSchedule, Err: err}
	}

	// Create the schedule
	if err := s.scheduleRepo.Create(ctx, schedule); err != nil {
		return nil, &ActivityError{Op: "create schedule", Err: err}
	}

	return schedule, nil
}

// GetSchedule retrieves a schedule by ID
func (s *Service) GetSchedule(ctx context.Context, id int64) (*activities.Schedule, error) {
	schedule, err := s.scheduleRepo.FindByID(ctx, id)
	if err != nil {
		// Check for "no rows" error and convert to our own error
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: opGetSchedule, Err: ErrScheduleNotFound}
		}
		// Check if the wrapped database error contains sql.ErrNoRows
		if dbErr, ok := err.(*base.DatabaseError); ok && errors.Is(dbErr.Err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: opGetSchedule, Err: ErrScheduleNotFound}
		}
		return nil, &ActivityError{Op: opGetSchedule, Err: err}
	}

	return schedule, nil
}

// GetGroupSchedules retrieves all schedules for a group
func (s *Service) GetGroupSchedules(ctx context.Context, groupID int64) ([]*activities.Schedule, error) {
	schedules, err := s.scheduleRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, &ActivityError{Op: "get group schedules", Err: err}
	}

	return schedules, nil
}

// DeleteSchedule deletes a schedule
func (s *Service) DeleteSchedule(ctx context.Context, id int64) error {
	// Execute in transaction for consistency
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Check if schedule exists
		_, err := txService.(*Service).scheduleRepo.FindByID(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrScheduleNotFound
			}
			return &ActivityError{Op: "find schedule", Err: err}
		}

		// Delete the schedule
		if err := txService.(*Service).scheduleRepo.Delete(ctx, id); err != nil {
			return &ActivityError{Op: "delete schedule", Err: err}
		}

		return nil
	})

	if err != nil {
		return &ActivityError{Op: "delete schedule", Err: err}
	}

	return nil
}

// UpdateSchedule updates an existing schedule
func (s *Service) UpdateSchedule(ctx context.Context, schedule *activities.Schedule) (*activities.Schedule, error) {
	// Execute in transaction for consistency
	var result *activities.Schedule

	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Check if schedule exists
		existingSchedule, err := txService.(*Service).scheduleRepo.FindByID(ctx, schedule.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrScheduleNotFound
			}
			return &ActivityError{Op: "find schedule", Err: err}
		}

		// Validate the schedule
		if err := schedule.Validate(); err != nil {
			return &ActivityError{Op: opValidateSchedule, Err: err}
		}

		// Make sure the relationship to group is preserved
		if schedule.ActivityGroupID != existingSchedule.ActivityGroupID {
			return &ActivityError{Op: "update schedule", Err: errors.New("cannot change activity group for a schedule")}
		}

		// Update the schedule
		if err := txService.(*Service).scheduleRepo.Update(ctx, schedule); err != nil {
			return &ActivityError{Op: "update schedule", Err: err}
		}

		// Get the updated schedule
		updatedSchedule, err := txService.(*Service).scheduleRepo.FindByID(ctx, schedule.ID)
		if err != nil {
			return &ActivityError{Op: "retrieve updated schedule", Err: err}
		}

		// Store result for returning after transaction completes
		result = updatedSchedule
		return nil
	})

	if err != nil {
		return nil, &ActivityError{Op: "update schedule", Err: err}
	}

	return result, nil
}

// ======== Supervisor Methods ========

// AddSupervisor adds a supervisor to an activity group
func (s *Service) AddSupervisor(ctx context.Context, groupID int64, staffID int64, isPrimary bool) (*activities.SupervisorPlanned, error) {
	// Execute everything in a transaction to ensure consistency
	var result *activities.SupervisorPlanned

	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Check if group exists
		_, err := txService.(*Service).groupRepo.FindByID(ctx, groupID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrGroupNotFound
			}
			return &ActivityError{Op: opFindGroup, Err: err}
		}

		// Check if supervisor already exists for this staff in this group
		existingSupervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, groupID)
		if err != nil {
			return &ActivityError{Op: "get existing supervisors", Err: err}
		}

		for _, existing := range existingSupervisors {
			if existing.StaffID == staffID {
				return &ActivityError{Op: "add supervisor", Err: errors.New("supervisor already assigned to this group")}
			}
		}

		// Create supervisor record
		supervisor := &activities.SupervisorPlanned{
			GroupID:   groupID,
			StaffID:   staffID,
			IsPrimary: isPrimary,
		}

		// Validate
		if err := supervisor.Validate(); err != nil {
			return &ActivityError{Op: opValidateSupervisor, Err: err}
		}

		// If this is primary, unset primary flag for all other supervisors
		if isPrimary {
			for _, existing := range existingSupervisors {
				if existing.IsPrimary {
					existing.IsPrimary = false
					if err := txService.(*Service).supervisorRepo.Update(ctx, existing); err != nil {
						return &ActivityError{Op: "update existing supervisor", Err: err}
					}
				}
			}
		}

		// Create the new supervisor
		if err := txService.(*Service).supervisorRepo.Create(ctx, supervisor); err != nil {
			return &ActivityError{Op: opCreateSupervisor, Err: err}
		}

		// Retrieve the created supervisor from DB to get all fields
		createdSupervisor, err := txService.(*Service).supervisorRepo.FindByID(ctx, supervisor.ID)
		if err != nil {
			return &ActivityError{Op: "retrieve created supervisor", Err: err}
		}

		// Store the result for returning after transaction completes
		result = createdSupervisor
		return nil
	})

	if err != nil {
		return nil, &ActivityError{Op: "add supervisor", Err: err}
	}

	return result, nil
}

// GetSupervisor retrieves a supervisor by ID
func (s *Service) GetSupervisor(ctx context.Context, id int64) (*activities.SupervisorPlanned, error) {
	supervisor, err := s.supervisorRepo.FindByID(ctx, id)
	if err != nil {
		// Check for "no rows" error and convert to our own error
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: "get supervisor", Err: ErrSupervisorNotFound}
		}
		// Check if the wrapped database error contains sql.ErrNoRows
		if dbErr, ok := err.(*base.DatabaseError); ok && errors.Is(dbErr.Err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: "get supervisor", Err: ErrSupervisorNotFound}
		}
		return nil, &ActivityError{Op: "get supervisor", Err: err}
	}

	return supervisor, nil
}

// GetGroupSupervisors retrieves all supervisors for a group
func (s *Service) GetGroupSupervisors(ctx context.Context, groupID int64) ([]*activities.SupervisorPlanned, error) {
	supervisors, err := s.supervisorRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, &ActivityError{Op: "get group supervisors", Err: err}
	}

	return supervisors, nil
}

// UpdateSupervisor updates a supervisor's details
func (s *Service) UpdateSupervisor(ctx context.Context, supervisor *activities.SupervisorPlanned) (*activities.SupervisorPlanned, error) {
	// Execute in transaction for consistency
	var result *activities.SupervisorPlanned

	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Check if supervisor exists
		existingSupervisor, err := txService.(*Service).supervisorRepo.FindByID(ctx, supervisor.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrSupervisorNotFound
			}
			return &ActivityError{Op: "find supervisor", Err: err}
		}

		// Validate the supervisor
		if err := supervisor.Validate(); err != nil {
			return &ActivityError{Op: opValidateSupervisor, Err: err}
		}

		// If updating primary status
		if existingSupervisor.IsPrimary != supervisor.IsPrimary {
			if supervisor.IsPrimary {
				// This supervisor is becoming primary, unset other primaries
				otherSupervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, supervisor.GroupID)
				if err != nil {
					return &ActivityError{Op: "find group supervisors", Err: err}
				}

				for _, other := range otherSupervisors {
					if other.ID != supervisor.ID && other.IsPrimary {
						other.IsPrimary = false
						if err := txService.(*Service).supervisorRepo.Update(ctx, other); err != nil {
							return &ActivityError{Op: "update other supervisor", Err: err}
						}
					}
				}
			} else {
				// This supervisor is losing primary status, ensure there's another primary
				return &ActivityError{Op: "update supervisor", Err: errors.New("at least one supervisor must remain primary")}
			}
		}

		// Update the supervisor
		if err := txService.(*Service).supervisorRepo.Update(ctx, supervisor); err != nil {
			return &ActivityError{Op: "update supervisor", Err: err}
		}

		// Get the updated supervisor
		updatedSupervisor, err := txService.(*Service).supervisorRepo.FindByID(ctx, supervisor.ID)
		if err != nil {
			return &ActivityError{Op: "retrieve updated supervisor", Err: err}
		}

		// Store result for returning after transaction completes
		result = updatedSupervisor
		return nil
	})

	if err != nil {
		return nil, &ActivityError{Op: "update supervisor", Err: err}
	}

	return result, nil
}

// GetStaffAssignments gets all supervisor assignments for a staff member
func (s *Service) GetStaffAssignments(ctx context.Context, staffID int64) ([]*activities.SupervisorPlanned, error) {
	// Directly use the repository
	assignments, err := s.supervisorRepo.FindByStaffID(ctx, staffID)
	if err != nil {
		return nil, &ActivityError{Op: "get staff assignments", Err: err}
	}

	return assignments, nil
}

// DeleteSupervisor deletes a supervisor
func (s *Service) DeleteSupervisor(ctx context.Context, id int64) error {
	// Execute in transaction for consistency
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Find the supervisor
		supervisor, err := txService.(*Service).supervisorRepo.FindByID(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrSupervisorNotFound
			}
			return &ActivityError{Op: "find supervisor", Err: err}
		}

		// If this is a primary supervisor, check if there are others
		if supervisor.IsPrimary {
			// Get all supervisors for this group
			allSupervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, supervisor.GroupID)
			if err != nil {
				return &ActivityError{Op: "find group supervisors", Err: err}
			}

			// Count other supervisors
			otherCount := 0
			var newPrimary *activities.SupervisorPlanned
			for _, s := range allSupervisors {
				if s.ID != id {
					otherCount++
					if newPrimary == nil {
						newPrimary = s
					}
				}
			}

			// If no other supervisors exist, cannot delete the primary
			if otherCount == 0 {
				return &ActivityError{Op: "delete supervisor", Err: errors.New("cannot delete the only supervisor for an activity")}
			}

			// Promote another supervisor to primary
			// The database trigger will automatically demote others
			newPrimary.IsPrimary = true
			if err := txService.(*Service).supervisorRepo.Update(ctx, newPrimary); err != nil {
				return &ActivityError{Op: "promote new primary supervisor", Err: err}
			}
		}

		// Delete the supervisor
		if err := txService.(*Service).supervisorRepo.Delete(ctx, id); err != nil {
			return &ActivityError{Op: "delete supervisor record", Err: err}
		}

		return nil
	})

	if err != nil {
		return &ActivityError{Op: "delete supervisor", Err: err}
	}

	return nil
}

// SetPrimarySupervisor sets a supervisor as primary and unsets others
func (s *Service) SetPrimarySupervisor(ctx context.Context, id int64) error {
	// Execute in transaction
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Find the supervisor
		supervisor, err := txService.(*Service).supervisorRepo.FindByID(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrSupervisorNotFound
			}
			return &ActivityError{Op: "find supervisor", Err: err}
		}

		// Find all supervisors for this group
		supervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, supervisor.GroupID)
		if err != nil {
			return &ActivityError{Op: "find group supervisors", Err: err}
		}

		// Update all supervisors
		for _, sup := range supervisors {
			// Only update if primary status is changing
			isPrimaryChanging := (sup.ID == id && !sup.IsPrimary) || (sup.ID != id && sup.IsPrimary)

			if isPrimaryChanging {
				// Set new primary status
				if sup.ID == id {
					sup.IsPrimary = true
				} else {
					sup.IsPrimary = false
				}

				// Update in database
				if err := txService.(*Service).supervisorRepo.Update(ctx, sup); err != nil {
					return &ActivityError{Op: "update supervisor primary status", Err: err}
				}
			}
		}

		return nil
	})

	if err != nil {
		return &ActivityError{Op: "set primary supervisor", Err: err}
	}

	return nil
}

// ======== Enrollment Methods ========

// EnrollStudent enrolls a student in an activity group
func (s *Service) EnrollStudent(ctx context.Context, groupID, studentID int64) error {
	// Execute in transaction to ensure consistency
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Check if group exists
		_, err := txService.(*Service).groupRepo.FindByID(ctx, groupID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrGroupNotFound
			}
			return &ActivityError{Op: opFindGroup, Err: err}
		}

		// Check if student is already enrolled
		enrollments, err := txService.(*Service).enrollmentRepo.FindByGroupID(ctx, groupID)
		if err != nil {
			return &ActivityError{Op: "check existing enrollment", Err: err}
		}

		// Check if student is already enrolled
		for _, enrollment := range enrollments {
			if enrollment.StudentID == studentID {
				return ErrStudentAlreadyEnrolled
			}
		}

		// Create enrollment
		enrollment := &activities.StudentEnrollment{
			StudentID:       studentID,
			ActivityGroupID: groupID,
			EnrollmentDate:  time.Now(),
		}

		if err := txService.(*Service).enrollmentRepo.Create(ctx, enrollment); err != nil {
			return &ActivityError{Op: "create enrollment", Err: err}
		}

		return nil
	})

	if err != nil {
		return &ActivityError{Op: "enroll student", Err: err}
	}

	return nil
}

// UnenrollStudent removes a student from an activity group
func (s *Service) UnenrollStudent(ctx context.Context, groupID, studentID int64) error {
	// Execute in transaction to ensure consistency
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Verify group exists
		_, err := txService.(*Service).groupRepo.FindByID(ctx, groupID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrGroupNotFound
			}
			return &ActivityError{Op: opFindGroup, Err: err}
		}

		// Find the enrollment
		enrollments, err := txService.(*Service).enrollmentRepo.FindByGroupID(ctx, groupID)
		if err != nil {
			return &ActivityError{Op: "find enrollments", Err: err}
		}

		// Look for the specific enrollment
		var found bool
		var enrollmentID int64
		for _, enrollment := range enrollments {
			if enrollment.StudentID == studentID {
				enrollmentID = enrollment.ID
				found = true
				break
			}
		}

		if !found {
			return ErrNotEnrolled
		}

		// Delete the enrollment
		if err := txService.(*Service).enrollmentRepo.Delete(ctx, enrollmentID); err != nil {
			return &ActivityError{Op: "delete enrollment", Err: err}
		}

		return nil
	})

	if err != nil {
		return &ActivityError{Op: "unenroll student", Err: err}
	}

	return nil
}

// UpdateGroupEnrollments updates the student enrollments for a group
// This follows the education.UpdateGroupTeachers pattern but for student enrollments
func (s *Service) UpdateGroupEnrollments(ctx context.Context, groupID int64, studentIDs []int64) error {
	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "UpdateGroupEnrollments", Err: ErrGroupNotFound}
	}

	// Execute in transaction
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Get current enrollments
		enrollments, err := txService.(*Service).enrollmentRepo.FindByGroupID(ctx, groupID)
		if err != nil {
			return &ActivityError{Op: "get current enrollments", Err: err}
		}

		// Create maps for easier comparison
		currentStudentIDs := make(map[int64]int64) // studentID -> enrollmentID
		for _, enrollment := range enrollments {
			currentStudentIDs[enrollment.StudentID] = enrollment.ID
		}

		newStudentIDs := make(map[int64]bool)
		for _, studentID := range studentIDs {
			newStudentIDs[studentID] = true
		}

		// Find students to remove (in current but not in new)
		for studentID, enrollmentID := range currentStudentIDs {
			if !newStudentIDs[studentID] {
				if err := txService.(*Service).enrollmentRepo.Delete(ctx, enrollmentID); err != nil {
					return &ActivityError{Op: "delete enrollment", Err: err}
				}
			}
		}

		// Find students to add (in new but not in current)
		for _, studentID := range studentIDs {
			if _, exists := currentStudentIDs[studentID]; !exists {
				// Create the enrollment
				enrollment := &activities.StudentEnrollment{
					StudentID:       studentID,
					ActivityGroupID: groupID,
					EnrollmentDate:  time.Now(),
				}

				if err := txService.(*Service).enrollmentRepo.Create(ctx, enrollment); err != nil {
					return &ActivityError{Op: "create enrollment", Err: err}
				}
			}
		}

		return nil
	})

	if err != nil {
		return &ActivityError{Op: "update group enrollments", Err: err}
	}

	return nil
}

// UpdateGroupSupervisors updates the supervisors for a group
// This follows the UpdateGroupEnrollments pattern but for supervisors
func (s *Service) UpdateGroupSupervisors(ctx context.Context, groupID int64, staffIDs []int64) error {
	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "UpdateGroupSupervisors", Err: ErrGroupNotFound}
	}

	// Execute in transaction
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Get current supervisors
		supervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, groupID)
		if err != nil {
			return &ActivityError{Op: "get current supervisors", Err: err}
		}

		// Create maps for easier comparison
		currentStaffIDs := make(map[int64]int64) // staffID -> supervisorID
		for _, supervisor := range supervisors {
			currentStaffIDs[supervisor.StaffID] = supervisor.ID
		}

		newStaffIDs := make(map[int64]bool)
		for _, staffID := range staffIDs {
			newStaffIDs[staffID] = true
		}

		// Find supervisors to remove (in current but not in new)
		for staffID, supervisorID := range currentStaffIDs {
			if !newStaffIDs[staffID] {
				// Special handling: if this is the only supervisor and we're trying to remove it,
				// we need to ensure at least one new supervisor is being added
				if len(currentStaffIDs) == 1 && len(staffIDs) == 0 {
					return &ActivityError{Op: "update supervisors", Err: errors.New("cannot remove all supervisors from an activity")}
				}

				// If we're removing a supervisor and there will be others remaining, safe to delete
				if err := txService.(*Service).supervisorRepo.Delete(ctx, supervisorID); err != nil {
					return &ActivityError{Op: "delete supervisor", Err: err}
				}
			}
		}

		// Find supervisors to add (in new but not in current)
		for i, staffID := range staffIDs {
			if _, exists := currentStaffIDs[staffID]; !exists {
				// First new supervisor becomes primary if no supervisors will remain
				isPrimary := i == 0 && len(newStaffIDs) == len(staffIDs)

				// Create the supervisor
				supervisor := &activities.SupervisorPlanned{
					GroupID:   groupID,
					StaffID:   staffID,
					IsPrimary: isPrimary,
				}

				if err := txService.(*Service).supervisorRepo.Create(ctx, supervisor); err != nil {
					return &ActivityError{Op: opCreateSupervisor, Err: err}
				}
			}
		}

		// Ensure the first supervisor in the list is primary
		if len(staffIDs) > 0 {
			// Get all supervisors again to update primary status
			updatedSupervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, groupID)
			if err != nil {
				return &ActivityError{Op: "get updated supervisors", Err: err}
			}

			// Find the supervisor for the first staffID and make it primary
			for _, supervisor := range updatedSupervisors {
				if supervisor.StaffID == staffIDs[0] {
					if !supervisor.IsPrimary {
						supervisor.IsPrimary = true
						if err := txService.(*Service).supervisorRepo.Update(ctx, supervisor); err != nil {
							return &ActivityError{Op: "set primary supervisor", Err: err}
						}
					}
				} else if supervisor.IsPrimary {
					// Remove primary status from others
					supervisor.IsPrimary = false
					if err := txService.(*Service).supervisorRepo.Update(ctx, supervisor); err != nil {
						return &ActivityError{Op: "remove primary status", Err: err}
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return &ActivityError{Op: "update group supervisors", Err: err}
	}

	return nil
}

// GetEnrolledStudents retrieves all students enrolled in a group
func (s *Service) GetEnrolledStudents(ctx context.Context, groupID int64) ([]*users.Student, error) {
	// Get the enrollments for this group
	enrollments, err := s.enrollmentRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, &ActivityError{Op: "get enrolled students", Err: err}
	}

	// Extract the Student objects from the enrollments
	students := make([]*users.Student, 0, len(enrollments))
	for _, enrollment := range enrollments {
		// Check if the Student relation is loaded
		if enrollment.Student != nil {
			students = append(students, enrollment.Student)
		}
	}

	return students, nil
}

// GetStudentEnrollments retrieves all groups a student is enrolled in
func (s *Service) GetStudentEnrollments(ctx context.Context, studentID int64) ([]*activities.Group, error) {
	var result []*activities.Group

	// Use transaction to ensure consistent data
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Get all enrollments for this student
		enrollments, err := txService.(*Service).enrollmentRepo.FindByStudentID(ctx, studentID)
		if err != nil {
			return &ActivityError{Op: "find student enrollments", Err: err}
		}

		// Extract group IDs from enrollments
		groupIDs := make([]int64, 0, len(enrollments))
		for _, enrollment := range enrollments {
			groupIDs = append(groupIDs, enrollment.ActivityGroupID)
		}

		// If no enrollments found, return empty slice
		if len(groupIDs) == 0 {
			result = []*activities.Group{}
			return nil
		}

		// Create a filter to get groups by IDs
		options := base.NewQueryOptions()
		filter := base.NewFilter()

		// Convert int64 slice to []interface{}
		interfaceIDs := make([]interface{}, len(groupIDs))
		for i, id := range groupIDs {
			interfaceIDs[i] = id
		}

		filter.In("id", interfaceIDs...)
		options.Filter = filter

		// Get groups using List method
		groups, err := txService.(*Service).groupRepo.List(ctx, options)
		if err != nil {
			return &ActivityError{Op: "get groups by ids", Err: err}
		}

		// Store result for returning after transaction completes
		result = groups
		return nil
	})

	if err != nil {
		return nil, &ActivityError{Op: "get student enrollments", Err: err}
	}

	return result, nil
}

// GetAvailableGroups retrieves all groups a student can enroll in (not already enrolled)
func (s *Service) GetAvailableGroups(ctx context.Context, studentID int64) ([]*activities.Group, error) {
	// Get all active groups - assuming FindOpenGroups is the correct method
	allGroups, err := s.groupRepo.FindOpenGroups(ctx)
	if err != nil {
		return nil, &ActivityError{Op: "get all groups", Err: err}
	}

	// Get enrollments for this student
	enrollments, err := s.enrollmentRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ActivityError{Op: "get student enrollments", Err: err}
	}

	// Create a map of enrolled group IDs for quick lookup
	enrolledMap := make(map[int64]bool)
	for _, enrollment := range enrollments {
		enrolledMap[enrollment.ActivityGroupID] = true
	}

	// Filter out already enrolled groups
	var availableGroups []*activities.Group
	for _, group := range allGroups {
		if !enrolledMap[group.ID] {
			availableGroups = append(availableGroups, group)
		}
	}

	return availableGroups, nil
}

// UpdateAttendanceStatus updates the attendance status for an enrollment
func (s *Service) UpdateAttendanceStatus(ctx context.Context, enrollmentID int64, status *string) error {
	// Execute in transaction for consistency
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Check if enrollment exists
		_, err := txService.(*Service).enrollmentRepo.FindByID(ctx, enrollmentID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrEnrollmentNotFound
			}
			return &ActivityError{Op: "find enrollment", Err: err}
		}

		// Update the status
		if err := txService.(*Service).enrollmentRepo.UpdateAttendanceStatus(ctx, enrollmentID, status); err != nil {
			return &ActivityError{Op: "update attendance status", Err: err}
		}

		return nil
	})

	if err != nil {
		return &ActivityError{Op: "update attendance status", Err: err}
	}

	return nil
}

// GetEnrollmentsByDate gets all enrollments for a specific date
func (s *Service) GetEnrollmentsByDate(ctx context.Context, date time.Time) ([]*activities.StudentEnrollment, error) {
	// NOTE: This is a placeholder implementation
	// The actual implementation would likely query enrollments with schedules active on the given date
	// and possibly filter for attendance status

	// This would require a custom repository method with SQL join between enrollments and schedules
	// For now, we just use the date range finder with the date as both start and end

	// Using the repository's FindByEnrollmentDateRange method
	enrollments, err := s.enrollmentRepo.FindByEnrollmentDateRange(ctx, date, date)
	if err != nil {
		return nil, &ActivityError{Op: "get enrollments by date", Err: err}
	}

	return enrollments, nil
}

// GetEnrollmentHistory gets a student's enrollment history within a date range
func (s *Service) GetEnrollmentHistory(ctx context.Context, studentID int64, startDate, endDate time.Time) ([]*activities.StudentEnrollment, error) {
	// NOTE: This is a placeholder implementation
	// The actual implementation would query enrollments with schedules active in the given date range

	// Get all enrollments for the student
	enrollments, err := s.enrollmentRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ActivityError{Op: "get enrollment history", Err: err}
	}

	// Filter by date range (simplified logic, actual implementation would be more complex)
	var filteredEnrollments []*activities.StudentEnrollment
	for _, enrollment := range enrollments {
		if !enrollment.EnrollmentDate.Before(startDate) && !enrollment.EnrollmentDate.After(endDate) {
			filteredEnrollments = append(filteredEnrollments, enrollment)
		}
	}

	return filteredEnrollments, nil
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
