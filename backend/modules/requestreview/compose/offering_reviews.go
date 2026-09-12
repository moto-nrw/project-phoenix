package compose

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carecompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentcompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetablecompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

type OfferingReviewPeople interface {
	ReviewPeople
	SearchPersons(context.Context, peopledirectory.PersonFilter) ([]peopledirectory.Person, error)
	ListStudentsByPersonID(context.Context, []int64) ([]peopledirectory.Student, error)
}

type OfferingReviewDependencies struct {
	People           OfferingReviewPeople
	Scope            carecompose.ReviewScopeResolver
	Today            func() careplan.Date
	ObserveCare      func(CareObservation)
	ObserveTimetable func(TimetableObservation)
}

func NewOfferingReviews(db *bun.DB, deps OfferingReviewDependencies) (careplan.OfferingReviewQuery, error) {
	if deps.People == nil {
		return nil, errors.New("offering reviews: people are required")
	}
	courses, err := timetablecompose.NewCourseGroupQueries(db, deps.ObserveTimetable)
	if err != nil {
		return nil, err
	}
	return carecompose.NewOfferingReviews(db, deps.ObserveCare, carecompose.OfferingReviewDependencies{
		People:     offeringReviewDirectory{reviewDirectory: reviewDirectory{people: deps.People}, search: deps.People},
		Enrollment: offeringReviewEnrollment{query: enrollmentcompose.New()}, Courses: offeringReviewCourses{query: courses},
		Scope: deps.Scope, Today: deps.Today,
	})
}

type offeringReviewDirectory struct {
	reviewDirectory
	search OfferingReviewPeople
}

func (d offeringReviewDirectory) SearchStudentIDs(ctx context.Context, search string, within []int64) ([]int64, error) {
	name := strings.TrimSpace(search)
	if name == "" {
		return within, nil
	}
	ids := make([]int64, 0)
	for page := 1; ; page++ {
		people, err := d.search.SearchPersons(ctx, peopledirectory.PersonFilter{FullNameContains: name, Page: page, PageSize: 100})
		if err != nil {
			return nil, err
		}
		for _, person := range people {
			ids = append(ids, person.ID)
		}
		if len(people) < 100 {
			break
		}
	}
	result := make([]int64, 0)
	if len(ids) == 0 {
		return result, nil
	}
	students, err := d.search.ListStudentsByPersonID(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, student := range students {
		if !student.IsAlumnus() && (within == nil || slices.Contains(within, student.ID)) {
			result = append(result, student.ID)
		}
	}
	return result, nil
}

type offeringReviewEnrollment struct{ query *enrollment.Module }

func (e offeringReviewEnrollment) Children(ctx context.Context, ids []int64) ([]carecompose.OfferingReviewChild, error) {
	rows, err := e.query.ChildrenByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]carecompose.OfferingReviewChild, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			child := carecompose.OfferingReviewChild{ID: row.ID, RequestID: row.RequestID, Grade: row.TargetGradeLevel}
			if row.TargetSchoolClass != nil {
				child.SchoolClass = *row.TargetSchoolClass
			}
			result = append(result, child)
		}
	}
	return result, nil
}
func (e offeringReviewEnrollment) Requests(ctx context.Context, ids []int64) ([]carecompose.OfferingReviewRequest, error) {
	rows, err := e.query.RequestsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]carecompose.OfferingReviewRequest, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			result = append(result, carecompose.OfferingReviewRequest{ID: row.ID, PhaseID: row.PhaseID})
		}
	}
	return result, nil
}
func (e offeringReviewEnrollment) Phases(ctx context.Context, ids []int64) ([]carecompose.OfferingReviewPhase, error) {
	rows, err := e.query.PhasesByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]carecompose.OfferingReviewPhase, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			result = append(result, carecompose.OfferingReviewPhase{ID: row.ID, Start: careplan.Date(row.ServiceStartDate), End: careplan.Date(row.ServiceEndDate), SelectionMode: row.CareOfferingSelectionMode})
		}
	}
	return result, nil
}
func (e offeringReviewEnrollment) Selections(ctx context.Context, dates map[int64]careplan.Date) ([]carecompose.OfferingReviewBooking, error) {
	ownerDates := make(map[int64]enrollment.Date, len(dates))
	for id, day := range dates {
		ownerDates[id] = enrollment.Date(day)
	}
	rows, err := e.query.RequestChildOfferingsAtDates(ctx, ownerDates)
	if err != nil {
		return nil, err
	}
	result := make([]carecompose.OfferingReviewBooking, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			result = append(result, carecompose.OfferingReviewBooking{ChildID: row.RequestChildID, OfferingID: row.CareOfferingID, SelectedDays: row.SelectedDays, ManualDays: row.ManualSelectedDays, AutomaticDays: row.AutomaticSelectedDays})
		}
	}
	return result, nil
}

type offeringReviewCourses struct{ query timetable.CourseGroupQuery }

func (c offeringReviewCourses) CourseGroups(ctx context.Context, refs []carecompose.OfferingReviewCourseRef, day careplan.Date) (map[int64][]carecompose.OfferingReviewCourse, error) {
	filter := timetable.CourseGroupFilter{EffectiveOn: day.String()}
	legacy := map[int64][]int64{}
	for _, ref := range refs {
		filter.SourceOfferingIDs = append(filter.SourceOfferingIDs, ref.OfferingID)
		if ref.LegacyGroupID != nil && *ref.LegacyGroupID > 0 {
			filter.LegacyGroupIDs = append(filter.LegacyGroupIDs, *ref.LegacyGroupID)
			legacy[*ref.LegacyGroupID] = append(legacy[*ref.LegacyGroupID], ref.OfferingID)
		}
	}
	groups, err := c.query.ListCourseGroups(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := map[int64][]carecompose.OfferingReviewCourse{}
	for _, group := range groups {
		ids := append(slices.Clone(legacy[group.ID]), group.SourceCareOfferingIDs...)
		for _, id := range ids {
			result[id] = append(result[id], carecompose.OfferingReviewCourse{Active: group.Active, Grades: group.SourceGradeLevels, SchoolClasses: group.SourceSchoolClasses})
		}
	}
	return result, nil
}
