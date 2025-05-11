package activities

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/activities"
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
	}, nil
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
		return nil, &ActivityError{Op: "get category", Err: ErrCategoryNotFound}
	}
	return category, nil
}

// UpdateCategory updates an existing category
func (s *Service) UpdateCategory(ctx context.Context, category *activities.Category) (*activities.Category, error) {
	if err := category.Validate(); err != nil {
		return nil, &ActivityError{Op: "update category", Err: err}
	}

	// Verify category exists - use _ since we don't use the result
	_, err := s.categoryRepo.FindByID(ctx, category.ID)
	if err != nil {
		return nil, &ActivityError{Op: "update category", Err: ErrCategoryNotFound}
	}

	// Update the category
	if err := s.categoryRepo.Update(ctx, category); err != nil {
		return nil, &ActivityError{Op: "update category", Err: err}
	}

	// Get the updated category
	updated, err := s.categoryRepo.FindByID(ctx, category.ID)
	if err != nil {
		return nil, &ActivityError{Op: "retrieve updated category", Err: err}
	}

	return updated, nil
}

// DeleteCategory deletes a category by ID
func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	// Verify category exists
	_, err := s.categoryRepo.FindByID(ctx, id)
	if err != nil {
		return &ActivityError{Op: "delete category", Err: ErrCategoryNotFound}
	}

	// Delete the category
	if err := s.categoryRepo.Delete(ctx, id); err != nil {
		return &ActivityError{Op: "delete category", Err: err}
	}

	return nil
}

// ListCategories retrieves all categories
func (s *Service) ListCategories(ctx context.Context) ([]*activities.Category, error) {
	return s.categoryRepo.ListAll(ctx)
}

// ======== Activity Group Methods ========

// CreateGroup creates a new activity group with supervisors and schedules
func (s *Service) CreateGroup(ctx context.Context, group *activities.Group, supervisorIDs []int64, schedules []*activities.Schedule) (*activities.Group, error) {
	if err := group.Validate(); err != nil {
		return nil, &ActivityError{Op: "create group", Err: err}
	}

	// Validate each schedule if provided
	for _, schedule := range schedules {
		if err := schedule.Validate(); err != nil {
			return nil, &ActivityError{Op: "create group", Err: fmt.Errorf("invalid schedule: %w", err)}
		}
	}

	// Execute in transaction
	err := s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Store transaction in context
		ctx = context.WithValue(ctx, "tx", &tx)

		// Create the group
		if err := s.groupRepo.Create(ctx, group); err != nil {
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

			if err := s.supervisorRepo.Create(ctx, supervisor); err != nil {
				return err
			}
		}

		// Create schedules if provided
		for _, schedule := range schedules {
			schedule.ActivityGroupID = group.ID
			if err := s.scheduleRepo.Create(ctx, schedule); err != nil {
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
		return nil, &ActivityError{Op: "get group", Err: ErrGroupNotFound}
	}
	return group, nil
}

// UpdateGroup updates an existing activity group
func (s *Service) UpdateGroup(ctx context.Context, group *activities.Group) (*activities.Group, error) {
	if err := group.Validate(); err != nil {
		return nil, &ActivityError{Op: "update group", Err: err}
	}

	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, group.ID)
	if err != nil {
		return nil, &ActivityError{Op: "update group", Err: ErrGroupNotFound}
	}

	// Update the group
	if err := s.groupRepo.Update(ctx, group); err != nil {
		return nil, &ActivityError{Op: "update group", Err: err}
	}

	// Get the updated group
	updated, err := s.groupRepo.FindByID(ctx, group.ID)
	if err != nil {
		return nil, &ActivityError{Op: "retrieve updated group", Err: err}
	}

	return updated, nil
}

// DeleteGroup deletes an activity group by ID
func (s *Service) DeleteGroup(ctx context.Context, id int64) error {
	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		return &ActivityError{Op: "delete group", Err: ErrGroupNotFound}
	}

	// Execute in transaction
	err = s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Store transaction in context
		ctx = context.WithValue(ctx, "tx", &tx)

		// Delete all related schedules
		schedules, err := s.scheduleRepo.FindByGroupID(ctx, id)
		if err == nil {
			for _, schedule := range schedules {
				if err := s.scheduleRepo.Delete(ctx, schedule.ID); err != nil {
					return err
				}
			}
		}

		// Delete all related supervisors
		supervisors, err := s.supervisorRepo.FindByGroupID(ctx, id)
		if err == nil {
			for _, supervisor := range supervisors {
				if err := s.supervisorRepo.Delete(ctx, supervisor.ID); err != nil {
					return err
				}
			}
		}

		// Delete the group
		return s.groupRepo.Delete(ctx, id)
	})

	if err != nil {
		return &ActivityError{Op: "delete group transaction", Err: err}
	}

	return nil
}

// ListGroups retrieves activity groups with optional filtering
func (s *Service) ListGroups(ctx context.Context, filters map[string]interface{}) ([]*activities.Group, error) {
	return s.groupRepo.List(ctx, nil) // Convert filters to QueryOptions
}

// GetGroupWithDetails retrieves an activity group with its supervisors and schedules
func (s *Service) GetGroupWithDetails(ctx context.Context, id int64) (*activities.Group, []*activities.SupervisorPlanned, []*activities.Schedule, error) {
	// Get the group
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, nil, nil, &ActivityError{Op: "get group details", Err: ErrGroupNotFound}
	}

	// Get supervisors
	supervisors, err := s.supervisorRepo.FindByGroupID(ctx, id)
	if err != nil {
		supervisors = []*activities.SupervisorPlanned{}
	}

	// Get schedules
	schedules, err := s.scheduleRepo.FindByGroupID(ctx, id)
	if err != nil {
		schedules = []*activities.Schedule{}
	}

	return group, supervisors, schedules, nil
}

// GetGroupsWithEnrollmentCounts retrieves all groups with their enrollment counts
func (s *Service) GetGroupsWithEnrollmentCounts(ctx context.Context) ([]*activities.Group, map[int64]int, error) {
	return s.groupRepo.FindWithEnrollmentCounts(ctx)
}

// ======== Schedule Methods ========

// AddSchedule adds a new schedule to an activity group
func (s *Service) AddSchedule(ctx context.Context, groupID int64, schedule *activities.Schedule) (*activities.Schedule, error) {
	if err := schedule.Validate(); err != nil {
		return nil, &ActivityError{Op: "add schedule", Err: err}
	}

	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, &ActivityError{Op: "add schedule", Err: ErrGroupNotFound}
	}

	// Set the group ID
	schedule.ActivityGroupID = groupID

	// Create the schedule
	if err := s.scheduleRepo.Create(ctx, schedule); err != nil {
		return nil, &ActivityError{Op: "add schedule", Err: err}
	}

	return schedule, nil
}

// GetSchedule retrieves a schedule by ID
func (s *Service) GetSchedule(ctx context.Context, id int64) (*activities.Schedule, error) {
	schedule, err := s.scheduleRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActivityError{Op: "get schedule", Err: ErrScheduleNotFound}
	}
	return schedule, nil
}

// GetGroupSchedules retrieves all schedules for an activity group
func (s *Service) GetGroupSchedules(ctx context.Context, groupID int64) ([]*activities.Schedule, error) {
	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, &ActivityError{Op: "get group schedules", Err: ErrGroupNotFound}
	}

	// Get schedules
	schedules, err := s.scheduleRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return []*activities.Schedule{}, nil
	}

	return schedules, nil
}

// DeleteSchedule deletes a schedule by ID
func (s *Service) DeleteSchedule(ctx context.Context, id int64) error {
	// Verify schedule exists
	schedule, err := s.scheduleRepo.FindByID(ctx, id)
	if err != nil {
		return &ActivityError{Op: "delete schedule", Err: ErrScheduleNotFound}
	}

	// Delete the schedule
	if err := s.scheduleRepo.Delete(ctx, schedule.ID); err != nil {
		return &ActivityError{Op: "delete schedule", Err: err}
	}

	return nil
}

// ======== Supervisor Methods ========

// AddSupervisor adds a new supervisor to an activity group
func (s *Service) AddSupervisor(ctx context.Context, groupID int64, staffID int64, isPrimary bool) (*activities.SupervisorPlanned, error) {
	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, &ActivityError{Op: "add supervisor", Err: ErrGroupNotFound}
	}

	// Create supervisor
	supervisor := &activities.SupervisorPlanned{
		StaffID:   staffID,
		GroupID:   groupID,
		IsPrimary: isPrimary,
	}

	if err := supervisor.Validate(); err != nil {
		return nil, &ActivityError{Op: "add supervisor", Err: err}
	}

	// If this is the primary supervisor, update any existing primary supervisors
	if isPrimary {
		err = s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			// Store transaction in context
			ctx = context.WithValue(ctx, "tx", &tx)

			// Get current primary supervisor if any
			primarySupervisor, err := s.supervisorRepo.FindPrimaryByGroupID(ctx, groupID)
			if err == nil && primarySupervisor != nil {
				// Unset primary flag
				primarySupervisor.IsPrimary = false
				if err := s.supervisorRepo.Update(ctx, primarySupervisor); err != nil {
					return err
				}
			}

			// Create the new supervisor
			return s.supervisorRepo.Create(ctx, supervisor)
		})
	} else {
		// Create the supervisor directly
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
		return nil, &ActivityError{Op: "get supervisor", Err: ErrSupervisorNotFound}
	}
	return supervisor, nil
}

// GetGroupSupervisors retrieves all supervisors for an activity group
func (s *Service) GetGroupSupervisors(ctx context.Context, groupID int64) ([]*activities.SupervisorPlanned, error) {
	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, &ActivityError{Op: "get group supervisors", Err: ErrGroupNotFound}
	}

	// Get supervisors
	supervisors, err := s.supervisorRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return []*activities.SupervisorPlanned{}, nil
	}

	return supervisors, nil
}

// DeleteSupervisor deletes a supervisor by ID
func (s *Service) DeleteSupervisor(ctx context.Context, id int64) error {
	// Verify supervisor exists
	supervisor, err := s.supervisorRepo.FindByID(ctx, id)
	if err != nil {
		return &ActivityError{Op: "delete supervisor", Err: ErrSupervisorNotFound}
	}

	// Check if this is the primary supervisor
	if supervisor.IsPrimary {
		// Can't delete the primary supervisor without assigning a new one
		return &ActivityError{Op: "delete supervisor", Err: errors.New("cannot delete primary supervisor, assign a new primary first")}
	}

	// Delete the supervisor
	if err := s.supervisorRepo.Delete(ctx, supervisor.ID); err != nil {
		return &ActivityError{Op: "delete supervisor", Err: err}
	}

	return nil
}

// SetPrimarySupervisor sets a supervisor as the primary supervisor for a group
func (s *Service) SetPrimarySupervisor(ctx context.Context, id int64) error {
	// Verify supervisor exists
	supervisor, err := s.supervisorRepo.FindByID(ctx, id)
	if err != nil {
		return &ActivityError{Op: "set primary supervisor", Err: ErrSupervisorNotFound}
	}

	// Execute in transaction
	err = s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Store transaction in context
		ctx = context.WithValue(ctx, "tx", &tx)

		// Get current primary supervisor if any
		primarySupervisor, err := s.supervisorRepo.FindPrimaryByGroupID(ctx, supervisor.GroupID)
		if err == nil && primarySupervisor != nil && primarySupervisor.ID != supervisor.ID {
			// Unset primary flag
			primarySupervisor.IsPrimary = false
			if err := s.supervisorRepo.Update(ctx, primarySupervisor); err != nil {
				return err
			}
		}

		// Set this supervisor as primary
		return s.supervisorRepo.SetPrimary(ctx, supervisor.ID)
	})

	if err != nil {
		return &ActivityError{Op: "set primary supervisor transaction", Err: err}
	}

	return nil
}

// ======== Enrollment Methods ========

// EnrollStudent enrolls a student in an activity group
func (s *Service) EnrollStudent(ctx context.Context, groupID, studentID int64) error {
	// Verify group exists
	group, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "enroll student", Err: ErrGroupNotFound}
	}

	// Check if group is open for enrollment
	if !group.IsOpen {
		return &ActivityError{Op: "enroll student", Err: ErrGroupClosed}
	}

	// Get current enrollment count
	count, err := s.enrollmentRepo.CountByGroupID(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "enroll student", Err: err}
	}

	// Check if group has available spots
	if !group.HasAvailableSpots(count) {
		return &ActivityError{Op: "enroll student", Err: ErrGroupFull}
	}

	// Check if student is already enrolled
	enrollments, err := s.enrollmentRepo.FindByStudentID(ctx, studentID)
	if err == nil {
		for _, enrollment := range enrollments {
			if enrollment.ActivityGroupID == groupID {
				return &ActivityError{Op: "enroll student", Err: ErrAlreadyEnrolled}
			}
		}
	}

	// Create enrollment
	enrollment := &activities.StudentEnrollment{
		StudentID:       studentID,
		ActivityGroupID: groupID,
		EnrollmentDate:  time.Now(),
	}

	if err := enrollment.Validate(); err != nil {
		return &ActivityError{Op: "enroll student", Err: err}
	}

	if err := s.enrollmentRepo.Create(ctx, enrollment); err != nil {
		return &ActivityError{Op: "enroll student", Err: err}
	}

	return nil
}

// UnenrollStudent removes a student from an activity group
func (s *Service) UnenrollStudent(ctx context.Context, groupID, studentID int64) error {
	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "unenroll student", Err: ErrGroupNotFound}
	}

	// Get student enrollments
	enrollments, err := s.enrollmentRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return &ActivityError{Op: "unenroll student", Err: ErrNotEnrolled}
	}

	// Find the enrollment for this group
	var enrollmentID int64
	found := false
	for _, enrollment := range enrollments {
		if enrollment.ActivityGroupID == groupID {
			enrollmentID = enrollment.ID
			found = true
			break
		}
	}

	if !found {
		return &ActivityError{Op: "unenroll student", Err: ErrNotEnrolled}
	}

	// Delete the enrollment
	if err := s.enrollmentRepo.Delete(ctx, enrollmentID); err != nil {
		return &ActivityError{Op: "unenroll student", Err: err}
	}

	return nil
}

// GetEnrolledStudents retrieves all students enrolled in an activity group
func (s *Service) GetEnrolledStudents(ctx context.Context, groupID int64) ([]*users.Student, error) {
	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, &ActivityError{Op: "get enrolled students", Err: ErrGroupNotFound}
	}

	// This requires a custom query that joins StudentEnrollment with Student
	// Similar to what we see in the API's ListEnrolledStudents method
	// You might need to implement this in the repository layer

	// For now, we'll return an empty list
	return []*users.Student{}, nil
}

// GetStudentEnrollments retrieves all activity groups a student is enrolled in
func (s *Service) GetStudentEnrollments(ctx context.Context, studentID int64) ([]*activities.Group, error) {
	// Get all enrollments for the student using StudentEnrollmentRepository
	enrollments, err := s.enrollmentRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ActivityError{Op: "get student enrollments", Err: err}
	}

	// Build list of groups from enrollments
	groups := make([]*activities.Group, 0, len(enrollments))
	for _, enrollment := range enrollments {
		group, err := s.groupRepo.FindByID(ctx, enrollment.ActivityGroupID)
		if err != nil {
			// Skip groups that can't be found
			continue
		}
		groups = append(groups, group)
	}

	return groups, nil
}

// GetAvailableGroups retrieves all activity groups a student can enroll in
func (s *Service) GetAvailableGroups(ctx context.Context, studentID int64) ([]*activities.Group, error) {
	// Get student's current enrollments
	enrolledGroups, err := s.GetStudentEnrollments(ctx, studentID)
	if err != nil {
		return nil, &ActivityError{Op: "get available groups", Err: err}
	}

	// Create a set of enrolled group IDs
	enrolledGroupIDs := make(map[int64]bool)
	for _, group := range enrolledGroups {
		enrolledGroupIDs[group.ID] = true
	}

	// Get all open groups
	openGroups, err := s.groupRepo.FindOpenGroups(ctx)
	if err != nil {
		return nil, &ActivityError{Op: "get available groups", Err: err}
	}

	// Get enrollment counts
	_, countMap, err := s.groupRepo.FindWithEnrollmentCounts(ctx)
	if err != nil {
		return nil, &ActivityError{Op: "get available groups", Err: err}
	}

	// Filter out groups the student is already enrolled in and groups that are full
	availableGroups := make([]*activities.Group, 0)
	for _, group := range openGroups {
		// Skip if student is already enrolled
		if enrolledGroupIDs[group.ID] {
			continue
		}

		// Get current enrollment count
		count := countMap[group.ID]

		// Skip if group is full
		if !group.HasAvailableSpots(count) {
			continue
		}

		availableGroups = append(availableGroups, group)
	}

	return availableGroups, nil
}

// UpdateAttendanceStatus updates the attendance status for a student enrollment
func (s *Service) UpdateAttendanceStatus(ctx context.Context, enrollmentID int64, status *string) error {
	// Validate status if provided
	if status != nil && !activities.IsValidAttendanceStatus(*status) {
		return &ActivityError{Op: "update attendance status", Err: ErrInvalidAttendanceStatus}
	}

	// Update the attendance status
	err := s.enrollmentRepo.UpdateAttendanceStatus(ctx, enrollmentID, status)
	if err != nil {
		return &ActivityError{Op: "update attendance status", Err: err}
	}

	return nil
}

// ======== Public Methods ========

// GetPublicGroups retrieves public activity groups with optional category filtering
func (s *Service) GetPublicGroups(ctx context.Context, categoryID *int64) ([]*activities.Group, map[int64]int, error) {
	// Build filters for open and active groups
	filters := map[string]interface{}{
		"is_open": true,
		"active":  true,
	}

	// Add category filter if provided
	if categoryID != nil {
		filters["category_id"] = *categoryID
	}

	// Get groups with enrollment counts
	groups, countMap, err := s.groupRepo.FindWithEnrollmentCounts(ctx)
	if err != nil {
		return nil, nil, &ActivityError{Op: "get public groups", Err: err}
	}

	// Filter for open and active groups
	publicGroups := make([]*activities.Group, 0)
	for _, group := range groups {
		if group.IsOpen {
			publicGroups = append(publicGroups, group)
		}
	}

	return publicGroups, countMap, nil
}

// GetPublicCategories retrieves all activity categories for public viewing
func (s *Service) GetPublicCategories(ctx context.Context) ([]*activities.Category, error) {
	return s.categoryRepo.ListAll(ctx)
}
