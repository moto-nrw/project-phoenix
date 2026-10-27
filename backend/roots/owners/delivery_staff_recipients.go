package owners

import (
	"context"
	"maps"
	"slices"
	"strings"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// deliveryStudentGroups names the education group of each reported child
// from People Directory.
type deliveryStudentGroups struct {
	students interface {
		FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.StudentState, error)
	}
}

func (g deliveryStudentGroups) StudentGroupIDs(ctx context.Context, studentIDs []int64) (map[int64]int64, error) {
	students, err := g.students.FindReadScopeByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	groups := make(map[int64]int64, len(students))
	for studentID, student := range students {
		if student != nil && student.GroupID != nil {
			groups[studentID] = *student.GroupID
		}
	}
	return groups, nil
}

// deliveryOnDutyStaff names today's staff on duty by Workforce's rule.
type deliveryOnDutyStaff struct {
	presence interface {
		WorkPresenceMap(context.Context) (map[int64]string, error)
	}
}

func (s deliveryOnDutyStaff) OnDutyStaffIDs(ctx context.Context) ([]int64, error) {
	presence, err := s.presence.WorkPresenceMap(ctx)
	if err != nil {
		return nil, err
	}
	return slices.Sorted(maps.Keys(workforce.OnDutyStaffIDs(presence))), nil
}

// deliverySchoolClassReach answers the school notice of an absence (#4792):
// the class of each child from People Directory and the accounts of its
// assigned Lehrkräfte (education.class_teachers, School Membership). The
// Schulverwaltung is added by Delivery from the subscribers holding
// class_day:all_classes.
type deliverySchoolClassReach struct {
	students interface {
		FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.StudentState, error)
	}
	assignments interface {
		ListClassAssignments(ctx context.Context, filter schoolmembership.ClassAssignmentFilter) ([]schoolmembership.ClassAssignment, error)
	}
	staff interface {
		ListAccountIDsByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]int64, error)
	}
}

func (r deliverySchoolClassReach) StudentClassTeachers(ctx context.Context, studentIDs []int64) (map[int64]deliveryContract.StudentClassTeachers, error) {
	students, err := r.students.FindReadScopeByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	classOf := make(map[int64]string, len(students))
	for studentID, student := range students {
		if student != nil && strings.TrimSpace(student.SchoolClass) != "" {
			classOf[studentID] = strings.TrimSpace(student.SchoolClass)
		}
	}
	accountsByClass, err := r.teacherAccounts(ctx, classOf)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]deliveryContract.StudentClassTeachers, len(classOf))
	for studentID, schoolClass := range classOf {
		result[studentID] = deliveryContract.StudentClassTeachers{SchoolClass: schoolClass, AccountIDs: accountsByClass[schoolClass]}
	}
	return result, nil
}

// teacherAccounts maps each class to the accounts of its assigned
// Lehrkräfte: one assignment read for all classes and one account read.
// School Membership compares classes trimmed and case-insensitive; the
// result is grouped back the same way (schoolstructure.NormalizeClass).
func (r deliverySchoolClassReach) teacherAccounts(ctx context.Context, classOf map[int64]string) (map[string][]int64, error) {
	keys := make([]string, 0, len(classOf))
	for _, schoolClass := range classOf {
		keys = append(keys, schoolstructure.NormalizeClass(schoolClass))
	}
	slices.Sort(keys)
	assignments, err := r.assignments.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{ClassKeys: slices.Compact(keys)})
	if err != nil {
		return nil, err
	}
	staffIDs := make([]int64, 0, len(assignments))
	for _, assignment := range assignments {
		staffIDs = append(staffIDs, assignment.StaffID)
	}
	accountOf, err := r.staff.ListAccountIDsByStaffIDs(ctx, staffIDs)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string][]int64, len(keys))
	for _, assignment := range assignments {
		if accountID, ok := accountOf[assignment.StaffID]; ok {
			key := schoolstructure.NormalizeClass(assignment.SchoolClass)
			byKey[key] = append(byKey[key], accountID)
		}
	}
	accountsByClass := make(map[string][]int64, len(classOf))
	for _, schoolClass := range classOf {
		accountsByClass[schoolClass] = byKey[schoolstructure.NormalizeClass(schoolClass)]
	}
	return accountsByClass, nil
}
