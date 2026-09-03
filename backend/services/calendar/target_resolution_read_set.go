package calendar

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

type targetSelection struct {
	allStaff      bool
	allParents    bool
	staffIDs      []int64
	guardianIDs   []int64
	studentIDs    []int64
	groupIDs      []int64
	schoolClasses []string
}

type targetResolutionReadSet struct {
	reachableStaff    map[int64]bool
	activeGuardians   map[int64]bool
	linksByGuardian   map[int64][]*userModels.StudentGuardian
	linksByStudent    map[int64][]*userModels.StudentGuardian
	allSchoolStudents []*userModels.Student
	studentsByGroup   map[int64][]*userModels.Student
	studentsByClass   map[string][]*userModels.Student
}

func (s *service) loadTargetResolutionReadSet(ctx context.Context, targets []AppointmentTarget) (*targetResolutionReadSet, error) {
	selection, err := collectTargetSelection(targets)
	if err != nil {
		return nil, err
	}
	readSet := newTargetResolutionReadSet()
	if err := s.loadTargetStaff(ctx, selection, readSet); err != nil {
		return nil, err
	}
	if err := s.loadExplicitGuardianLinks(ctx, selection.guardianIDs, readSet); err != nil {
		return nil, err
	}
	groupStudents, err := s.loadTargetStudents(ctx, selection, readSet)
	if err != nil {
		return nil, err
	}
	studentIDs := targetStudentIDs(selection, readSet.allSchoolStudents, groupStudents, readSet.studentsByClass)
	if err := s.loadStudentGuardianLinks(ctx, selection.guardianIDs, studentIDs, readSet); err != nil {
		return nil, err
	}
	return readSet, nil
}

func newTargetResolutionReadSet() *targetResolutionReadSet {
	return &targetResolutionReadSet{
		reachableStaff:  make(map[int64]bool),
		activeGuardians: make(map[int64]bool),
		linksByGuardian: make(map[int64][]*userModels.StudentGuardian),
		linksByStudent:  make(map[int64][]*userModels.StudentGuardian),
		studentsByGroup: make(map[int64][]*userModels.Student),
		studentsByClass: make(map[string][]*userModels.Student),
	}
}

func (s *service) loadTargetStaff(ctx context.Context, selection targetSelection, readSet *targetResolutionReadSet) error {
	if selection.allStaff || len(selection.staffIDs) > 0 {
		ids := selection.staffIDs
		if selection.allStaff {
			ids = nil
		}
		reachable, err := s.cfg.StaffRepo.FindReachableCalendarStaffIDs(ctx, ids)
		if err != nil {
			return err
		}
		readSet.reachableStaff = reachable
	}
	return nil
}

func (s *service) loadExplicitGuardianLinks(ctx context.Context, guardianIDs []int64, readSet *targetResolutionReadSet) error {
	guardianLinks, err := s.cfg.StudentGuardianRepo.FindByGuardianProfileIDs(ctx, guardianIDs)
	if err != nil {
		return err
	}
	for _, link := range guardianLinks {
		readSet.linksByGuardian[link.GuardianProfileID] = append(readSet.linksByGuardian[link.GuardianProfileID], link)
	}
	return nil
}

func (s *service) loadTargetStudents(ctx context.Context, selection targetSelection, readSet *targetResolutionReadSet) ([]*userModels.Student, error) {
	if selection.allParents || len(selection.schoolClasses) > 0 {
		students, err := s.cfg.StudentRepo.List(ctx, map[string]any{"status": string(userModels.StudentStatusActive)})
		if err != nil {
			return nil, err
		}
		readSet.allSchoolStudents = students
		for _, student := range readSet.allSchoolStudents {
			className := normalizeCalendarClass(student.SchoolClass)
			readSet.studentsByClass[className] = append(readSet.studentsByClass[className], student)
		}
	}
	groupStudents, err := s.cfg.StudentRepo.FindByGroupIDs(ctx, selection.groupIDs)
	if err != nil {
		return nil, err
	}
	for _, student := range groupStudents {
		if student.GroupID != nil {
			readSet.studentsByGroup[*student.GroupID] = append(readSet.studentsByGroup[*student.GroupID], student)
		}
	}
	return groupStudents, nil
}

func (s *service) loadStudentGuardianLinks(ctx context.Context, explicitGuardianIDs, studentIDs []int64, readSet *targetResolutionReadSet) error {
	studentLinks, err := s.cfg.StudentGuardianRepo.FindByStudentIDs(ctx, studentIDs)
	if err != nil {
		return err
	}
	guardianIDs := append([]int64(nil), explicitGuardianIDs...)
	seenGuardians := int64SliceSet(guardianIDs)
	for _, link := range studentLinks {
		readSet.linksByStudent[link.StudentID] = append(readSet.linksByStudent[link.StudentID], link)
		if authorize.StudentGuardianHasPermission(link, authorize.GuardianPermissionPortalAccess) && !seenGuardians[link.GuardianProfileID] {
			seenGuardians[link.GuardianProfileID] = true
			guardianIDs = append(guardianIDs, link.GuardianProfileID)
		}
	}
	if len(guardianIDs) > 0 {
		profiles, err := s.cfg.GuardianProfileRepo.FindActivePortalProfilesByIDs(ctx, guardianIDs)
		if err != nil {
			return err
		}
		for id := range profiles {
			readSet.activeGuardians[id] = true
		}
	}
	return nil
}

func collectTargetSelection(targets []AppointmentTarget) (targetSelection, error) {
	collector := targetSelectionCollector{
		seenStaff:     make(map[int64]bool),
		seenGuardians: make(map[int64]bool),
		seenStudents:  make(map[int64]bool),
		seenGroups:    make(map[int64]bool),
		seenClasses:   make(map[string]bool),
	}
	for _, target := range targets {
		if err := collector.add(target); err != nil {
			return targetSelection{}, err
		}
	}
	return collector.selection, nil
}

type targetSelectionCollector struct {
	selection     targetSelection
	seenStaff     map[int64]bool
	seenGuardians map[int64]bool
	seenStudents  map[int64]bool
	seenGroups    map[int64]bool
	seenClasses   map[string]bool
}

func (c *targetSelectionCollector) add(target AppointmentTarget) error {
	switch target.Type {
	case calModels.TargetTypeAllStaff:
		c.selection.allStaff = true
	case calModels.TargetTypeAllSchoolParents:
		c.selection.allParents = true
	case calModels.TargetTypeParentsByClass:
		return c.addClass(target.Value)
	case calModels.TargetTypeStaff, calModels.TargetTypeGuardianProfile,
		calModels.TargetTypeParentsByStudent, calModels.TargetTypeParentsByGroup:
		return c.addID(target.Type, target.ID)
	default:
		return fmt.Errorf("%w: unknown target type %q", ErrInvalidRequest, target.Type)
	}
	return nil
}

func (c *targetSelectionCollector) addID(targetType string, id *int64) error {
	labels := map[string]string{
		calModels.TargetTypeStaff:            "staff",
		calModels.TargetTypeGuardianProfile:  "guardian",
		calModels.TargetTypeParentsByStudent: "student",
		calModels.TargetTypeParentsByGroup:   "group",
	}
	if id == nil || *id <= 0 {
		return fmt.Errorf("%w: %s target requires id", ErrInvalidRequest, labels[targetType])
	}
	switch targetType {
	case calModels.TargetTypeStaff:
		appendDistinctID(&c.selection.staffIDs, c.seenStaff, *id)
	case calModels.TargetTypeGuardianProfile:
		appendDistinctID(&c.selection.guardianIDs, c.seenGuardians, *id)
	case calModels.TargetTypeParentsByStudent:
		appendDistinctID(&c.selection.studentIDs, c.seenStudents, *id)
	case calModels.TargetTypeParentsByGroup:
		appendDistinctID(&c.selection.groupIDs, c.seenGroups, *id)
	}
	return nil
}

func (c *targetSelectionCollector) addClass(value *string) error {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fmt.Errorf("%w: class target requires value", ErrInvalidRequest)
	}
	className := normalizeCalendarClass(*value)
	if !c.seenClasses[className] {
		c.seenClasses[className] = true
		c.selection.schoolClasses = append(c.selection.schoolClasses, className)
	}
	return nil
}

// normalizeCalendarClass preserves the case-insensitive, trimmed identity
// previously enforced by StudentRepository.FindBySchoolClass's SQL predicate.
func normalizeCalendarClass(class string) string {
	return strings.ToLower(strings.TrimSpace(class))
}

func targetStudentIDs(
	selection targetSelection,
	allSchoolStudents, groupStudents []*userModels.Student,
	studentsByClass map[string][]*userModels.Student,
) []int64 {
	ids := append([]int64(nil), selection.studentIDs...)
	seen := int64SliceSet(ids)
	addActive := func(students []*userModels.Student) {
		for _, id := range activeStudentIDs(students) {
			appendDistinctID(&ids, seen, id)
		}
	}
	if selection.allParents {
		addActive(allSchoolStudents)
	}
	addActive(groupStudents)
	for _, className := range selection.schoolClasses {
		addActive(studentsByClass[className])
	}
	return ids
}

func appendDistinctID(ids *[]int64, seen map[int64]bool, id int64) {
	if !seen[id] {
		seen[id] = true
		*ids = append(*ids, id)
	}
}

func int64SliceSet(ids []int64) map[int64]bool {
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		seen[id] = true
	}
	return seen
}
