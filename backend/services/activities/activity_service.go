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
			return &ActivityError{Op: opUpdateSchedule, Err: errors.New("cannot change activity group for a schedule")}
		}

		// Update the schedule
		if err := txService.(*Service).scheduleRepo.Update(ctx, schedule); err != nil {
			return &ActivityError{Op: opUpdateSchedule, Err: err}
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
		return nil, &ActivityError{Op: opUpdateSchedule, Err: err}
	}

	return result, nil
}

// ======== Supervisor Methods ========

// AddSupervisor adds a supervisor to an activity group
func (s *Service) AddSupervisor(ctx context.Context, groupID int64, staffID int64, isPrimary bool) (*activities.SupervisorPlanned, error) {
	var result *activities.SupervisorPlanned

	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(ActivityService)

		supervisor, err := s.addSupervisorInTx(ctx, txService, groupID, staffID, isPrimary)
		if err != nil {
			return err
		}

		result = supervisor
		return nil
	})

	if err != nil {
		return nil, &ActivityError{Op: "add supervisor", Err: err}
	}

	return result, nil
}

// addSupervisorInTx contains the transaction logic for adding a supervisor
func (s *Service) addSupervisorInTx(ctx context.Context, txService ActivityService, groupID, staffID int64, isPrimary bool) (*activities.SupervisorPlanned, error) {
	if err := s.validateGroupExists(ctx, txService, groupID); err != nil {
		return nil, err
	}

	if err := s.validateStaffExists(ctx, staffID); err != nil {
		return nil, err
	}

	existingSupervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, &ActivityError{Op: "get existing supervisors", Err: err}
	}

	if err := s.checkSupervisorNotDuplicate(staffID, existingSupervisors); err != nil {
		return nil, err
	}

	supervisor := &activities.SupervisorPlanned{
		GroupID:   groupID,
		StaffID:   staffID,
		IsPrimary: isPrimary,
	}

	if err := supervisor.Validate(); err != nil {
		return nil, &ActivityError{Op: opValidateSupervisor, Err: err}
	}

	if err := s.unsetPrimarySupervisorsInTx(ctx, txService, isPrimary, existingSupervisors); err != nil {
		return nil, err
	}

	if err := txService.(*Service).supervisorRepo.Create(ctx, supervisor); err != nil {
		return nil, &ActivityError{Op: opCreateSupervisor, Err: err}
	}

	createdSupervisor, err := txService.(*Service).supervisorRepo.FindByID(ctx, supervisor.ID)
	if err != nil {
		return nil, &ActivityError{Op: "retrieve created supervisor", Err: err}
	}

	return createdSupervisor, nil
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

// validateStaffExists checks if a staff member exists before creating supervisor
func (s *Service) validateStaffExists(ctx context.Context, staffID int64) error {
	exists, err := s.db.NewSelect().
		TableExpr("users.staff").
		Where("id = ?", staffID).
		Exists(ctx)
	if err != nil {
		return &ActivityError{Op: "validate staff", Err: err}
	}
	if !exists {
		return &ActivityError{Op: "validate staff", Err: errors.New("staff not found")}
	}
	return nil
}

// checkSupervisorNotDuplicate verifies a supervisor is not already assigned to the group
func (s *Service) checkSupervisorNotDuplicate(staffID int64, existingSupervisors []*activities.SupervisorPlanned) error {
	for _, existing := range existingSupervisors {
		if existing.StaffID == staffID {
			return &ActivityError{Op: "add supervisor", Err: errors.New("supervisor already assigned to this group")}
		}
	}
	return nil
}

// unsetPrimarySupervisorsInTx unsets primary flag for other supervisors if needed
func (s *Service) unsetPrimarySupervisorsInTx(ctx context.Context, txService ActivityService, isPrimary bool, existingSupervisors []*activities.SupervisorPlanned) error {
	if !isPrimary {
		return nil
	}

	for _, existing := range existingSupervisors {
		if existing.IsPrimary {
			existing.IsPrimary = false
			if err := txService.(*Service).supervisorRepo.Update(ctx, existing); err != nil {
				return &ActivityError{Op: "update existing supervisor", Err: err}
			}
		}
	}
	return nil
}

// GetSupervisor retrieves a supervisor by ID
func (s *Service) GetSupervisor(ctx context.Context, id int64) (*activities.SupervisorPlanned, error) {
	supervisor, err := s.supervisorRepo.FindByID(ctx, id)
	if err != nil {
		// Check for "no rows" error and convert to our own error
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: opGetSupervisor, Err: ErrSupervisorNotFound}
		}
		// Check if the wrapped database error contains sql.ErrNoRows
		if dbErr, ok := err.(*base.DatabaseError); ok && errors.Is(dbErr.Err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: opGetSupervisor, Err: ErrSupervisorNotFound}
		}
		return nil, &ActivityError{Op: opGetSupervisor, Err: err}
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

// GetSupervisorsForGroups retrieves all supervisors for multiple groups in a single query
func (s *Service) GetSupervisorsForGroups(ctx context.Context, groupIDs []int64) (map[int64][]*activities.SupervisorPlanned, error) {
	if len(groupIDs) == 0 {
		return make(map[int64][]*activities.SupervisorPlanned), nil
	}

	supervisors, err := s.supervisorRepo.FindByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, &ActivityError{Op: "get supervisors for groups", Err: err}
	}

	// Build map of group ID to supervisors
	supervisorMap := make(map[int64][]*activities.SupervisorPlanned)
	for _, supervisor := range supervisors {
		supervisorMap[supervisor.GroupID] = append(supervisorMap[supervisor.GroupID], supervisor)
	}

	return supervisorMap, nil
}

// UpdateSupervisor updates a supervisor's details
func (s *Service) UpdateSupervisor(ctx context.Context, supervisor *activities.SupervisorPlanned) (*activities.SupervisorPlanned, error) {
	var result *activities.SupervisorPlanned

	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(ActivityService)

		existingSupervisor, err := s.findExistingSupervisor(ctx, txService, supervisor.ID)
		if err != nil {
			return err
		}

		if err := supervisor.Validate(); err != nil {
			return &ActivityError{Op: opValidateSupervisor, Err: err}
		}

		if err := s.handlePrimaryStatusChangeInTx(ctx, txService, supervisor, existingSupervisor); err != nil {
			return err
		}

		if err := txService.(*Service).supervisorRepo.Update(ctx, supervisor); err != nil {
			return &ActivityError{Op: opUpdateSupervisor, Err: err}
		}

		updatedSupervisor, err := txService.(*Service).supervisorRepo.FindByID(ctx, supervisor.ID)
		if err != nil {
			return &ActivityError{Op: "retrieve updated supervisor", Err: err}
		}

		result = updatedSupervisor
		return nil
	})

	if err != nil {
		return nil, &ActivityError{Op: opUpdateSupervisor, Err: err}
	}

	return result, nil
}

// findExistingSupervisor finds and validates that a supervisor exists
func (s *Service) findExistingSupervisor(ctx context.Context, txService ActivityService, supervisorID int64) (*activities.SupervisorPlanned, error) {
	existingSupervisor, err := txService.(*Service).supervisorRepo.FindByID(ctx, supervisorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSupervisorNotFound
		}
		return nil, &ActivityError{Op: opFindSupervisor, Err: err}
	}
	return existingSupervisor, nil
}

// handlePrimaryStatusChangeInTx handles primary status changes for supervisors
func (s *Service) handlePrimaryStatusChangeInTx(ctx context.Context, txService ActivityService, supervisor, existingSupervisor *activities.SupervisorPlanned) error {
	if existingSupervisor.IsPrimary == supervisor.IsPrimary {
		return nil
	}

	if !supervisor.IsPrimary {
		return &ActivityError{Op: opUpdateSupervisor, Err: errors.New("at least one supervisor must remain primary")}
	}

	otherSupervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, supervisor.GroupID)
	if err != nil {
		return &ActivityError{Op: opFindGroupSupervisors, Err: err}
	}

	for _, other := range otherSupervisors {
		if other.ID != supervisor.ID && other.IsPrimary {
			other.IsPrimary = false
			if err := txService.(*Service).supervisorRepo.Update(ctx, other); err != nil {
				return &ActivityError{Op: "update other supervisor", Err: err}
			}
		}
	}

	return nil
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
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(ActivityService)

		supervisor, err := txService.(*Service).supervisorRepo.FindByID(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrSupervisorNotFound
			}
			return &ActivityError{Op: opFindSupervisor, Err: err}
		}

		if err := s.handlePrimaryDeletionInTx(ctx, txService, supervisor, id); err != nil {
			return err
		}

		if err := txService.(*Service).supervisorRepo.Delete(ctx, id); err != nil {
			return &ActivityError{Op: "delete supervisor record", Err: err}
		}

		return nil
	})

	if err != nil {
		return &ActivityError{Op: opDeleteSupervisor, Err: err}
	}

	return nil
}

// handlePrimaryDeletionInTx ensures a new primary is promoted when deleting a primary supervisor
func (s *Service) handlePrimaryDeletionInTx(ctx context.Context, txService ActivityService, supervisor *activities.SupervisorPlanned, supervisorID int64) error {
	if !supervisor.IsPrimary {
		return nil
	}

	allSupervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, supervisor.GroupID)
	if err != nil {
		return &ActivityError{Op: opFindGroupSupervisors, Err: err}
	}

	newPrimary := s.findNewPrimarySupervisor(allSupervisors, supervisorID)
	if newPrimary == nil {
		return &ActivityError{Op: opDeleteSupervisor, Err: errors.New("cannot delete the only supervisor for an activity")}
	}

	newPrimary.IsPrimary = true
	if err := txService.(*Service).supervisorRepo.Update(ctx, newPrimary); err != nil {
		return &ActivityError{Op: "promote new primary supervisor", Err: err}
	}

	return nil
}

// findNewPrimarySupervisor finds a suitable supervisor to promote to primary
func (s *Service) findNewPrimarySupervisor(supervisors []*activities.SupervisorPlanned, excludeID int64) *activities.SupervisorPlanned {
	for _, supervisor := range supervisors {
		if supervisor.ID != excludeID {
			return supervisor
		}
	}
	return nil
}

// SetPrimarySupervisor sets a supervisor as primary and unsets others
func (s *Service) SetPrimarySupervisor(ctx context.Context, id int64) error {
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(ActivityService)

		supervisor, err := txService.(*Service).supervisorRepo.FindByID(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrSupervisorNotFound
			}
			return &ActivityError{Op: opFindSupervisor, Err: err}
		}

		supervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, supervisor.GroupID)
		if err != nil {
			return &ActivityError{Op: opFindGroupSupervisors, Err: err}
		}

		if err := s.updateSupervisorPrimaryStatusInTx(ctx, txService, id, supervisors); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return &ActivityError{Op: "set primary supervisor", Err: err}
	}

	return nil
}

// updateSupervisorPrimaryStatusInTx updates primary status for all supervisors in a group
func (s *Service) updateSupervisorPrimaryStatusInTx(ctx context.Context, txService ActivityService, primaryID int64, supervisors []*activities.SupervisorPlanned) error {
	for _, sup := range supervisors {
		newPrimaryStatus := sup.ID == primaryID
		if sup.IsPrimary == newPrimaryStatus {
			continue
		}

		sup.IsPrimary = newPrimaryStatus
		if err := txService.(*Service).supervisorRepo.Update(ctx, sup); err != nil {
			return &ActivityError{Op: "update supervisor primary status", Err: err}
		}
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
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(ActivityService)

		if err := s.validateGroupExists(ctx, txService, groupID); err != nil {
			return err
		}

		enrollments, err := txService.(*Service).enrollmentRepo.FindByGroupID(ctx, groupID)
		if err != nil {
			return &ActivityError{Op: "find enrollments", Err: err}
		}

		enrollmentID, err := s.findEnrollmentID(enrollments, studentID)
		if err != nil {
			return err
		}

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

// findEnrollmentID finds the enrollment ID for a specific student in a group
func (s *Service) findEnrollmentID(enrollments []*activities.StudentEnrollment, studentID int64) (int64, error) {
	for _, enrollment := range enrollments {
		if enrollment.StudentID == studentID {
			return enrollment.ID, nil
		}
	}
	return 0, ErrNotEnrolled
}

// UpdateGroupEnrollments updates the student enrollments for a group
// This follows the education.UpdateGroupTeachers pattern but for student enrollments
func (s *Service) UpdateGroupEnrollments(ctx context.Context, groupID int64, studentIDs []int64) error {
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "UpdateGroupEnrollments", Err: ErrGroupNotFound}
	}

	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(ActivityService)

		enrollments, err := txService.(*Service).enrollmentRepo.FindByGroupID(ctx, groupID)
		if err != nil {
			return &ActivityError{Op: "get current enrollments", Err: err}
		}

		currentStudentIDs, newStudentIDs := s.buildEnrollmentMaps(enrollments, studentIDs)

		if err := s.removeUnwantedEnrollmentsInTx(ctx, txService, currentStudentIDs, newStudentIDs); err != nil {
			return err
		}

		if err := s.addNewEnrollmentsInTx(ctx, txService, groupID, currentStudentIDs, studentIDs); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return &ActivityError{Op: "update group enrollments", Err: err}
	}

	return nil
}

// buildEnrollmentMaps creates comparison maps for current and new enrollments
func (s *Service) buildEnrollmentMaps(enrollments []*activities.StudentEnrollment, studentIDs []int64) (map[int64]int64, map[int64]bool) {
	currentStudentIDs := make(map[int64]int64) // studentID -> enrollmentID
	for _, enrollment := range enrollments {
		currentStudentIDs[enrollment.StudentID] = enrollment.ID
	}

	newStudentIDs := make(map[int64]bool)
	for _, studentID := range studentIDs {
		newStudentIDs[studentID] = true
	}

	return currentStudentIDs, newStudentIDs
}

// removeUnwantedEnrollmentsInTx removes students that are no longer enrolled
func (s *Service) removeUnwantedEnrollmentsInTx(ctx context.Context, txService ActivityService, currentStudentIDs map[int64]int64, newStudentIDs map[int64]bool) error {
	for studentID, enrollmentID := range currentStudentIDs {
		if !newStudentIDs[studentID] {
			if err := txService.(*Service).enrollmentRepo.Delete(ctx, enrollmentID); err != nil {
				return &ActivityError{Op: "delete enrollment", Err: err}
			}
		}
	}
	return nil
}

// addNewEnrollmentsInTx adds new student enrollments
func (s *Service) addNewEnrollmentsInTx(ctx context.Context, txService ActivityService, groupID int64, currentStudentIDs map[int64]int64, studentIDs []int64) error {
	for _, studentID := range studentIDs {
		if _, exists := currentStudentIDs[studentID]; !exists {
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
}

// UpdateGroupSupervisors updates the supervisors for a group
// This follows the UpdateGroupEnrollments pattern but for supervisors
func (s *Service) UpdateGroupSupervisors(ctx context.Context, groupID int64, staffIDs []int64) error {
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "UpdateGroupSupervisors", Err: ErrGroupNotFound}
	}

	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(ActivityService)

		supervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, groupID)
		if err != nil {
			return &ActivityError{Op: "get current supervisors", Err: err}
		}

		currentStaffIDs, newStaffIDs := s.buildSupervisorMaps(supervisors, staffIDs)

		if err := s.removeUnwantedSupervisorsInTx(ctx, txService, currentStaffIDs, newStaffIDs, staffIDs); err != nil {
			return err
		}

		if err := s.addNewSupervisorsInTx(ctx, txService, groupID, currentStaffIDs, newStaffIDs, staffIDs); err != nil {
			return err
		}

		if err := s.ensurePrimarySupervisorInTx(ctx, txService, groupID, staffIDs); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return &ActivityError{Op: "update group supervisors", Err: err}
	}

	return nil
}

// buildSupervisorMaps creates comparison maps for current and new supervisors
func (s *Service) buildSupervisorMaps(supervisors []*activities.SupervisorPlanned, staffIDs []int64) (map[int64]int64, map[int64]bool) {
	currentStaffIDs := make(map[int64]int64) // staffID -> supervisorID
	for _, supervisor := range supervisors {
		currentStaffIDs[supervisor.StaffID] = supervisor.ID
	}

	newStaffIDs := make(map[int64]bool)
	for _, staffID := range staffIDs {
		newStaffIDs[staffID] = true
	}

	return currentStaffIDs, newStaffIDs
}

// removeUnwantedSupervisorsInTx removes supervisors that are no longer assigned
func (s *Service) removeUnwantedSupervisorsInTx(ctx context.Context, txService ActivityService, currentStaffIDs map[int64]int64, newStaffIDs map[int64]bool, staffIDs []int64) error {
	if len(currentStaffIDs) == 1 && len(staffIDs) == 0 {
		return &ActivityError{Op: "update supervisors", Err: errors.New("cannot remove all supervisors from an activity")}
	}

	for staffID, supervisorID := range currentStaffIDs {
		if !newStaffIDs[staffID] {
			if err := txService.(*Service).supervisorRepo.Delete(ctx, supervisorID); err != nil {
				return &ActivityError{Op: opDeleteSupervisor, Err: err}
			}
		}
	}
	return nil
}

// addNewSupervisorsInTx adds new supervisor assignments
func (s *Service) addNewSupervisorsInTx(ctx context.Context, txService ActivityService, groupID int64, currentStaffIDs map[int64]int64, newStaffIDs map[int64]bool, staffIDs []int64) error {
	for i, staffID := range staffIDs {
		if _, exists := currentStaffIDs[staffID]; !exists {
			// Validate staff exists before creating supervisor
			if err := s.validateStaffExists(ctx, staffID); err != nil {
				return err
			}

			isPrimary := i == 0 && len(newStaffIDs) == len(staffIDs)

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
	return nil
}

// ensurePrimarySupervisorInTx ensures the first supervisor in the list is primary
func (s *Service) ensurePrimarySupervisorInTx(ctx context.Context, txService ActivityService, groupID int64, staffIDs []int64) error {
	if len(staffIDs) == 0 {
		return nil
	}

	updatedSupervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "get updated supervisors", Err: err}
	}

	primaryStaffID := staffIDs[0]
	for _, supervisor := range updatedSupervisors {
		shouldBePrimary := supervisor.StaffID == primaryStaffID
		if supervisor.IsPrimary == shouldBePrimary {
			continue
		}

		supervisor.IsPrimary = shouldBePrimary
		if err := txService.(*Service).supervisorRepo.Update(ctx, supervisor); err != nil {
			if shouldBePrimary {
				return &ActivityError{Op: "set primary supervisor", Err: err}
			}
			return &ActivityError{Op: "remove primary status", Err: err}
		}
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
