package education

import (
	"context"
	"log/slog"
	"maps"
	"slices"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// service implements the Education Service interface
type service struct {
	groupRepo        education.GroupRepository
	groupTeacherRepo education.GroupTeacherRepository
	substitutionRepo education.GroupSubstitutionRepository
	roomRepo         facilities.RoomRepository
	teacherRepo      users.TeacherRepository
	staffRepo        users.StaffRepository
	studentRepo      users.StudentRepository

	// broadcaster announces group_access_changed after a write to either table
	// that decides group access (#2084). Optional: services constructed without
	// one (tests, CLI) simply emit nothing, they never fail the write.
	broadcaster realtime.Broadcaster
}

// SetBroadcaster wires the SSE hub after construction, matching the
// duck-typed SetBroadcaster block the service factory already uses for every
// other broadcasting service.
func (s *service) SetBroadcaster(b realtime.Broadcaster) { s.broadcaster = b }

// announceGroupAccessChanged queues the tenant-wide invalidation for a write
// that changed who may open which group. Education writes funnel through here,
// so a cascade (DeleteGroup drops both) announces itself for free. Staff
// offboarding also uses the shared realtime helper after its direct cleanup.
func (s *service) announceGroupAccessChanged(ctx context.Context, source string) {
	realtime.QueueGroupAccessChanged(ctx, s.broadcaster, nil, source)
}

// NewService creates a new education service instance
func NewService(
	groupRepo education.GroupRepository,
	groupTeacherRepo education.GroupTeacherRepository,
	substitutionRepo education.GroupSubstitutionRepository,
	roomRepo facilities.RoomRepository,
	teacherRepo users.TeacherRepository,
	staffRepo users.StaffRepository,
	studentRepo users.StudentRepository,
) Service {
	return &service{
		groupRepo:        groupRepo,
		groupTeacherRepo: groupTeacherRepo,
		substitutionRepo: substitutionRepo,
		roomRepo:         roomRepo,
		teacherRepo:      teacherRepo,
		staffRepo:        staffRepo,
		studentRepo:      studentRepo,
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

// GetGroupsByIDs retrieves multiple groups by their IDs in a single query
func (s *service) GetGroupsByIDs(ctx context.Context, ids []int64) (map[int64]*education.Group, error) {
	if len(ids) == 0 {
		return make(map[int64]*education.Group), nil
	}

	groups, err := s.groupRepo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, &EducationError{Op: "GetGroupsByIDs", Err: err}
	}

	return groups, nil
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
	group.SetTenantID(tenant.FromContext(ctx))
	if err := s.groupRepo.Create(ctx, group); err != nil {
		return &EducationError{Op: "CreateGroup", Err: err}
	}

	return nil
}

// UpdateGroup updates an existing education group
func (s *service) UpdateGroup(ctx context.Context, group *education.Group) error {
	if err := group.Validate(); err != nil {
		return &EducationError{Op: "UpdateGroup", Err: err}
	}

	existingGroup, err := s.groupRepo.FindByID(ctx, group.ID)
	if err != nil {
		return &EducationError{Op: "UpdateGroup", Err: ErrGroupNotFound}
	}

	if err := s.checkGroupNameUnique(ctx, existingGroup, group); err != nil {
		return err
	}

	if err := s.validateAndSetRoom(ctx, existingGroup, group); err != nil {
		return err
	}

	if err := s.groupRepo.Update(ctx, group); err != nil {
		return &EducationError{Op: "UpdateGroup", Err: err}
	}

	return nil
}

// checkGroupNameUnique checks if name changed and validates no duplicates
func (s *service) checkGroupNameUnique(ctx context.Context, existing, updated *education.Group) error {
	if existing.Name == updated.Name {
		return nil
	}

	nameGroup, err := s.groupRepo.FindByName(ctx, updated.Name)
	if err == nil && nameGroup != nil && nameGroup.ID != updated.ID {
		return &EducationError{Op: "UpdateGroup", Err: ErrDuplicateGroup}
	}

	return nil
}

// validateAndSetRoom validates room change and sets room reference
func (s *service) validateAndSetRoom(ctx context.Context, existing, updated *education.Group) error {
	if !roomIDHasChanged(existing.RoomID, updated.RoomID) {
		return nil
	}

	if updated.RoomID != nil && *updated.RoomID > 0 {
		room, err := s.roomRepo.FindByID(ctx, *updated.RoomID)
		if err != nil {
			return &EducationError{Op: "UpdateGroup", Err: ErrRoomNotFound}
		}
		updated.Room = room
	} else {
		updated.Room = nil
	}

	return nil
}

// roomIDHasChanged checks if room ID has changed (handles nil comparisons)
func roomIDHasChanged(oldRoomID, newRoomID *int64) bool {
	if oldRoomID == nil && newRoomID != nil {
		return true
	}
	if oldRoomID != nil && newRoomID == nil {
		return true
	}
	if oldRoomID != nil && newRoomID != nil && *oldRoomID != *newRoomID {
		return true
	}
	return false
}

// DeleteGroup deletes an education group by ID
func (s *service) DeleteGroup(ctx context.Context, id int64) error {
	if _, err := s.groupRepo.FindByID(ctx, id); err != nil {
		return &EducationError{Op: "DeleteGroup", Err: ErrGroupNotFound}
	}

	// Best-effort pre-check: students with group_id would lose their group (SET NULL).
	// The real protection is this check; the DB allows the delete but silently orphans students.
	counts, preCheckErr := s.studentRepo.CountByGroupIDs(ctx, []int64{id})
	if preCheckErr != nil {
		slog.Warn("group_delete_precheck_failed",
			"group_id", id,
			"error", preCheckErr.Error(),
		)
	} else if count, ok := counts[id]; ok && count > 0 {
		return &EducationError{Op: "DeleteGroup", Err: ErrGroupHasStudents}
	}

	if err := deleteGroupTeacherRelations(ctx, s, id); err != nil {
		return &EducationError{Op: "DeleteGroup", Err: err}
	}

	if err := deleteGroupSubstitutions(ctx, s, id); err != nil {
		return &EducationError{Op: "DeleteGroup", Err: err}
	}

	if err := s.groupRepo.Delete(ctx, id); err != nil {
		return &EducationError{Op: "DeleteGroup", Err: err}
	}

	// Even an empty group changes the structural group list. Per-link cleanup
	// emits above only when leaders or active substitutions exist, so it cannot
	// be the sole signal for immutable group caches.
	s.announceGroupAccessChanged(ctx, "group_delete")
	return nil
}

// deleteGroupTeacherRelations deletes all teacher relationships for a group
func deleteGroupTeacherRelations(ctx context.Context, service Service, groupID int64) error {
	groupTeachers, err := service.GetGroupTeachers(ctx, groupID)
	if err != nil || len(groupTeachers) == 0 {
		return nil
	}

	for _, teacher := range groupTeachers {
		if err := service.RemoveTeacherFromGroup(ctx, groupID, teacher.ID); err != nil {
			return err
		}
	}

	return nil
}

// deleteGroupSubstitutions deletes all substitutions for a group
func deleteGroupSubstitutions(ctx context.Context, service Service, groupID int64) error {
	substitutions, err := service.GetActiveGroupSubstitutions(ctx, groupID, timezone.TodayDate())
	if err != nil || len(substitutions) == 0 {
		return nil
	}

	for _, sub := range substitutions {
		if err := service.DeleteSubstitution(ctx, sub.ID); err != nil {
			return err
		}
	}

	return nil
}

// ListGroups retrieves groups with optional filtering
func (s *service) ListGroups(ctx context.Context, options *base.QueryOptions) ([]*education.Group, error) {
	// Now we can directly use the modern ListWithOptions method
	groups, err := s.groupRepo.ListWithOptions(ctx, options)
	if err != nil {
		return nil, &EducationError{Op: "ListGroups", Err: err}
	}
	return groups, nil
}

// CountGroups counts groups matching the query options (filters only, no pagination)
func (s *service) CountGroups(ctx context.Context, options *base.QueryOptions) (int, error) {
	count, err := s.groupRepo.CountWithOptions(ctx, options)
	if err != nil {
		return 0, &EducationError{Op: "CountGroups", Err: err}
	}
	return count, nil
}

// FindGroupWithRoom retrieves a group with its associated room
func (s *service) FindGroupWithRoom(ctx context.Context, groupID int64) (*education.Group, error) {
	group, err := s.groupRepo.FindWithRoom(ctx, groupID)
	if err != nil {
		return nil, &EducationError{Op: "FindGroupWithRoom", Err: ErrGroupNotFound}
	}
	return group, nil
}

// GetGroupsWithRoomsByIDs bulk-loads groups with their room relation preloaded.
func (s *service) GetGroupsWithRoomsByIDs(ctx context.Context, ids []int64) (map[int64]*education.Group, error) {
	groups, err := s.groupRepo.FindByIDsWithRooms(ctx, ids)
	if err != nil {
		return nil, &EducationError{Op: "GetGroupsWithRoomsByIDs", Err: err}
	}
	return groups, nil
}

// Group-Teacher operations

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

	s.announceGroupAccessChanged(ctx, "group_teacher_remove")
	return nil
}

// UpdateGroupTeachers updates the teacher assignments for a group
func (s *service) UpdateGroupTeachers(ctx context.Context, groupID int64, teacherIDs []int64) error {
	if _, err := s.groupRepo.FindByID(ctx, groupID); err != nil {
		return &EducationError{Op: "UpdateGroupTeachers", Err: ErrGroupNotFound}
	}

	currentRelations, err := s.groupTeacherRepo.FindByGroup(ctx, groupID)
	if err != nil {
		return &EducationError{Op: "UpdateGroupTeachers", Err: err}
	}

	currentTeacherIDs, newTeacherIDs := buildTeacherIDMaps(currentRelations, teacherIDs)

	removed, err := s.removeObsoleteTeachers(ctx, currentTeacherIDs, newTeacherIDs)
	if err != nil {
		return err
	}

	added, err := s.addNewTeachersToGroup(ctx, groupID, currentTeacherIDs, teacherIDs)
	if err != nil {
		return err
	}

	// Leadership decides group access exactly like a substitution does
	// (usercontext.GetMyGroups is the union of both tables), so an admin
	// adding a colleague here must reach their open tab too.
	//
	// Only when the set ACTUALLY changed: the group form submits teacher_ids on
	// every save, so announcing unconditionally would make every client in the
	// school refetch its group list when an admin only renamed a group.
	if removed || added {
		s.announceGroupAccessChanged(ctx, "group_teachers")
	}
	return nil
}

// buildTeacherIDMaps builds maps for current and new teacher IDs
func buildTeacherIDMaps(currentRelations []*education.GroupTeacher, teacherIDs []int64) (map[int64]int64, map[int64]bool) {
	currentTeacherIDs := make(map[int64]int64)
	for _, rel := range currentRelations {
		currentTeacherIDs[rel.TeacherID] = rel.ID
	}

	newTeacherIDs := make(map[int64]bool)
	for _, teacherID := range teacherIDs {
		newTeacherIDs[teacherID] = true
	}

	return currentTeacherIDs, newTeacherIDs
}

// removeObsoleteTeachers removes teachers that are no longer in the assignment list
// removeObsoleteTeachers drops the links no longer in the submitted set and
// reports whether it removed any, so the caller only announces a real change.
func (s *service) removeObsoleteTeachers(ctx context.Context, currentTeacherIDs map[int64]int64, newTeacherIDs map[int64]bool) (bool, error) {
	removed := false
	for teacherID, relationID := range currentTeacherIDs {
		if !newTeacherIDs[teacherID] {
			if err := s.groupTeacherRepo.Delete(ctx, relationID); err != nil {
				return removed, &EducationError{Op: "UpdateGroupTeachers", Err: err}
			}
			removed = true
		}
	}
	return removed, nil
}

// addNewTeachersToGroup adds new teachers to the group
// addNewTeachersToGroup links the teachers not yet on the group and reports
// whether it added any, so the caller only announces a real change.
func (s *service) addNewTeachersToGroup(ctx context.Context, groupID int64, currentTeacherIDs map[int64]int64, teacherIDs []int64) (bool, error) {
	added := false
	for _, teacherID := range teacherIDs {
		if _, exists := currentTeacherIDs[teacherID]; !exists {
			if err := s.addTeacherToGroup(ctx, groupID, teacherID); err != nil {
				return added, err
			}
			added = true
		}
	}
	return added, nil
}

// addTeacherToGroup adds a single teacher to a group
func (s *service) addTeacherToGroup(ctx context.Context, groupID, teacherID int64) error {
	if _, err := s.teacherRepo.FindByID(ctx, teacherID); err != nil {
		return &EducationError{Op: "UpdateGroupTeachers", Err: ErrTeacherNotFound}
	}

	relation := &education.GroupTeacher{
		GroupID:   groupID,
		TeacherID: teacherID,
	}
	relation.SetTenantID(tenant.FromContext(ctx))

	if err := s.groupTeacherRepo.Create(ctx, relation); err != nil {
		return &EducationError{Op: "UpdateGroupTeachers", Err: err}
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

	teacherIDs := make([]int64, 0, len(relations))
	for _, rel := range relations {
		teacherIDs = append(teacherIDs, rel.TeacherID)
	}
	if len(teacherIDs) == 0 {
		return []*users.Teacher{}, nil
	}

	// Batch fetch teachers with staff+person in one query — same path as
	// GetTeachersForGroups; replaces the per-teacher N+1 enrichment.
	teachers, err := s.teacherRepo.FindWithStaffAndPersonByIDs(ctx, teacherIDs)
	if err != nil {
		return nil, &EducationError{Op: "GetGroupTeachers", Err: err}
	}
	return teachers, nil
}

// GetTeachersForGroups batch-loads all teachers for multiple groups in 2 queries
func (s *service) GetTeachersForGroups(ctx context.Context, groupIDs []int64) (map[int64][]*users.Teacher, error) {
	if len(groupIDs) == 0 {
		return make(map[int64][]*users.Teacher), nil
	}

	// 1. Batch fetch all group-teacher relationships (1 query)
	relations, err := s.groupTeacherRepo.FindByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, &EducationError{Op: "GetTeachersForGroups", Err: err}
	}

	// 2. Collect unique teacher IDs
	teacherIDSet := make(map[int64]bool)
	for _, rel := range relations {
		teacherIDSet[rel.TeacherID] = true
	}

	teacherIDs := slices.Collect(maps.Keys(teacherIDSet))

	if len(teacherIDs) == 0 {
		result := make(map[int64][]*users.Teacher)
		for _, gid := range groupIDs {
			result[gid] = []*users.Teacher{}
		}
		return result, nil
	}

	// 3. Batch fetch all teachers with staff+person (1 query)
	teachers, err := s.teacherRepo.FindWithStaffAndPersonByIDs(ctx, teacherIDs)
	if err != nil {
		return nil, &EducationError{Op: "GetTeachersForGroups", Err: err}
	}

	// 4. Build teacher lookup map
	teacherMap := make(map[int64]*users.Teacher, len(teachers))
	for _, t := range teachers {
		teacherMap[t.ID] = t
	}

	// 5. Build result: groupID -> []Teacher
	result := make(map[int64][]*users.Teacher, len(groupIDs))
	for _, rel := range relations {
		if teacher, ok := teacherMap[rel.TeacherID]; ok {
			result[rel.GroupID] = append(result[rel.GroupID], teacher)
		}
	}

	// Ensure all requested group IDs have an entry
	for _, gid := range groupIDs {
		if _, ok := result[gid]; !ok {
			result[gid] = []*users.Teacher{}
		}
	}

	return result, nil
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

	// Validate no backdating - start date must be today or in the future
	today := timezone.TodayDate()
	if substitution.StartDate.Before(today) {
		return &EducationError{Op: "CreateSubstitution", Err: ErrSubstitutionBackdated}
	}

	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, substitution.GroupID)
	if err != nil {
		return &EducationError{Op: "CreateSubstitution", Err: ErrGroupNotFound}
	}

	// Verify regular staff exists - only if RegularStaffID is provided
	if substitution.RegularStaffID != nil {
		_, err = s.staffRepo.FindByID(ctx, *substitution.RegularStaffID)
		if err != nil {
			return &EducationError{Op: "CreateSubstitution", Err: ErrTeacherNotFound}
		}
	}

	// Verify substitute staff exists
	_, err = s.staffRepo.FindByID(ctx, substitution.SubstituteStaffID)
	if err != nil {
		return &EducationError{Op: "CreateSubstitution", Err: ErrTeacherNotFound}
	}

	// Note: We intentionally allow staff members to have multiple overlapping substitutions.
	// This enables a staff member to supervise multiple groups simultaneously.

	// Create the substitution
	substitution.SetTenantID(tenant.FromContext(ctx))
	if err := s.substitutionRepo.Create(ctx, substitution); err != nil {
		return &EducationError{Op: "CreateSubstitution", Err: err}
	}

	s.announceGroupAccessChanged(ctx, "substitution_create")
	return nil
}

// UpdateSubstitution updates an existing substitution
func (s *service) UpdateSubstitution(ctx context.Context, substitution *education.GroupSubstitution) error {
	// Validate substitution data
	if err := substitution.Validate(); err != nil {
		return &EducationError{Op: "UpdateSubstitution", Err: err}
	}

	// Validate no backdating - start date must be today or in the future
	today := timezone.TodayDate()
	if substitution.StartDate.Before(today) {
		return &EducationError{Op: "UpdateSubstitution", Err: ErrSubstitutionBackdated}
	}

	// Verify substitution exists
	existing, err := s.substitutionRepo.FindByID(ctx, substitution.ID)
	if err != nil {
		return &EducationError{Op: "UpdateSubstitution", Err: ErrSubstitutionNotFound}
	}
	accessChanged := existing.GroupID != substitution.GroupID ||
		existing.SubstituteStaffID != substitution.SubstituteStaffID ||
		existing.StartDate != substitution.StartDate ||
		existing.EndDate != substitution.EndDate

	// Carry the stored tenant over the caller's model. Callers that decode a
	// JSON body into a bare model leave tenant_id at 0 (tenancy is a
	// server-side fact, never a client input), and the generic repository
	// Update writes EVERY column — the row would be rewritten with
	// tenant_id = 0 and RLS rejects it with "new row violates row-level
	// security policy". Reading it from the row we just loaded also means a
	// caller cannot move a substitution into another tenant by sending one.
	substitution.SetTenantID(existing.GetTenantID())
	// created_at is immutable server-owned metadata. The generic repository
	// writes every column, and a JSON PUT body leaves this field at its zero
	// value; bun would therefore apply the column default and reset it.
	substitution.CreatedAt = existing.CreatedAt

	// Verify group exists
	_, err = s.groupRepo.FindByID(ctx, substitution.GroupID)
	if err != nil {
		return &EducationError{Op: "UpdateSubstitution", Err: ErrGroupNotFound}
	}

	// Verify regular staff exists - only if RegularStaffID is provided
	if substitution.RegularStaffID != nil {
		_, err = s.staffRepo.FindByID(ctx, *substitution.RegularStaffID)
		if err != nil {
			return &EducationError{Op: "UpdateSubstitution", Err: ErrTeacherNotFound}
		}
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

	if accessChanged {
		s.announceGroupAccessChanged(ctx, "substitution_update")
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

	s.announceGroupAccessChanged(ctx, "substitution_delete")
	return nil
}

// GetSubstitution retrieves a substitution by ID
func (s *service) GetSubstitution(ctx context.Context, id int64) (*education.GroupSubstitution, error) {
	substitution, err := s.substitutionRepo.FindByIDWithRelations(ctx, id)
	if err != nil {
		return nil, &EducationError{Op: "GetSubstitution", Err: ErrSubstitutionNotFound}
	}
	return substitution, nil
}

// ListSubstitutions retrieves substitutions with optional filtering
func (s *service) ListSubstitutions(ctx context.Context, options *base.QueryOptions) ([]*education.GroupSubstitution, error) {
	// Now using the modern ListWithOptions method with relations loaded
	substitutions, err := s.substitutionRepo.ListWithRelations(ctx, options)
	if err != nil {
		return nil, &EducationError{Op: "ListSubstitutions", Err: err}
	}
	return substitutions, nil
}

// GetActiveSubstitutions gets all active substitutions for a specific date
func (s *service) GetActiveSubstitutions(ctx context.Context, date timezone.Date) ([]*education.GroupSubstitution, error) {
	substitutions, err := s.substitutionRepo.FindActiveWithRelations(ctx, date)
	if err != nil {
		return nil, &EducationError{Op: "GetActiveSubstitutions", Err: err}
	}
	return substitutions, nil
}

// GetActiveGroupSubstitutions gets active substitutions for a specific group and date
func (s *service) GetActiveGroupSubstitutions(ctx context.Context, groupID int64, date timezone.Date) ([]*education.GroupSubstitution, error) {
	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, &EducationError{Op: "GetActiveGroupSubstitutions", Err: ErrGroupNotFound}
	}

	substitutions, err := s.substitutionRepo.FindActiveByGroupWithRelations(ctx, groupID, date)
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
func (s *service) CheckSubstitutionConflicts(ctx context.Context, staffID int64, startDate, endDate timezone.Date) ([]*education.GroupSubstitution, error) {
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

// CreateGroupTransfer persists a group-transfer substitution WITHOUT the
// FindOverlapping conflict check: group transfers deliberately allow staff to
// hold multiple groups at once (issue #584: moved verbatim from api/groups).
func (s *service) CreateGroupTransfer(ctx context.Context, substitution *education.GroupSubstitution) error {
	if err := s.substitutionRepo.Create(ctx, substitution); err != nil {
		return err
	}

	s.announceGroupAccessChanged(ctx, "group_transfer")
	return nil
}
