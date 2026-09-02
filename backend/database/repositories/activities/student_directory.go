package activities

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
)

// DirectoryStudent is the People Directory projection this package reads.
// users.students belongs to that owner (#2662); the composition root binds
// the directory behind StudentDirectory instead of the former SQL joins.
type DirectoryStudent struct {
	ID            int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	PersonID      int64
	SchoolClass   string
	GroupID       *int64
	Status        string
	EnrolledFrom  string
	EnrolledUntil string
}

// StudentDirectory is the owner query the activities repositories read
// students through. Every method fails while unbound; there is no fallback
// join.
type StudentDirectory interface {
	// ListStudentsByID returns the tenant's rows for ids, alumni included.
	ListStudentsByID(ctx context.Context, ids []int64) ([]DirectoryStudent, error)
	// ListEnrolledStudents returns every non-alumni student of the current
	// tenant ordered by id.
	ListEnrolledStudents(ctx context.Context) ([]DirectoryStudent, error)
}

var errStudentDirectoryRequired = errors.New("activities repositories: student directory is not bound")

var leadingDigits = regexp.MustCompile(`[0-9]+`)

// matchTargetStudents resolves the dynamic target cohorts of timetable
// templates in Go, with the predicates the former SQL join used: a Jahrgang
// target matches the first digit run of the class, a Klasse target the
// trimmed class case-insensitively, a Gruppe target the education group.
// When enrolledOn is set, children whose care ended before that day drop
// out (#2487). Each cohort is distinct and ordered by student id.
func matchTargetStudents(targets map[int64][]*activities.GroupTarget, students []DirectoryStudent, enrolledOn *timezone.Date) map[int64][]int64 {
	result := make(map[int64][]int64, len(targets))
	for groupID, rules := range targets {
		members := make([]int64, 0)
		for _, student := range students {
			if enrolledOn != nil && careEndedBefore(student.EnrolledUntil, *enrolledOn) {
				continue
			}
			for _, rule := range rules {
				if targetMatches(rule, student) {
					members = append(members, student.ID)
					break
				}
			}
		}
		slices.Sort(members)
		result[groupID] = slices.Compact(members)
	}
	return result
}

// careEndedBefore reports whether the directory's YYYY-MM-DD care end date
// lies before day; an empty or unparsable value means open-ended care.
func careEndedBefore(enrolledUntil string, day timezone.Date) bool {
	until := parseDirectoryDate(enrolledUntil)
	return until != nil && until.Before(day)
}

// parseDirectoryDate turns the directory's calendar day into the legacy
// model's timezone.Date; empty stays nil.
func parseDirectoryDate(value string) *timezone.Date {
	if value == "" {
		return nil
	}
	date, err := timezone.ParseDate(value)
	if err != nil {
		return nil
	}
	return &date
}

func targetMatches(rule *activities.GroupTarget, student DirectoryStudent) bool {
	switch rule.TargetGroupType {
	case activities.TargetGroupTypeJahrgang:
		if rule.TargetGradeLevel == nil {
			return false
		}
		return leadingDigits.FindString(student.SchoolClass) == strconv.Itoa(int(*rule.TargetGradeLevel))
	case activities.TargetGroupTypeKlasse:
		if rule.TargetSchoolClass == nil {
			return false
		}
		return strings.EqualFold(strings.TrimSpace(student.SchoolClass), strings.TrimSpace(*rule.TargetSchoolClass))
	case activities.TargetGroupTypeGruppe:
		return rule.EducationGroupID != nil && student.GroupID != nil && *rule.EducationGroupID == *student.GroupID
	default:
		return false
	}
}
