package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

type OfferingReviewDirectory interface {
	FindStudents(context.Context, []int64) (map[int64]ReviewStudent, error)
	PersonNames(context.Context, []int64) (map[int64]PersonName, error)
	ReviewerNames(context.Context, []int64) (map[int64]PersonName, error)
	SearchStudentIDs(context.Context, string, []int64) ([]int64, error)
}

type OfferingReviewChild struct {
	ID          int64
	RequestID   int64
	Grade       *int16
	SchoolClass string
}

type OfferingReviewRequest struct {
	ID      int64
	PhaseID int64
}

type OfferingReviewPhase struct {
	ID            int64
	Start         domain.Date
	End           domain.Date
	SelectionMode string
}

type OfferingReviewBooking struct {
	ChildID       int64
	OfferingID    int64
	SelectedDays  []string
	ManualDays    []string
	AutomaticDays []string
}

type OfferingReviewEnrollment interface {
	Children(context.Context, []int64) ([]OfferingReviewChild, error)
	Requests(context.Context, []int64) ([]OfferingReviewRequest, error)
	Phases(context.Context, []int64) ([]OfferingReviewPhase, error)
	Selections(context.Context, map[int64]domain.Date) ([]OfferingReviewBooking, error)
}

type OfferingReviewCourseRef struct {
	OfferingID    int64
	LegacyGroupID *int64
}

type OfferingReviewCourse struct {
	Active        bool
	Grades        []int
	SchoolClasses []string
}

type OfferingReviewCourses interface {
	CourseGroups(context.Context, []OfferingReviewCourseRef, domain.Date) (map[int64][]OfferingReviewCourse, error)
}
