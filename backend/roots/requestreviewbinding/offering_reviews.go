package requestreviewbinding

import (
	"context"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type OfferingReviewPeople interface {
	ReviewPeople
	SearchPersons(context.Context, peopledirectory.PersonFilter) ([]peopledirectory.Person, error)
	ListStudentsByPersonID(context.Context, []int64) ([]peopledirectory.Student, error)
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

type offeringReviewEnrollmentRows struct{ query enrollment.Owner }
type offeringReviewSelectionRows struct {
	bookings careplan.OfferingBookingQueries
}
type offeringReviewChildReads interface {
	Children(context.Context, []int64) ([]careplan.OfferingReviewChild, error)
	Requests(context.Context, []int64) ([]careplan.OfferingReviewRequest, error)
	Phases(context.Context, []int64) ([]careplan.OfferingReviewPhase, error)
}
type offeringReviewSelectionReads interface {
	Selections(context.Context, map[int64]careplan.Date) ([]careplan.OfferingReviewBooking, error)
}
type offeringReviewEnrollment struct {
	offeringReviewChildReads
	offeringReviewSelectionReads
}

func (e offeringReviewEnrollmentRows) Children(ctx context.Context, ids []int64) ([]careplan.OfferingReviewChild, error) {
	rows, err := e.query.ChildrenByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]careplan.OfferingReviewChild, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			child := careplan.OfferingReviewChild{ID: row.ID, RequestID: row.RequestID, Grade: row.TargetGradeLevel}
			if row.TargetSchoolClass != nil {
				child.SchoolClass = *row.TargetSchoolClass
			}
			result = append(result, child)
		}
	}
	return result, nil
}
func (e offeringReviewEnrollmentRows) Requests(ctx context.Context, ids []int64) ([]careplan.OfferingReviewRequest, error) {
	rows, err := e.query.RequestsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]careplan.OfferingReviewRequest, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			result = append(result, careplan.OfferingReviewRequest{ID: row.ID, PhaseID: row.PhaseID})
		}
	}
	return result, nil
}
func (e offeringReviewEnrollmentRows) Phases(ctx context.Context, ids []int64) ([]careplan.OfferingReviewPhase, error) {
	rows, err := e.query.PhasesByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]careplan.OfferingReviewPhase, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			result = append(result, careplan.OfferingReviewPhase{ID: row.ID, Start: careplan.Date(row.ServiceStartDate), End: careplan.Date(row.ServiceEndDate), SelectionMode: row.CareOfferingSelectionMode})
		}
	}
	return result, nil
}
func (e offeringReviewSelectionRows) Selections(ctx context.Context, dates map[int64]careplan.Date) ([]careplan.OfferingReviewBooking, error) {
	rows, err := e.bookings.CareOfferingBookingsAtDates(ctx, dates)
	if err != nil {
		return nil, err
	}
	result := make([]careplan.OfferingReviewBooking, 0, len(rows))
	for _, row := range rows {
		result = append(result, careplan.OfferingReviewBooking{ChildID: row.RequestChildID, OfferingID: row.CareOfferingID, SelectedDays: row.EffectiveSelectedDays(), ManualDays: row.ManualSelectedDays, AutomaticDays: row.AutomaticSelectedDays})
	}
	return result, nil
}

type offeringReviewCourses struct{ query timetable.CourseGroupQuery }

func (c offeringReviewCourses) CourseGroups(ctx context.Context, refs []careplan.OfferingReviewCourseRef, day careplan.Date) (map[int64][]careplan.OfferingReviewCourse, error) {
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
	result := map[int64][]careplan.OfferingReviewCourse{}
	for _, group := range groups {
		ids := append(slices.Clone(legacy[group.ID]), group.SourceCareOfferingIDs...)
		for _, id := range ids {
			result[id] = append(result[id], careplan.OfferingReviewCourse{Active: group.Active, Grades: group.SourceGradeLevels, SchoolClasses: group.SourceSchoolClasses})
		}
	}
	return result, nil
}

func NewOfferingReviewDirectory(people OfferingReviewPeople) careplan.OfferingReviewDirectory {
	return offeringReviewDirectory{reviewDirectory: reviewDirectory{people: people}, search: people}
}
func NewOfferingReviewEnrollment(query enrollment.Owner, bookings careplan.OfferingBookingQueries) careplan.OfferingReviewEnrollment {
	return offeringReviewEnrollment{offeringReviewChildReads: offeringReviewEnrollmentRows{query: query}, offeringReviewSelectionReads: offeringReviewSelectionRows{bookings: bookings}}
}
func NewOfferingReviewCourses(query timetable.CourseGroupQuery) careplan.OfferingReviewCourses {
	return offeringReviewCourses{query: query}
}
