package active

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

type statusDayOverviewPeople interface {
	GetStudentsByGroupIDs(ctx context.Context, groupIDs []int64) ([]*userModels.Student, error)
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Person, error)
}

// StudentStatusDayOverviewService owns the fully configured read model for
// the tenant-wide absence overview.
type StudentStatusDayOverviewService struct {
	repo   activeModels.StudentStatusDayOverviewRepository
	people statusDayOverviewPeople
}

func NewStudentStatusDayOverviewService(repo activeModels.StudentStatusDayOverviewRepository, people statusDayOverviewPeople) *StudentStatusDayOverviewService {
	return &StudentStatusDayOverviewService{repo: repo, people: people}
}

type StatusDayOverviewEntry struct {
	StatusDay *activeModels.StudentStatusDay
	Student   *userModels.Student
	Person    *userModels.Person
	Group     *educationModels.Group
}

type StatusDayOverviewFilters struct {
	Query    string
	Status   string
	Page     int
	PageSize int
}

type StatusDayOverview struct {
	Entries []StatusDayOverviewEntry
	HasMore bool
}

// GetOverview loads and assembles the absence rows for the authorized groups.
// Enrollment is evaluated on each status-day date; lifecycle status is used
// only for legacy students without enrollment bounds.
func (s *StudentStatusDayOverviewService) GetOverview(ctx context.Context, groups []*educationModels.Group, from, to, today timezone.Date, filters StatusDayOverviewFilters) (*StatusDayOverview, error) {
	if s.people == nil || s.repo == nil {
		return nil, errors.New("student status day overview dependencies are not configured")
	}
	groupIDs, groupsByID := indexOverviewGroups(groups)
	students, err := s.people.GetStudentsByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	studentIDs, personIDs, studentsByID := indexOverviewStudents(students)
	persons, err := s.people.GetByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	studentIDs = filterOverviewStudentIDs(studentIDs, studentsByID, persons, filters.Query)
	if len(studentIDs) == 0 {
		return &StatusDayOverview{Entries: []StatusDayOverviewEntry{}}, nil
	}
	options := statusDayOverviewOptions(studentIDs, studentsByID, from, to, today, filters)
	total, err := s.repo.CountWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListOverviewWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}
	return &StatusDayOverview{Entries: assembleStatusDayOverview(rows, studentsByID, persons, groupsByID, today), HasMore: filters.Page*filters.PageSize < total}, nil
}

func statusDayOverviewOptions(studentIDs []int64, students map[int64]*userModels.Student, from, to, today timezone.Date, filters StatusDayOverviewFilters) *modelBase.QueryOptions {
	filter := modelBase.NewFilter().GreaterThanOrEqual("date", from).LessThanOrEqual("date", to)
	filter.And(*statusDayEnrollmentFilter(studentIDs, students, today))
	active := modelBase.NewFilter().IsNull("cleared_at")
	active.Or(*modelBase.NewFilter().Equal("source", activeModels.StudentStatusSourceEndOfDay))
	filter.And(*active)
	if filters.Status != "" {
		filter.Equal("status", filters.Status)
	}
	return &modelBase.QueryOptions{
		Filter:     filter,
		Pagination: &modelBase.Pagination{Page: filters.Page, PageSize: filters.PageSize},
	}
}

func statusDayEnrollmentFilter(studentIDs []int64, students map[int64]*userModels.Student, today timezone.Date) *modelBase.Filter {
	var eligible *modelBase.Filter
	for _, id := range studentIDs {
		student := students[id]
		if student == nil || (student.EnrolledFrom == nil && student.EnrolledUntil == nil && student.Status == userModels.StudentStatusInactive) {
			continue
		}
		studentFilter := modelBase.NewFilter().Equal("student_id", id)
		if student.EnrolledFrom != nil {
			from := *student.EnrolledFrom
			if student.Status == userModels.StudentStatusActive && today.Before(from) {
				from = today
			}
			studentFilter.GreaterThanOrEqual("date", from)
		}
		if student.EnrolledUntil != nil {
			studentFilter.LessThanOrEqual("date", *student.EnrolledUntil)
		}
		if eligible == nil {
			eligible = studentFilter
		} else {
			eligible.Or(*studentFilter)
		}
	}
	if eligible == nil {
		return modelBase.NewFilter().Equal("student_id", int64(-1))
	}
	return eligible
}

func filterOverviewStudentIDs(ids []int64, students map[int64]*userModels.Student, persons map[int64]*userModels.Person, query string) []int64 {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return ids
	}
	filtered := make([]int64, 0, len(ids))
	for _, id := range ids {
		student := students[id]
		person := persons[student.PersonID]
		if person != nil && strings.Contains(strings.ToLower(person.FirstName+" "+person.LastName+" "+student.SchoolClass), needle) {
			filtered = append(filtered, id)
		}
	}
	return filtered
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

func assembleStatusDayOverview(rows []*activeModels.StudentStatusDay, students map[int64]*userModels.Student, persons map[int64]*userModels.Person, groups map[int64]*educationModels.Group, today timezone.Date) []StatusDayOverviewEntry {
	entries := make([]StatusDayOverviewEntry, 0, len(rows))
	for _, row := range rows {
		student := students[row.StudentID]
		if !studentEnrolledOn(student, row.Date, today) || student.GroupID == nil {
			continue
		}
		entries = append(entries, StatusDayOverviewEntry{
			StatusDay: row,
			Student:   student,
			Person:    persons[student.PersonID],
			Group:     groups[*student.GroupID],
		})
	}
	return entries
}

func studentEnrolledOn(student *userModels.Student, date, today timezone.Date) bool {
	if student == nil {
		return false
	}
	if student.EnrolledFrom != nil && date.Before(*student.EnrolledFrom) &&
		(student.Status != userModels.StudentStatusActive || date.Before(today)) {
		return false
	}
	if student.EnrolledUntil != nil && date.After(*student.EnrolledUntil) {
		return false
	}
	return student.EnrolledFrom != nil || student.EnrolledUntil != nil || student.Status != userModels.StudentStatusInactive
}
