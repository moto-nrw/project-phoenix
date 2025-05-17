package education

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// service implements the Education Service interface
type service struct {
	groupRepo        education.GroupRepository
	groupTeacherRepo education.GroupTeacherRepository
	substitutionRepo education.GroupSubstitutionRepository
	roomRepo         facilities.RoomRepository
	teacherRepo      users.TeacherRepository
	staffRepo        users.StaffRepository
	db               *bun.DB
	txHandler        *base.TxHandler
}

// NewService creates a new education service instance
func NewService(
	groupRepo education.GroupRepository,
	groupTeacherRepo education.GroupTeacherRepository,
	substitutionRepo education.GroupSubstitutionRepository,
	roomRepo facilities.RoomRepository,
	teacherRepo users.TeacherRepository,
	staffRepo users.StaffRepository,
	db *bun.DB,
) Service {
	return &service{
		groupRepo:        groupRepo,
		groupTeacherRepo: groupTeacherRepo,
		substitutionRepo: substitutionRepo,
		roomRepo:         roomRepo,
		teacherRepo:      teacherRepo,
		staffRepo:        staffRepo,
		db:               db,
		txHandler:        base.NewTxHandler(db),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *service) WithTx(tx bun.Tx) interface{} {
	// Get repositories with transaction if they implement the TransactionalRepository interface
	var groupRepo = s.groupRepo
	var groupTeacherRepo = s.groupTeacherRepo
	var substitutionRepo = s.substitutionRepo
	var roomRepo = s.roomRepo
	var teacherRepo = s.teacherRepo
	var staffRepo = s.staffRepo

	// Try to cast repositories to TransactionalRepository and apply the transaction
	if txRepo, ok := s.groupRepo.(base.TransactionalRepository); ok {
		groupRepo = txRepo.WithTx(tx).(education.GroupRepository)
	}
	if txRepo, ok := s.groupTeacherRepo.(base.TransactionalRepository); ok {
		groupTeacherRepo = txRepo.WithTx(tx).(education.GroupTeacherRepository)
	}
	if txRepo, ok := s.substitutionRepo.(base.TransactionalRepository); ok {
		substitutionRepo = txRepo.WithTx(tx).(education.GroupSubstitutionRepository)
	}
	if txRepo, ok := s.roomRepo.(base.TransactionalRepository); ok {
		roomRepo = txRepo.WithTx(tx).(facilities.RoomRepository)
	}
	if txRepo, ok := s.teacherRepo.(base.TransactionalRepository); ok {
		teacherRepo = txRepo.WithTx(tx).(users.TeacherRepository)
	}
	if txRepo, ok := s.staffRepo.(base.TransactionalRepository); ok {
		staffRepo = txRepo.WithTx(tx).(users.StaffRepository)
	}

	// Return a new service with the transaction
	return &service{
		groupRepo:        groupRepo,
		groupTeacherRepo: groupTeacherRepo,
		substitutionRepo: substitutionRepo,
		roomRepo:         roomRepo,
		teacherRepo:      teacherRepo,
		staffRepo:        staffRepo,
		db:               s.db,
		txHandler:        s.txHandler.WithTx(tx),
	}
}

// Group operations

// GetGroup retrieves a group by its ID
func (s *service) GetGroup(ctx context.Context, id int64) (*education.Group, error) {
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &EducationError{Op: "GetGroup", Err: ErrGroupNotFound}
	}
	return group, nil
}

// CreateGroup creates a new education group
func (s *service) CreateGroup(ctx context.Context, group *education.Group) error {
	// Validate group data
	if err := group.Validate(); err != nil {
		return &EducationError{Op: "CreateGroup", Err: err}
	}

	// Check if group with same name already exists
	existingGroup, err := s.groupRepo.FindByName(ctx, group.Name)
	if err == nil && existingGroup != nil && existingGroup.ID > 0 {
		return &EducationError{Op: "CreateGroup", Err: ErrDuplicateGroup}
	}

	// If room ID is specified, verify the room exists
	if group.RoomID != nil && *group.RoomID > 0 {
		room, err := s.roomRepo.FindByID(ctx, *group.RoomID)
		if err != nil {
			return &EducationError{Op: "CreateGroup", Err: ErrRoomNotFound}
		}
		group.Room = room
	}

	// Create the group
	if err := s.groupRepo.Create(ctx, group); err != nil {
		return &EducationError{Op: "CreateGroup", Err: err}
	}

	return nil
}

// UpdateGroup updates an existing education group
func (s *service) UpdateGroup(ctx context.Context, group *education.Group) error {
	// Validate group data
	if err := group.Validate(); err != nil {
		return &EducationError{Op: "UpdateGroup", Err: err}
	}

	// Check if group exists
	existingGroup, err := s.groupRepo.FindByID(ctx, group.ID)
	if err != nil {
		return &EducationError{Op: "UpdateGroup", Err: ErrGroupNotFound}
	}

	// If name has changed, check for duplicates
	if existingGroup.Name != group.Name {
		nameGroup, err := s.groupRepo.FindByName(ctx, group.Name)
		if err == nil && nameGroup != nil && nameGroup.ID != group.ID {
			return &EducationError{Op: "UpdateGroup", Err: ErrDuplicateGroup}
		}
	}

	// If room ID has changed, verify the new room exists
	if (existingGroup.RoomID == nil && group.RoomID != nil) ||
		(existingGroup.RoomID != nil && group.RoomID == nil) ||
		(existingGroup.RoomID != nil && group.RoomID != nil && *existingGroup.RoomID != *group.RoomID) {
		if group.RoomID != nil && *group.RoomID > 0 {
			room, err := s.roomRepo.FindByID(ctx, *group.RoomID)
			if err != nil {
				return &EducationError{Op: "UpdateGroup", Err: ErrRoomNotFound}
			}
			group.Room = room
		} else {
			group.Room = nil
		}
	}

	// Update the group
	if err := s.groupRepo.Update(ctx, group); err != nil {
		return &EducationError{Op: "UpdateGroup", Err: err}
	}

	return nil
}

// DeleteGroup deletes an education group by ID
func (s *service) DeleteGroup(ctx context.Context, id int64) error {
	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		return &EducationError{Op: "DeleteGroup", Err: ErrGroupNotFound}
	}

	// Execute in transaction using txHandler
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(Service)

		// Delete all group-teacher relationships
		groupTeachers, err := txService.GetGroupTeachers(ctx, id)
		if err == nil && len(groupTeachers) > 0 {
			for _, teacher := range groupTeachers {
				if err := txService.RemoveTeacherFromGroup(ctx, id, teacher.ID); err != nil {
					return err
				}
			}
		}

		// Delete all substitutions for this group
		substitutions, err := txService.GetActiveGroupSubstitutions(ctx, id, time.Now())
		if err == nil && len(substitutions) > 0 {
			for _, sub := range substitutions {
				if err := txService.DeleteSubstitution(ctx, sub.ID); err != nil {
					return err
				}
			}
		}

		// Delete the group via repository (as we don't have a dedicated delete method in service)
		return s.groupRepo.Delete(ctx, id)
	})

	if err != nil {
		return &EducationError{Op: "DeleteGroup", Err: err}
	}

	return nil
}

// ListGroups retrieves groups with optional filtering
func (s *service) ListGroups(ctx context.Context, options *base.QueryOptions) ([]*education.Group, error) {
	groups, err := s.groupRepo.List(ctx, nil)
	if err != nil {
		return nil, &EducationError{Op: "ListGroups", Err: err}
	}
	return groups, nil
}

// FindGroupByName finds a group by its name
func (s *service) FindGroupByName(ctx context.Context, name string) (*education.Group, error) {
	group, err := s.groupRepo.FindByName(ctx, name)
	if err != nil {
		return nil, &EducationError{Op: "FindGroupByName", Err: ErrGroupNotFound}
	}
	return group, nil
}

// FindGroupsByRoom finds all groups assigned to a specific room
func (s *service) FindGroupsByRoom(ctx context.Context, roomID int64) ([]*education.Group, error) {
	// Verify room exists
	_, err := s.roomRepo.FindByID(ctx, roomID)
	if err != nil {
		return nil, &EducationError{Op: "FindGroupsByRoom", Err: ErrRoomNotFound}
	}

	groups, err := s.groupRepo.FindByRoom(ctx, roomID)
	if err != nil {
		return nil, &EducationError{Op: "FindGroupsByRoom", Err: err}
	}
	return groups, nil
}

// FindGroupWithRoom retrieves a group with its associated room
func (s *service) FindGroupWithRoom(ctx context.Context, groupID int64) (*education.Group, error) {
	group, err := s.groupRepo.FindWithRoom(ctx, groupID)
	if err != nil {
		return nil, &EducationError{Op: "FindGroupWithRoom", Err: ErrGroupNotFound}
	}
	return group, nil
}

// AssignRoomToGroup assigns a room to a group
func (s *service) AssignRoomToGroup(ctx context.Context, groupID, roomID int64) error {
	// Verify group exists
	group, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return &EducationError{Op: "AssignRoomToGroup", Err: ErrGroupNotFound}
	}

	// Verify room exists
	room, err := s.roomRepo.FindByID(ctx, roomID)
	if err != nil {
		return &EducationError{Op: "AssignRoomToGroup", Err: ErrRoomNotFound}
	}

	// Update group's room
	group.SetRoom(room)
	if err := s.groupRepo.Update(ctx, group); err != nil {
		return &EducationError{Op: "AssignRoomToGroup", Err: err}
	}

	return nil
}

// RemoveRoomFromGroup removes a room assignment from a group
func (s *service) RemoveRoomFromGroup(ctx context.Context, groupID int64) error {
	// Verify group exists
	group, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return &EducationError{Op: "RemoveRoomFromGroup", Err: ErrGroupNotFound}
	}

	// Remove room assignment
	group.SetRoom(nil)
	if err := s.groupRepo.Update(ctx, group); err != nil {
		return &EducationError{Op: "RemoveRoomFromGroup", Err: err}
	}

	return nil
}

// Group-Teacher operations

// AddTeacherToGroup adds a teacher to a group
func (s *service) AddTeacherToGroup(ctx context.Context, groupID, teacherID int64) error {
	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return &EducationError{Op: "AddTeacherToGroup", Err: ErrGroupNotFound}
	}

	// Verify teacher exists
	teacher, err := s.teacherRepo.FindByID(ctx, teacherID)
	if err != nil {
		return &EducationError{Op: "AddTeacherToGroup", Err: ErrTeacherNotFound}
	}

	// Check if relationship already exists
	relations, err := s.groupTeacherRepo.FindByGroup(ctx, groupID)
	if err == nil {
		for _, rel := range relations {
			if rel.TeacherID == teacherID {
				return &EducationError{Op: "AddTeacherToGroup", Err: ErrDuplicateTeacherInGroup}
			}
		}
	}

	// Create group-teacher relationship
	groupTeacher := &education.GroupTeacher{
		GroupID:   groupID,
		TeacherID: teacher.ID,
	}

	if err := s.groupTeacherRepo.Create(ctx, groupTeacher); err != nil {
		return &EducationError{Op: "AddTeacherToGroup", Err: err}
	}

	return nil
}

// RemoveTeacherFromGroup removes a teacher from a group
func (s *service) RemoveTeacherFromGroup(ctx context.Context, groupID, teacherID int64) error {
	// Find the group-teacher relationship
	relations, err := s.groupTeacherRepo.FindByGroup(ctx, groupID)
	if err != nil {
		return &EducationError{Op: "RemoveTeacherFromGroup", Err: ErrGroupTeacherNotFound}
	}

	// Find the specific relationship to delete
	var relationID int64
	found := false
	for _, rel := range relations {
		if rel.TeacherID == teacherID {
			relationID = rel.ID
			found = true
			break
		}
	}

	if !found {
		return &EducationError{Op: "RemoveTeacherFromGroup", Err: ErrGroupTeacherNotFound}
	}

	// Delete the relationship
	if err := s.groupTeacherRepo.Delete(ctx, relationID); err != nil {
		return &EducationError{Op: "RemoveTeacherFromGroup", Err: err}
	}

	return nil
}

// GetGroupTeachers gets all teachers for a group
func (s *service) GetGroupTeachers(ctx context.Context, groupID int64) ([]*users.Teacher, error) {
	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, &EducationError{Op: "GetGroupTeachers", Err: ErrGroupNotFound}
	}

	// Find all group-teacher relationships
	relations, err := s.groupTeacherRepo.FindByGroup(ctx, groupID)
	if err != nil {
		return []*users.Teacher{}, nil
	}

	// Extract teacher IDs
	teacherIDs := make([]int64, 0, len(relations))
	for _, rel := range relations {
		teacherIDs = append(teacherIDs, rel.TeacherID)
	}

	if len(teacherIDs) == 0 {
		return []*users.Teacher{}, nil
	}

	// Build query options with an IN filter for teacher IDs
	options := base.NewQueryOptions()
	filter := base.NewFilter()

	// Convert int64 slice to []interface{}
	interfaceIDs := make([]interface{}, len(teacherIDs))
	for i, id := range teacherIDs {
		interfaceIDs[i] = id
	}

	filter.In("id", interfaceIDs...)
	options.Filter = filter

	// Get teachers
	// Note: This assumes the TeacherRepository has a List method with QueryOptions
	// Similar to what we have in the GroupRepository
	teachers, err := s.teacherRepo.List(ctx, nil)
	if err != nil {
		return nil, &EducationError{Op: "GetGroupTeachers", Err: err}
	}

	// Filter teachers by IDs (fallback if the List method doesn't support complex filtering)
	var filteredTeachers []*users.Teacher
	idMap := make(map[int64]bool)
	for _, id := range teacherIDs {
		idMap[id] = true
	}

	for _, teacher := range teachers {
		if idMap[teacher.ID] {
			filteredTeachers = append(filteredTeachers, teacher)
		}
	}

	return filteredTeachers, nil
}

// GetTeacherGroups gets all groups for a teacher
func (s *service) GetTeacherGroups(ctx context.Context, teacherID int64) ([]*education.Group, error) {
	// Verify teacher exists
	_, err := s.teacherRepo.FindByID(ctx, teacherID)
	if err != nil {
		return nil, &EducationError{Op: "GetTeacherGroups", Err: ErrTeacherNotFound}
	}

	// Get all groups for this teacher
	groups, err := s.groupRepo.FindByTeacher(ctx, teacherID)
	if err != nil {
		return nil, &EducationError{Op: "GetTeacherGroups", Err: err}
	}

	return groups, nil
}

// Substitution operations

// CreateSubstitution creates a new substitution
func (s *service) CreateSubstitution(ctx context.Context, substitution *education.GroupSubstitution) error {
	// Validate substitution data
	if err := substitution.Validate(); err != nil {
		return &EducationError{Op: "CreateSubstitution", Err: err}
	}

	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, substitution.GroupID)
	if err != nil {
		return &EducationError{Op: "CreateSubstitution", Err: ErrGroupNotFound}
	}

	// Verify regular staff exists
	_, err = s.staffRepo.FindByID(ctx, substitution.RegularStaffID)
	if err != nil {
		return &EducationError{Op: "CreateSubstitution", Err: ErrTeacherNotFound}
	}

	// Verify substitute staff exists
	_, err = s.staffRepo.FindByID(ctx, substitution.SubstituteStaffID)
	if err != nil {
		return &EducationError{Op: "CreateSubstitution", Err: ErrTeacherNotFound}
	}

	// Check for conflicting substitutions (overlapping dates for same substitute)
	conflicts, err := s.substitutionRepo.FindOverlapping(ctx, substitution.SubstituteStaffID,
		substitution.StartDate, substitution.EndDate)
	if err == nil && len(conflicts) > 0 {
		return &EducationError{Op: "CreateSubstitution", Err: ErrSubstitutionConflict}
	}

	// Create the substitution
	if err := s.substitutionRepo.Create(ctx, substitution); err != nil {
		return &EducationError{Op: "CreateSubstitution", Err: err}
	}

	return nil
}

// UpdateSubstitution updates an existing substitution
func (s *service) UpdateSubstitution(ctx context.Context, substitution *education.GroupSubstitution) error {
	// Validate substitution data
	if err := substitution.Validate(); err != nil {
		return &EducationError{Op: "UpdateSubstitution", Err: err}
	}

	// Verify substitution exists
	_, err := s.substitutionRepo.FindByID(ctx, substitution.ID)
	if err != nil {
		return &EducationError{Op: "UpdateSubstitution", Err: ErrSubstitutionNotFound}
	}

	// Verify group exists
	_, err = s.groupRepo.FindByID(ctx, substitution.GroupID)
	if err != nil {
		return &EducationError{Op: "UpdateSubstitution", Err: ErrGroupNotFound}
	}

	// Verify regular staff exists
	_, err = s.staffRepo.FindByID(ctx, substitution.RegularStaffID)
	if err != nil {
		return &EducationError{Op: "UpdateSubstitution", Err: ErrTeacherNotFound}
	}

	// Verify substitute staff exists
	_, err = s.staffRepo.FindByID(ctx, substitution.SubstituteStaffID)
	if err != nil {
		return &EducationError{Op: "UpdateSubstitution", Err: ErrTeacherNotFound}
	}

	// Check for conflicting substitutions (excluding this one)
	conflicts, err := s.substitutionRepo.FindOverlapping(ctx, substitution.SubstituteStaffID,
		substitution.StartDate, substitution.EndDate)
	if err == nil {
		for _, conflict := range conflicts {
			if conflict.ID != substitution.ID {
				return &EducationError{Op: "UpdateSubstitution", Err: ErrSubstitutionConflict}
			}
		}
	}

	// Update the substitution
	if err := s.substitutionRepo.Update(ctx, substitution); err != nil {
		return &EducationError{Op: "UpdateSubstitution", Err: err}
	}

	return nil
}

// DeleteSubstitution deletes a substitution by ID
func (s *service) DeleteSubstitution(ctx context.Context, id int64) error {
	// Verify substitution exists
	_, err := s.substitutionRepo.FindByID(ctx, id)
	if err != nil {
		return &EducationError{Op: "DeleteSubstitution", Err: ErrSubstitutionNotFound}
	}

	// Delete the substitution
	if err := s.substitutionRepo.Delete(ctx, id); err != nil {
		return &EducationError{Op: "DeleteSubstitution", Err: err}
	}

	return nil
}

// GetSubstitution retrieves a substitution by ID
func (s *service) GetSubstitution(ctx context.Context, id int64) (*education.GroupSubstitution, error) {
	substitution, err := s.substitutionRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &EducationError{Op: "GetSubstitution", Err: ErrSubstitutionNotFound}
	}
	return substitution, nil
}

// ListSubstitutions retrieves substitutions with optional filtering
func (s *service) ListSubstitutions(ctx context.Context, options *base.QueryOptions) ([]*education.GroupSubstitution, error) {
	substitutions, err := s.substitutionRepo.List(ctx, nil)
	if err != nil {
		return nil, &EducationError{Op: "ListSubstitutions", Err: err}
	}
	return substitutions, nil
}

// GetActiveSubstitutions gets all active substitutions for a specific date
func (s *service) GetActiveSubstitutions(ctx context.Context, date time.Time) ([]*education.GroupSubstitution, error) {
	substitutions, err := s.substitutionRepo.FindActive(ctx, date)
	if err != nil {
		return nil, &EducationError{Op: "GetActiveSubstitutions", Err: err}
	}
	return substitutions, nil
}

// GetActiveGroupSubstitutions gets active substitutions for a specific group and date
func (s *service) GetActiveGroupSubstitutions(ctx context.Context, groupID int64, date time.Time) ([]*education.GroupSubstitution, error) {
	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, &EducationError{Op: "GetActiveGroupSubstitutions", Err: ErrGroupNotFound}
	}

	substitutions, err := s.substitutionRepo.FindActiveByGroup(ctx, groupID, date)
	if err != nil {
		return nil, &EducationError{Op: "GetActiveGroupSubstitutions", Err: err}
	}
	return substitutions, nil
}

// GetStaffSubstitutions gets all substitutions for a staff member
func (s *service) GetStaffSubstitutions(ctx context.Context, staffID int64, asRegular bool) ([]*education.GroupSubstitution, error) {
	// Verify staff exists
	_, err := s.staffRepo.FindByID(ctx, staffID)
	if err != nil {
		return nil, &EducationError{Op: "GetStaffSubstitutions", Err: ErrTeacherNotFound}
	}

	var substitutions []*education.GroupSubstitution
	var repoErr error

	if asRegular {
		substitutions, repoErr = s.substitutionRepo.FindByRegularStaff(ctx, staffID)
	} else {
		substitutions, repoErr = s.substitutionRepo.FindBySubstituteStaff(ctx, staffID)
	}

	if repoErr != nil {
		return nil, &EducationError{Op: "GetStaffSubstitutions", Err: repoErr}
	}

	return substitutions, nil
}

// CheckSubstitutionConflicts checks for conflicting substitutions for a staff member
func (s *service) CheckSubstitutionConflicts(ctx context.Context, staffID int64, startDate, endDate time.Time) ([]*education.GroupSubstitution, error) {
	// Verify staff exists
	_, err := s.staffRepo.FindByID(ctx, staffID)
	if err != nil {
		return nil, &EducationError{Op: "CheckSubstitutionConflicts", Err: ErrTeacherNotFound}
	}

	// Validate date range
	if startDate.After(endDate) {
		return nil, &EducationError{Op: "CheckSubstitutionConflicts", Err: ErrInvalidDateRange}
	}

	// Check for conflicts
	conflicts, err := s.substitutionRepo.FindOverlapping(ctx, staffID, startDate, endDate)
	if err != nil {
		return nil, &EducationError{Op: "CheckSubstitutionConflicts", Err: err}
	}

	return conflicts, nil
}
