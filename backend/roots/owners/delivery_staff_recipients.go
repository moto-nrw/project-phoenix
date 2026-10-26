package owners

import (
	"context"
	"maps"
	"slices"
	"strings"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"

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
	classTeachers interface {
		List(ctx context.Context, filters map[string]any) ([]*schoolmembership.ClassAssignment, error)
	}
	staff interface {
		ListAccountIDsByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]int64, error)
	}
}

func (r deliverySchoolClassReach) StudentClassTeachers(ctx context.Context, studentIDs []int64) (map[int64]deliveryCompose.StudentClassTeachers, error) {
	students, err := r.students.FindReadScopeByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	classOf := make(map[int64]string, len(students))
	staffByClass := make(map[string][]int64)
	for studentID, student := range students {
		if student == nil {
			continue
		}
		schoolClass := strings.TrimSpace(student.SchoolClass)
		if schoolClass == "" {
			continue
		}
		classOf[studentID] = schoolClass
		if _, loaded := staffByClass[schoolClass]; loaded {
			continue
		}
		// School Membership compares classes trimmed and case-insensitive.
		assignments, err := r.classTeachers.List(ctx, map[string]any{"school_class": schoolClass})
		if err != nil {
			return nil, err
		}
		staffByClass[schoolClass] = []int64{}
		for _, assignment := range assignments {
			if assignment != nil {
				staffByClass[schoolClass] = append(staffByClass[schoolClass], assignment.StaffID)
			}
		}
	}
	accountOf, err := r.staff.ListAccountIDsByStaffIDs(ctx, slices.Concat(slices.Collect(maps.Values(staffByClass))...))
	if err != nil {
		return nil, err
	}
	result := make(map[int64]deliveryCompose.StudentClassTeachers, len(classOf))
	for studentID, schoolClass := range classOf {
		accounts := make([]int64, 0, len(staffByClass[schoolClass]))
		for _, staffID := range staffByClass[schoolClass] {
			if accountID, ok := accountOf[staffID]; ok {
				accounts = append(accounts, accountID)
			}
		}
		result[studentID] = deliveryCompose.StudentClassTeachers{SchoolClass: schoolClass, AccountIDs: accounts}
	}
	return result, nil
}
