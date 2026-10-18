//go:build testhooks

package api

import (
	"context"
	"slices"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewOfferingReviewTestQuery composes the staff review queue of offering
// changes (#3179) over the test database, reviewing the whole school on the
// given day, with the People Directory, Enrollment and Timetable reads the
// request-review projection binds in the server. Offering-change suites
// assert the queue a staff member sees for the requests they submit.
func NewOfferingReviewTestQuery(db transactionruntime.Database, today func() calendar.Date) (careplan.OfferingReviewQuery, error) {
	auditCommand, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return nil, err
	}
	_, repos, err := NewStudentTestRepositories(db, auditCommand)
	if err != nil {
		return nil, err
	}
	reposTimetable := NewEnrollmentTimetableRows(db)
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return nil, err
	}
	return careplanCompose.NewOfferingReviews(db.Pool(), func(careplanCompose.Observation) {}, careplanCompose.OfferingReviewDependencies{
		People:     offeringReviewTestDirectory{people: people},
		Enrollment: offeringReviewTestEnrollment{enrollment: repos.Enrollment()},
		Courses:    offeringReviewTestCourses{query: reposTimetable.Timetable},
		Scope: func(context.Context) (careplanCompose.ReviewScope, error) {
			return careplanCompose.ReviewScope{SchoolWide: true}, nil
		},
		Today: func() careplan.Date { return careplan.Date(today()) },
	})
}

type offeringReviewTestDirectory struct{ people peopledirectory.Capability }

func (d offeringReviewTestDirectory) FindStudents(ctx context.Context, ids []int64) (map[int64]careplanCompose.ReviewStudent, error) {
	students, err := d.people.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]careplanCompose.ReviewStudent, len(students))
	for _, student := range students {
		result[student.ID] = careplanCompose.ReviewStudent{
			ID: student.ID, PersonID: student.PersonID, GroupID: student.GroupID, Alumnus: student.IsAlumnus(),
			EnrolledUntil: careplan.Date(student.EnrolledUntil), SchoolClass: student.SchoolClass,
		}
	}
	return result, nil
}

func (d offeringReviewTestDirectory) PersonNames(ctx context.Context, ids []int64) (map[int64]careplanCompose.PersonName, error) {
	persons, err := d.people.ListPersonsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]careplanCompose.PersonName, len(persons))
	for _, person := range persons {
		names[person.ID] = careplanCompose.PersonName{FirstName: person.FirstName, LastName: person.LastName}
	}
	return names, nil
}

func (d offeringReviewTestDirectory) ReviewerNames(ctx context.Context, ids []int64) (map[int64]careplanCompose.PersonName, error) {
	persons, err := d.people.ListPersonsByAccount(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]careplanCompose.PersonName, len(persons))
	for _, person := range persons {
		if person.AccountID != nil {
			names[*person.AccountID] = careplanCompose.PersonName{FirstName: person.FirstName, LastName: person.LastName}
		}
	}
	return names, nil
}

// SearchStudentIDs is never reached: the suites list the queue unfiltered.
func (d offeringReviewTestDirectory) SearchStudentIDs(_ context.Context, _ string, within []int64) ([]int64, error) {
	return within, nil
}

// offeringReviewTestEnrollment serves the review's Enrollment reads from the
// projection and the bookings from Care Plan's tenant-scoped offering
// bookings, which carry no state and are composed per read.
type offeringReviewTestEnrollment struct {
	enrollment EnrollmentBookingProjection
}

func (e offeringReviewTestEnrollment) Children(ctx context.Context, ids []int64) ([]careplanCompose.OfferingReviewChild, error) {
	rows, err := e.enrollment.ChildrenByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]careplanCompose.OfferingReviewChild, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		child := careplanCompose.OfferingReviewChild{ID: row.ID, RequestID: row.RequestID, Grade: row.TargetGradeLevel}
		if row.TargetSchoolClass != nil {
			child.SchoolClass = *row.TargetSchoolClass
		}
		result = append(result, child)
	}
	return result, nil
}

func (e offeringReviewTestEnrollment) Requests(ctx context.Context, ids []int64) ([]careplanCompose.OfferingReviewRequest, error) {
	rows, err := e.enrollment.RequestsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]careplanCompose.OfferingReviewRequest, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			result = append(result, careplanCompose.OfferingReviewRequest{ID: row.ID, PhaseID: row.PhaseID})
		}
	}
	return result, nil
}

func (e offeringReviewTestEnrollment) Phases(ctx context.Context, ids []int64) ([]careplanCompose.OfferingReviewPhase, error) {
	rows, err := e.enrollment.PhasesByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]careplanCompose.OfferingReviewPhase, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			result = append(result, careplanCompose.OfferingReviewPhase{
				ID: row.ID, Start: careplan.Date(row.ServiceStartDate), End: careplan.Date(row.ServiceEndDate),
				SelectionMode: row.CareOfferingSelectionMode,
			})
		}
	}
	return result, nil
}

func (e offeringReviewTestEnrollment) Selections(ctx context.Context, dates map[int64]careplan.Date) ([]careplanCompose.OfferingReviewBooking, error) {
	rows, err := careplanCompose.NewOfferingBookings().CareOfferingBookingsAtDates(ctx, dates)
	if err != nil {
		return nil, err
	}
	result := make([]careplanCompose.OfferingReviewBooking, 0, len(rows))
	for _, row := range rows {
		result = append(result, careplanCompose.OfferingReviewBooking{
			ChildID: row.RequestChildID, OfferingID: row.CareOfferingID, SelectedDays: row.EffectiveSelectedDays(),
			ManualDays: row.ManualSelectedDays, AutomaticDays: row.AutomaticSelectedDays,
		})
	}
	return result, nil
}

type offeringReviewTestCourses struct{ query timetable.CourseGroupQuery }

func (c offeringReviewTestCourses) CourseGroups(ctx context.Context, refs []careplanCompose.OfferingReviewCourseRef, day careplan.Date) (map[int64][]careplanCompose.OfferingReviewCourse, error) {
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
	result := map[int64][]careplanCompose.OfferingReviewCourse{}
	for _, group := range groups {
		for _, id := range append(slices.Clone(legacy[group.ID]), group.SourceCareOfferingIDs...) {
			result[id] = append(result[id], careplanCompose.OfferingReviewCourse{
				Active: group.Active, Grades: group.SourceGradeLevels, SchoolClasses: group.SourceSchoolClasses,
			})
		}
	}
	return result, nil
}
