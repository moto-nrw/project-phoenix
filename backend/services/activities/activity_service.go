package activities

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/models/activities"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Service implements the ActivityService interface
type Service struct {
	timetable       timetable.Capability
	groupRepo       activities.GroupRepository
	scheduleRepo    activities.ScheduleRepository
	supervisorRepo  activities.SupervisorPlannedRepository
	enrollmentRepo  activities.StudentEnrollmentRepository
	activeGroupRepo activityOccupancy
	staffRepo       userModels.StaffRepository
	studentRepo     userModels.StudentRepository
}

type activityOccupancy interface {
	GetOccupiedActivityGroupIDs(context.Context, []int64) (map[int64]bool, error)
}

// NewService creates a new activity service
func NewService(
	timetableCapability timetable.Capability,
	groupRepo activities.GroupRepository,
	scheduleRepo activities.ScheduleRepository,
	supervisorRepo activities.SupervisorPlannedRepository,
	enrollmentRepo activities.StudentEnrollmentRepository,
	activeGroupRepo activityOccupancy,
	staffRepo userModels.StaffRepository,
	studentRepo userModels.StudentRepository,
) (*Service, error) {
	return &Service{
		timetable:       timetableCapability,
		groupRepo:       groupRepo,
		scheduleRepo:    scheduleRepo,
		supervisorRepo:  supervisorRepo,
		enrollmentRepo:  enrollmentRepo,
		activeGroupRepo: activeGroupRepo,
		staffRepo:       staffRepo,
		studentRepo:     studentRepo,
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

// ======== Category Methods ========

// CreateCategory creates a new activity category
func (s *Service) CreateCategory(ctx context.Context, category *activities.Category) (*activities.Category, error) {
	if category == nil || s.timetable == nil {
		return nil, &ActivityError{Op: "create category", Err: timetable.ErrInvalidCategory}
	}
	created, err := s.timetable.CreateCategory(ctx, timetable.CreateCategory{
		Name: category.Name, Description: category.Description, Color: category.Color, IsSystem: category.IsSystem,
	})
	if err != nil {
		return nil, categoryActivityError("create category", err)
	}
	return categoryFromOwner(created), nil
}

// GetCategory retrieves a category by ID
func (s *Service) GetCategory(ctx context.Context, id int64) (*activities.Category, error) {
	if s.timetable == nil {
		return nil, &ActivityError{Op: opGetCategory, Err: timetable.ErrCategoryNotFound}
	}
	category, err := s.timetable.FindCategory(ctx, id)
	if err != nil {
		return nil, categoryActivityError(opGetCategory, err)
	}
	return categoryFromOwner(category), nil
}

// ListCategories lists all activity categories
func (s *Service) ListCategories(ctx context.Context) ([]*activities.Category, error) {
	if s.timetable == nil {
		return nil, &ActivityError{Op: "list categories", Err: errors.New("category capability is required")}
	}
	categories, err := s.timetable.ListCategories(ctx)
	if err != nil {
		return nil, &ActivityError{Op: "list categories", Err: err}
	}
	result := make([]*activities.Category, 0, len(categories))
	for _, category := range categories {
		result = append(result, categoryFromOwner(category))
	}
	return result, nil
}

// SetCategoryShiftTypeLinks maps the given categories to a Dienstplan shift type
// and clears the mapping on any category no longer linked to it (#1837
// follow-up). Called from the shift-types admin flow; the FK lives on the
// category side, so the write is owned here. Runs inside the caller's tenant
// transaction (the shift-types router wires TenantTxMiddleware).
func (s *Service) SetCategoryShiftTypeLinks(ctx context.Context, shiftTypeID int64, categoryIDs []int64) error {
	if s.timetable == nil {
		return &ActivityError{Op: "set category shift type links", Err: errors.New("category capability is required")}
	}
	if err := s.timetable.SetCategoryShiftTypeLinks(ctx, shiftTypeID, categoryIDs); err != nil {
		return categoryActivityError("set category shift type links", err)
	}
	return nil
}

// ======== Activity Group Methods ========

// CreateGroup creates a new activity group with supervisors and schedules
func (s *Service) CreateGroup(ctx context.Context, group *activities.Group, supervisorIDs []int64, schedules []*activities.Schedule) (*activities.Group, error) {
	if group != nil && group.IsTemplate {
		return nil, &ActivityError{Op: "create group", Err: ErrTimetableTemplateProtected}
	}
	if err := group.Validate(); err != nil {
		return nil, &ActivityError{Op: "validate group", Err: err}
	}

	if err := s.validateAndSetCategory(ctx, group); err != nil {
		return nil, err
	}
	validatedCategory := group.Category
	group.SetTenantID(group.Category.GetTenantID())

	if err := s.groupRepo.Create(ctx, group); err != nil {
		return nil, &ActivityError{Op: "create group", Err: err}
	}

	if err := s.createSupervisorsInTx(ctx, s, group.ID, supervisorIDs); err != nil {
		return nil, &ActivityError{Op: "create group", Err: err}
	}

	if err := s.createSchedulesInTx(ctx, s, group, schedules); err != nil {
		return nil, &ActivityError{Op: "create group", Err: err}
	}

	result, err := s.groupRepo.FindByID(ctx, group.ID)
	if err != nil {
		return nil, &ActivityError{Op: "retrieve created group", Err: err}
	}
	result.Category = validatedCategory

	return result, nil
}

// validateAndSetCategory validates and sets the category if provided
func (s *Service) validateAndSetCategory(ctx context.Context, group *activities.Group) error {
	if group.CategoryID <= 0 {
		return nil
	}

	category, err := s.timetable.FindCategoryForAssignment(ctx, group.CategoryID)
	if err != nil {
		return categoryActivityError("validate category", err)
	}
	group.Category = categoryFromOwner(category)
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
func (s *Service) createSchedulesInTx(ctx context.Context, txService ActivityService, group *activities.Group, schedules []*activities.Schedule) error {
	for _, schedule := range schedules {
		schedule.ActivityGroupID = group.ID
		schedule.SetTenantID(group.GetTenantID())

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
	group, err := s.timetable.FindGroup(ctx, id)
	if err != nil {
		if errors.Is(err, timetable.ErrGroupNotFound) || errors.Is(err, timetable.ErrInvalidGroupQuery) {
			return nil, &ActivityError{Op: opGetGroup, Err: ErrGroupNotFound}
		}
		return nil, &ActivityError{Op: opGetGroup, Err: err}
	}

	return groupFromOwner(group), nil
}

// findMutableActivityGroup resolves the persisted group and rejects timetable
// templates. The generic activities service has none of the recurrence lock,
// split-lineage, or care-offering validation required to mutate templates.
// Every legacy mutation entry point must call this guard before writing.
func (s *Service) findMutableActivityGroup(ctx context.Context, id int64) (*activities.Group, error) {
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		if isRepositoryNotFound(err) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}
	if group.IsTemplate {
		return nil, ErrTimetableTemplateProtected
	}
	return group, nil
}

// UpdateGroup updates an activity group with ownership verification
// Only the creator, supervisors, or users with manage permission can update
func (s *Service) UpdateGroup(ctx context.Context, group *activities.Group, requestingStaffID int64, hasManagePermission bool) (*activities.Group, error) {
	if group != nil && group.IsTemplate {
		return nil, &ActivityError{Op: "update group", Err: ErrTimetableTemplateProtected}
	}
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

	// Block renaming system activities (Schulhof Freispiel, WC).
	// Placed after CanModifyActivity to avoid an extra FindByID call — CanModifyActivity
	// already fetches the group internally for non-admin users.
	existingGroup, err := s.findMutableActivityGroup(ctx, group.ID)
	if err != nil {
		return nil, &ActivityError{Op: "update group", Err: err}
	}
	if timetable.IsSystemActivityName(existingGroup.Name) && group.Name != existingGroup.Name {
		return nil, &ActivityError{Op: "update group", Err: ErrSystemActivityProtected}
	}
	if group.CategoryID != existingGroup.CategoryID {
		if err := s.validateAndSetCategory(ctx, group); err != nil {
			return nil, err
		}
	}
	group.SetTenantID(existingGroup.GetTenantID())

	if err := s.groupRepo.Update(ctx, group); err != nil {
		return nil, &ActivityError{Op: "update group", Err: err}
	}

	return group, nil
}

// DeleteGroup deletes an activity group and all related records with ownership verification
// Only the creator, supervisors, or users with manage permission can delete
func (s *Service) DeleteGroup(ctx context.Context, id int64, requestingStaffID int64, hasManagePermission bool) error {
	// Resolve exactly once before permission and mutation checks. Managers keep
	// the historical idempotent-delete contract for a missing row, but return
	// immediately so a concurrently inserted template with the same sequence ID
	// can never be deleted after an absent preflight. Infrastructure failures
	// are never treated as absence.
	existingGroup, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		if isRepositoryNotFound(err) && hasManagePermission {
			return nil
		}
		if isRepositoryNotFound(err) {
			return &ActivityError{Op: "delete group", Err: ErrGroupNotFound}
		}
		return &ActivityError{Op: "delete group", Err: err}
	}
	if timetable.IsSystemActivityName(existingGroup.Name) {
		return &ActivityError{Op: "delete group", Err: ErrSystemActivityProtected}
	}
	if existingGroup.IsTemplate {
		return &ActivityError{Op: "delete group", Err: ErrTimetableTemplateProtected}
	}

	// Check if user can modify this activity before starting transaction
	canModify, err := s.CanModifyActivity(ctx, id, requestingStaffID, hasManagePermission)
	if err != nil {
		return &ActivityError{Op: opCheckPermissions, Err: err}
	}
	if !canModify {
		return &ActivityError{Op: "delete group", Err: ErrNotOwner}
	}

	// Delete all related records
	if err := deleteGroupEnrollments(ctx, s, id); err != nil {
		return &ActivityError{Op: "delete group transaction", Err: err}
	}

	if err := deleteGroupSupervisors(ctx, s, id); err != nil {
		return &ActivityError{Op: "delete group transaction", Err: err}
	}

	if err := deleteGroupSchedules(ctx, s, id); err != nil {
		return &ActivityError{Op: "delete group transaction", Err: err}
	}

	// Finally delete the group
	if err := s.groupRepo.Delete(ctx, id); err != nil {
		return &ActivityError{Op: "delete group transaction", Err: err}
	}

	return nil
}

// deleteGroupRows deletes every row FindByGroupID returns, one delete per
// row (per-row deletes keep the historical query pattern and repo hooks).
func deleteGroupRows[E interface{ GetID() any }](ctx context.Context, groupID int64, find func(context.Context, int64) ([]E, error), del func(context.Context, any) error) error {
	rows, err := find(ctx, groupID)
	if err != nil {
		return err
	}

	for _, row := range rows {
		if err := del(ctx, row.GetID()); err != nil {
			return err
		}
	}

	return nil
}

// deleteGroupEnrollments deletes all enrollments for a group
func deleteGroupEnrollments(ctx context.Context, service *Service, groupID int64) error {
	return deleteGroupRows(ctx, groupID, service.enrollmentRepo.FindByGroupID, service.enrollmentRepo.Delete)
}

// deleteGroupSupervisors deletes all supervisors for a group
func deleteGroupSupervisors(ctx context.Context, service *Service, groupID int64) error {
	return deleteGroupRows(ctx, groupID, service.supervisorRepo.FindByGroupID, service.supervisorRepo.Delete)
}

// deleteGroupSchedules deletes all schedules for a group
func deleteGroupSchedules(ctx context.Context, service *Service, groupID int64) error {
	return deleteGroupRows(ctx, groupID, service.scheduleRepo.FindByGroupID, service.scheduleRepo.Delete)
}

// ListGroups lists activity groups with optional filters
func (s *Service) ListGroups(ctx context.Context, query *activities.GroupListQuery) ([]*activities.Group, error) {
	filter := timetable.GroupFilter{}
	if query != nil {
		filter = timetable.GroupFilter{
			Name: query.Name, CategoryID: query.CategoryID, IsSystem: query.IsSystem, IDs: query.IDs,
		}
	}
	groups, err := s.timetable.ListGroups(ctx, filter)
	if err != nil {
		return nil, &ActivityError{Op: "list groups", Err: err}
	}
	return groupsFromOwner(groups), nil
}

// ListGroupsWithOccupancy returns all activity groups with their active session status
func (s *Service) ListGroupsWithOccupancy(ctx context.Context) ([]ActivityGroupWithOccupancy, error) {
	groups, err := s.groupRepo.ListWithCategory(ctx, nil)
	if err != nil {
		return nil, &ActivityError{Op: "list groups with occupancy", Err: err}
	}

	if len(groups) == 0 {
		return []ActivityGroupWithOccupancy{}, nil
	}

	// Collect all activity group IDs
	groupIDs := make([]int64, len(groups))
	for i, g := range groups {
		groupIDs[i] = g.ID
	}

	// Batch-fetch occupancy status
	occupiedMap, err := s.activeGroupRepo.GetOccupiedActivityGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, &ActivityError{Op: "get activity occupancy", Err: err}
	}

	// Combine
	result := make([]ActivityGroupWithOccupancy, len(groups))
	for i, g := range groups {
		result[i] = ActivityGroupWithOccupancy{
			Group:      g,
			IsOccupied: occupiedMap[g.ID],
		}
	}

	return result, nil
}

// FindByCategory finds all activity groups in a specific category
func (s *Service) FindByCategory(ctx context.Context, categoryID int64) ([]*activities.Group, error) {
	// First verify the category exists
	_, err := s.timetable.FindCategory(ctx, categoryID)
	if err != nil {
		return nil, categoryActivityError(opFindByCategory, err)
	}

	// Use the repository method
	groups, err := s.timetable.ListGroups(ctx, timetable.GroupFilter{CategoryID: &categoryID, OrderByName: true})
	if err != nil {
		return nil, &ActivityError{Op: opFindByCategory, Err: err}
	}

	return groupsFromOwner(groups), nil
}

// GetGroupWithDetails retrieves a group with its supervisors and schedules
func (s *Service) GetGroupWithDetails(ctx context.Context, id int64) (*activities.Group, []*activities.SupervisorPlanned, []*activities.Schedule, error) {
	// Get the group
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		// Convert "no rows" (bare or DatabaseError-wrapped) to our own error
		if isRepositoryNotFound(err) {
			return nil, nil, nil, &ActivityError{Op: opGetGroup, Err: ErrGroupNotFound}
		}
		return nil, nil, nil, &ActivityError{Op: opGetGroup, Err: err}
	}

	// Load the category if not already loaded.
	if group.Category == nil && group.CategoryID > 0 {
		category, err := s.timetable.FindCategory(ctx, group.CategoryID)
		if err != nil {
			return nil, nil, nil, &ActivityError{Op: opGetCategory, Err: err}
		}
		group.Category = categoryFromOwner(category)
	}

	supervisors, err := s.supervisorRepo.FindByGroupID(ctx, id)
	if err != nil {
		return nil, nil, nil, &ActivityError{Op: "get supervisors", Err: err}
	}

	// Get schedules
	schedules, err := s.scheduleRepo.FindByGroupID(ctx, id)
	if err != nil {
		return nil, nil, nil, &ActivityError{Op: "get schedules", Err: err}
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
	// Hold the group lock through the caller's tenant transaction so a
	// supervisor or creator cannot lose modification authority between this
	// check and the subsequent write.
	group, err := s.groupRepo.FindByIDForUpdate(ctx, groupID)
	if err != nil {
		if isRepositoryNotFound(err) {
			return false, &ActivityError{Op: opCheckPermissions, Err: ErrGroupNotFound}
		}
		return false, &ActivityError{Op: opCheckPermissions, Err: err}
	}

	// Check if user is the creator (system-created groups with NULL created_by are not owned by any staff)
	if group.CreatedBy != nil && *group.CreatedBy == staffID {
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
