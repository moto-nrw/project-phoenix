package education

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

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

	if err := s.removeObsoleteTeachers(ctx, currentTeacherIDs, newTeacherIDs); err != nil {
		return err
	}

	return s.addNewTeachersToGroup(ctx, groupID, currentTeacherIDs, teacherIDs)
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
func (s *service) removeObsoleteTeachers(ctx context.Context, currentTeacherIDs map[int64]int64, newTeacherIDs map[int64]bool) error {
	for teacherID, relationID := range currentTeacherIDs {
		if !newTeacherIDs[teacherID] {
			if err := s.groupTeacherRepo.Delete(ctx, relationID); err != nil {
				return &EducationError{Op: "UpdateGroupTeachers", Err: err}
			}
		}
	}
	return nil
}

// addNewTeachersToGroup adds new teachers to the group
func (s *service) addNewTeachersToGroup(ctx context.Context, groupID int64, currentTeacherIDs map[int64]int64, teacherIDs []int64) error {
	for _, teacherID := range teacherIDs {
		if _, exists := currentTeacherIDs[teacherID]; !exists {
			if err := s.addTeacherToGroup(ctx, groupID, teacherID); err != nil {
				return err
			}
		}
	}
	return nil
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
		return nil, &EducationError{Op: "GetGroupTeachers", Err: err}
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

	// Get teachers using the modern ListWithOptions method
	teachers, err := s.teacherRepo.ListWithOptions(ctx, options)
	if err != nil {
		return nil, &EducationError{Op: "GetGroupTeachers", Err: err}
	}

	// Always filter to ensure we only return teachers that were requested
	var filteredTeachers []*users.Teacher
	idMap := make(map[int64]bool)
	for _, id := range teacherIDs {
		idMap[id] = true
	}

	// Fetch staff and person data for each teacher
	for _, teacher := range teachers {
		if idMap[teacher.ID] {
			// Try to get teacher with staff and person data
			fullTeacher, err := s.teacherRepo.FindWithStaffAndPerson(ctx, teacher.ID)
			if err == nil {
				filteredTeachers = append(filteredTeachers, fullTeacher)
			} else {
				// If fetch fails, use teacher without staff/person data
				filteredTeachers = append(filteredTeachers, teacher)
			}
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
