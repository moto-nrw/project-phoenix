package owners

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// requestSubmissionAdapter reads the plan facts a request records and the
// child's name its review item shows.
type requestSubmissionAdapter struct{ s *CareScheduleRequestService }

func (a requestSubmissionAdapter) PickupTime(ctx context.Context, id int64, date calendar.Date) (*time.Time, error) {
	if a.s.Pickup == nil {
		return nil, nil
	}
	effective, err := a.s.Pickup.GetEffectivePickupTimeForDate(ctx, id, date)
	if err != nil || effective == nil {
		return nil, err
	}
	return effective.PickupTime, nil
}

func (a requestSubmissionAdapter) DayDepartureMode(ctx context.Context, id int64, date calendar.Date) (string, bool, error) {
	days, err := a.s.RequestRecords.ListDepartureExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Date: careplan.Date(date)})
	if err != nil || len(days) == 0 {
		return "", false, err
	}
	return days[0].DepartureMode, true, nil
}

func (a requestSubmissionAdapter) WeeklyDepartureModes(ctx context.Context, id int64) (map[string][]string, error) {
	return requestDiffAdapter(a).DepartureModesForStudent(ctx, id)
}

func (a requestSubmissionAdapter) StudentName(ctx context.Context, studentID int64) (compose.PersonName, error) {
	student, err := a.s.People.FindStudentRecord(ctx, studentID)
	if err != nil {
		return compose.PersonName{}, err
	}
	person, err := a.s.People.FindPerson(ctx, student.PersonID)
	if err != nil {
		return compose.PersonName{}, err
	}
	return compose.PersonName{FirstName: person.FirstName, LastName: person.LastName}, nil
}
