package active

import (
	"context"
	"errors"
	"sort"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

type statusDayOverviewPeople interface {
	GetStudentsByGroupIDs(ctx context.Context, groupIDs []int64) ([]*userModels.Student, error)
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Person, error)
}

type StatusDayOverviewEntry struct {
	StatusDay *activeModels.StudentStatusDay
	Student   *userModels.Student
	Person    *userModels.Person
	Group     *educationModels.Group
}

// GetOverview loads and assembles the absence rows for the authorized groups.
// Enrollment is evaluated on each status-day date; lifecycle status is used
// only for legacy students without enrollment bounds.
func (s *StudentStatusDayService) GetOverview(ctx context.Context, groups []*educationModels.Group, from, to timezone.Date) ([]StatusDayOverviewEntry, error) {
	if s.people == nil {
		return nil, errors.New("student status day overview people service is not configured")
	}
	groupIDs, groupsByID := indexOverviewGroups(groups)
	students, err := s.people.GetStudentsByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	studentIDs, personIDs, studentsByID := indexOverviewStudents(students)
	rows, err := s.repo.FindActiveByStudentIDsAndDateRange(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	persons, err := s.people.GetByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	return assembleStatusDayOverview(rows, studentsByID, persons, groupsByID), nil
}

func indexOverviewGroups(groups []*educationModels.Group) ([]int64, map[int64]*educationModels.Group) {
	ids := make([]int64, 0, len(groups))
	byID := make(map[int64]*educationModels.Group, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
		byID[group.ID] = group
	}
	return ids, byID
}

func indexOverviewStudents(students []*userModels.Student) ([]int64, []int64, map[int64]*userModels.Student) {
	studentIDs := make([]int64, 0, len(students))
	personIDs := make([]int64, 0, len(students))
	byID := make(map[int64]*userModels.Student, len(students))
	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
		personIDs = append(personIDs, student.PersonID)
		byID[student.ID] = student
	}
	return studentIDs, personIDs, byID
}

func assembleStatusDayOverview(rows []*activeModels.StudentStatusDay, students map[int64]*userModels.Student, persons map[int64]*userModels.Person, groups map[int64]*educationModels.Group) []StatusDayOverviewEntry {
	entries := make([]StatusDayOverviewEntry, 0, len(rows))
	for _, row := range rows {
		student := students[row.StudentID]
		if !studentEnrolledOn(student, row.Date) || student.GroupID == nil {
			continue
		}
		entries = append(entries, StatusDayOverviewEntry{
			StatusDay: row,
			Student:   student,
			Person:    persons[student.PersonID],
			Group:     groups[*student.GroupID],
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].StatusDay.Date != entries[j].StatusDay.Date {
			return entries[i].StatusDay.Date.Before(entries[j].StatusDay.Date)
		}
		if entries[i].Person == nil || entries[j].Person == nil {
			return entries[i].Student.ID < entries[j].Student.ID
		}
		if entries[i].Person.LastName != entries[j].Person.LastName {
			return entries[i].Person.LastName < entries[j].Person.LastName
		}
		return entries[i].Person.FirstName < entries[j].Person.FirstName
	})
	return entries
}

func studentEnrolledOn(student *userModels.Student, date timezone.Date) bool {
	if student == nil {
		return false
	}
	if student.EnrolledFrom != nil && date.Before(*student.EnrolledFrom) {
		return false
	}
	if student.EnrolledUntil != nil && date.After(*student.EnrolledUntil) {
		return false
	}
	return student.EnrolledFrom != nil || student.EnrolledUntil != nil || student.Status != userModels.StudentStatusInactive
}
