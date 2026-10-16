package presence

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type StatusDayOverviewPeople interface {
	GetStudentsByGroupIDs(ctx context.Context, groupIDs []int64) ([]*StudentRecord, error)
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*PersonName, error)
}

// StudentStatusDayOverviewService owns the fully configured read model for
// the tenant-wide absence overview.
type StudentStatusDayOverviewService struct {
	repo   StudentStatusDayOverviewRepository
	people StatusDayOverviewPeople
}

func NewStudentStatusDayOverviewService(repo StudentStatusDayOverviewRepository, people StatusDayOverviewPeople) *StudentStatusDayOverviewService {
	return &StudentStatusDayOverviewService{repo: repo, people: people}
}

// GetOverview loads and assembles the absence rows for the authorized groups.
// Enrollment is evaluated on each status-day date; lifecycle status is used
// only for legacy students without enrollment bounds.
func (s *StudentStatusDayOverviewService) GetOverview(ctx context.Context, groups []*StatusDayOverviewGroup, from, to, today calendar.Date, filters StatusDayOverviewFilters) (*StatusDayOverview, error) {
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
	orderedStudentIDs := orderOverviewStudentIDs(studentIDs, studentsByID, persons)
	query := statusDayOverviewQuery(studentIDs, studentsByID, from, to, today, filters)
	total, err := s.repo.CountOverview(ctx, query)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListOverview(ctx, query, orderedStudentIDs)
	if err != nil {
		return nil, err
	}
	return &StatusDayOverview{Entries: assembleStatusDayOverview(rows, studentsByID, persons, groupsByID, today), HasMore: filters.Page*filters.PageSize < total}, nil
}

// orderOverviewStudentIDs applies the name portion of the overview order. The
// repository keeps date first and uses this rank before SQL pagination.
func orderOverviewStudentIDs(ids []int64, students map[int64]*StudentRecord, persons map[int64]*PersonName) []int64 {
	ordered := slices.Clone(ids)
	personOf := func(studentID int64) *PersonName {
		if student := students[studentID]; student != nil {
			return persons[student.PersonID]
		}
		return nil
	}
	nameOf := func(person *PersonName) (string, string) {
		if person == nil {
			return "", ""
		}
		return person.LastName, person.FirstName
	}
	slices.SortFunc(ordered, func(left, right int64) int {
		leftLast, leftFirst := nameOf(personOf(left))
		rightLast, rightFirst := nameOf(personOf(right))
		if order := strings.Compare(leftLast, rightLast); order != 0 {
			return order
		}
		if order := strings.Compare(leftFirst, rightFirst); order != 0 {
			return order
		}
		if left != right {
			if left < right {
				return -1
			}
			return 1
		}
		return 0
	})
	return ordered
}

// statusDayOverviewQuery selects the overview page: every requested child
// inside its enrollment window, the date range, and the optional status.
func statusDayOverviewQuery(studentIDs []int64, students map[int64]*StudentRecord, from, to, today calendar.Date, filters StatusDayOverviewFilters) StatusDayOverviewQuery {
	query := StatusDayOverviewQuery{From: from, To: to, Status: filters.Status, Page: filters.Page, PageSize: filters.PageSize}
	for _, id := range studentIDs {
		if window, ok := studentEnrollmentWindow(id, students[id], today); ok {
			query.Children = append(query.Children, window)
		}
	}
	return query
}

// studentEnrollmentWindow limits one student's rows to their enrollment
// interval; an active student enrolled from a future date counts from today.
// It reports false for an unknown student or an inactive one without an
// interval.
func studentEnrollmentWindow(id int64, student *StudentRecord, today calendar.Date) (StatusDayWindow, bool) {
	if student == nil || (student.EnrolledFrom == nil && student.EnrolledUntil == nil && student.Lifecycle == StudentLifecycleInactive) {
		return StatusDayWindow{}, false
	}
	window := StatusDayWindow{StudentID: id, Until: student.EnrolledUntil}
	if student.EnrolledFrom != nil {
		from := *student.EnrolledFrom
		if student.Lifecycle == StudentLifecycleActive && today.Before(from) {
			from = today
		}
		window.From = &from
	}
	return window, true
}

func filterOverviewStudentIDs(ids []int64, students map[int64]*StudentRecord, persons map[int64]*PersonName, query string) []int64 {
	needle := strings.ToLower(strings.TrimSpace(query))
	filtered := make([]int64, 0, len(ids))
	for _, id := range ids {
		student := students[id]
		if student == nil {
			continue
		}
		person := persons[student.PersonID]
		if person == nil {
			if needle == "" {
				filtered = append(filtered, id)
			}
			continue
		}
		if needle == "" || strings.Contains(strings.ToLower(person.FirstName+" "+person.LastName+" "+student.SchoolClass), needle) {
			filtered = append(filtered, id)
		}
	}
	return filtered
}

func indexOverviewGroups(groups []*StatusDayOverviewGroup) ([]int64, map[int64]*StatusDayOverviewGroup) {
	ids := make([]int64, 0, len(groups))
	byID := make(map[int64]*StatusDayOverviewGroup, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
		byID[group.ID] = group
	}
	return ids, byID
}

func indexOverviewStudents(students []*StudentRecord) ([]int64, []int64, map[int64]*StudentRecord) {
	studentIDs := make([]int64, 0, len(students))
	personIDs := make([]int64, 0, len(students))
	byID := make(map[int64]*StudentRecord, len(students))
	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
		personIDs = append(personIDs, student.PersonID)
		byID[student.ID] = student
	}
	return studentIDs, personIDs, byID
}

func assembleStatusDayOverview(rows []*absencerecords.StudentStatusDay, students map[int64]*StudentRecord, persons map[int64]*PersonName, groups map[int64]*StatusDayOverviewGroup, today calendar.Date) []StatusDayOverviewEntry {
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

func studentEnrolledOn(student *StudentRecord, date, today calendar.Date) bool {
	if student == nil {
		return false
	}
	if student.EnrolledFrom != nil && date.Before(*student.EnrolledFrom) &&
		(student.Lifecycle != StudentLifecycleActive || date.Before(today)) {
		return false
	}
	if student.EnrolledUntil != nil && date.After(*student.EnrolledUntil) {
		return false
	}
	return student.EnrolledFrom != nil || student.EnrolledUntil != nil || student.Lifecycle != StudentLifecycleInactive
}
