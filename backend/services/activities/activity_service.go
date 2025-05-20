package activities

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
			return nil, &ActivityError{Op: "get category", Err: ErrCategoryNotFound}
		}
		// Check if the wrapped database error contains sql.ErrNoRows
		if dbErr, ok := err.(*base.DatabaseError); ok && errors.Is(dbErr.Err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: "get category", Err: ErrCategoryNotFound}
		}
		return nil, &ActivityError{Op: "get category", Err: err}
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

	// Execute in transaction
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Create the group
		if err := txService.(*Service).groupRepo.Create(ctx, group); err != nil {
			return err
		}

		// Create supervisors if provided
		for i, staffID := range supervisorIDs {
			isPrimary := i == 0 // First supervisor is primary
			supervisor := &activities.SupervisorPlanned{
				StaffID:   staffID,
				GroupID:   group.ID,
				IsPrimary: isPrimary,
			}

			if err := supervisor.Validate(); err != nil {
				return fmt.Errorf("invalid supervisor (%d): %w", staffID, err)
			}

			if err := txService.(*Service).supervisorRepo.Create(ctx, supervisor); err != nil {
				return err
			}
		}

		// Create schedules if provided
		for _, schedule := range schedules {
			schedule.ActivityGroupID = group.ID
			if err := txService.(*Service).scheduleRepo.Create(ctx, schedule); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, &ActivityError{Op: "create group transaction", Err: err}
	}

	// Retrieve the created group with all relations
	createdGroup, err := s.groupRepo.FindByID(ctx, group.ID)
	if err != nil {
		return nil, &ActivityError{Op: "retrieve created group", Err: err}
	}

	return createdGroup, nil
}

// GetGroup retrieves an activity group by ID
func (s *Service) GetGroup(ctx context.Context, id int64) (*activities.Group, error) {
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		// Check for "no rows" error and convert to our own error
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: "get group", Err: ErrGroupNotFound}
		}
		// Check if the wrapped database error contains sql.ErrNoRows
		if dbErr, ok := err.(*base.DatabaseError); ok && errors.Is(dbErr.Err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: "get group", Err: ErrGroupNotFound}
		}
		return nil, &ActivityError{Op: "get group", Err: err}
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
func (s *Service) ListGroups(ctx context.Context, filters map[string]interface{}) ([]*activities.Group, error) {
	// Create QueryOptions with filter
	options := base.NewQueryOptions()

	// Apply filters if provided
	if len(filters) > 0 {
		for key, value := range filters {
			options.Filter.Equal(key, value)
		}
	}

	groups, err := s.groupRepo.List(ctx, options)
	if err != nil {
		return nil, &ActivityError{Op: "list groups", Err: err}
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
			return nil, nil, nil, &ActivityError{Op: "get group", Err: ErrGroupNotFound}
		}
		// Check if the wrapped database error contains sql.ErrNoRows
		if dbErr, ok := err.(*base.DatabaseError); ok && errors.Is(dbErr.Err, sql.ErrNoRows) {
			return nil, nil, nil, &ActivityError{Op: "get group", Err: ErrGroupNotFound}
		}
		return nil, nil, nil, &ActivityError{Op: "get group", Err: err}
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
		return nil, &ActivityError{Op: "find group", Err: err}
	}

	// Set group ID
	schedule.ActivityGroupID = groupID

	// Validate the schedule
	if err := schedule.Validate(); err != nil {
		return nil, &ActivityError{Op: "validate schedule", Err: err}
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
			return nil, &ActivityError{Op: "get schedule", Err: ErrScheduleNotFound}
		}
		// Check if the wrapped database error contains sql.ErrNoRows
		if dbErr, ok := err.(*base.DatabaseError); ok && errors.Is(dbErr.Err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: "get schedule", Err: ErrScheduleNotFound}
		}
		return nil, &ActivityError{Op: "get schedule", Err: err}
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
	if err := s.scheduleRepo.Delete(ctx, id); err != nil {
		return &ActivityError{Op: "delete schedule", Err: err}
	}

	return nil
}

// ======== Supervisor Methods ========

// AddSupervisor adds a supervisor to an activity group
func (s *Service) AddSupervisor(ctx context.Context, groupID int64, staffID int64, isPrimary bool) (*activities.SupervisorPlanned, error) {
	// Check if group exists
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, &ActivityError{Op: "find group", Err: err}
	}

	// Create supervisor record
	supervisor := &activities.SupervisorPlanned{
		GroupID:   groupID,
		StaffID:   staffID,
		IsPrimary: isPrimary,
	}

	// Validate
	if err := supervisor.Validate(); err != nil {
		return nil, &ActivityError{Op: "validate supervisor", Err: err}
	}

	// Execute in transaction if this is the primary supervisor
	if isPrimary {
		err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
			// Get transactional service
			txService := s.WithTx(tx).(ActivityService)

			// If this is primary, unset primary flag for all other supervisors
			existingSupervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, groupID)
			if err != nil {
				return err
			}

			for _, existing := range existingSupervisors {
				if existing.IsPrimary {
					existing.IsPrimary = false
					if err := txService.(*Service).supervisorRepo.Update(ctx, existing); err != nil {
						return err
					}
				}
			}

			// Create the new supervisor
			return txService.(*Service).supervisorRepo.Create(ctx, supervisor)
		})
	} else {
		// Not primary, just create it
		err = s.supervisorRepo.Create(ctx, supervisor)
	}

	if err != nil {
		return nil, &ActivityError{Op: "add supervisor", Err: err}
	}

	return supervisor, nil
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

// DeleteSupervisor deletes a supervisor
func (s *Service) DeleteSupervisor(ctx context.Context, id int64) error {
	supervisor, err := s.supervisorRepo.FindByID(ctx, id)
	if err != nil {
		return &ActivityError{Op: "find supervisor", Err: err}
	}

	if supervisor.IsPrimary {
		return &ActivityError{Op: "delete supervisor", Err: errors.New("cannot delete primary supervisor")}
	}

	if err := s.supervisorRepo.Delete(ctx, id); err != nil {
		return &ActivityError{Op: "delete supervisor", Err: err}
	}

	return nil
}

// SetPrimarySupervisor sets a supervisor as primary and unsets others
func (s *Service) SetPrimarySupervisor(ctx context.Context, id int64) error {
	supervisor, err := s.supervisorRepo.FindByID(ctx, id)
	if err != nil {
		return &ActivityError{Op: "find supervisor", Err: err}
	}

	// Execute in transaction
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Find all supervisors for this group
		supervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, supervisor.GroupID)
		if err != nil {
			return err
		}

		// Update all supervisors
		for _, sup := range supervisors {
			if sup.ID == id {
				sup.IsPrimary = true
			} else {
				sup.IsPrimary = false
			}

			if err := txService.(*Service).supervisorRepo.Update(ctx, sup); err != nil {
				return err
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
	// Check if group exists
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "find group", Err: err}
	}

	// Check if student is already enrolled
	enrollments, err := s.enrollmentRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "check existing enrollment", Err: err}
	}

	// Check if student is already enrolled
	for _, enrollment := range enrollments {
		if enrollment.StudentID == studentID {
			return &ActivityError{Op: "enroll student", Err: errors.New("student is already enrolled in this group")}
		}
	}

	// Create enrollment
	enrollment := &activities.StudentEnrollment{
		StudentID:       studentID,
		ActivityGroupID: groupID,
		EnrollmentDate:  time.Now(),
	}

	if err := s.enrollmentRepo.Create(ctx, enrollment); err != nil {
		return &ActivityError{Op: "enroll student", Err: err}
	}

	return nil
}

// UnenrollStudent removes a student from an activity group
func (s *Service) UnenrollStudent(ctx context.Context, groupID, studentID int64) error {
	// Find the enrollment
	enrollments, err := s.enrollmentRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "find enrollment", Err: err}
	}

	// Look for the specific enrollment
	var found bool
	for _, enrollment := range enrollments {
		if enrollment.StudentID == studentID {
			if err := s.enrollmentRepo.Delete(ctx, enrollment.ID); err != nil {
				return &ActivityError{Op: "unenroll student", Err: err}
			}
			found = true
			break
		}
	}

	if !found {
		return &ActivityError{Op: "unenroll student", Err: errors.New("student is not enrolled in this group")}
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
	// This might require custom implementation based on your specific enrollment repository
	// For now, returning a placeholder error indicating this needs implementation
	return nil, &ActivityError{Op: "get student enrollments", Err: errors.New("method needs implementation")}
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
	return s.enrollmentRepo.UpdateAttendanceStatus(ctx, enrollmentID, status)
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
