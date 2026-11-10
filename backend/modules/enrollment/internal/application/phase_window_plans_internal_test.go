package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	peopleEnrollment "github.com/moto-nrw/project-phoenix/modules/peopledirectory/enrollment"
)

// followedWindow moves only the bounds a phase still decides. A pending
// child with approvals in several phases starts with the earliest and ends
// with the latest approval inside its current window; a bound from another
// enrollment or a manual edit stays (#6505).
func TestFollowedWindow(t *testing.T) {
	t.Parallel()

	const schoolYear, holiday = 1, 2
	window := func(from, until string) peopleEnrollment.Record {
		return peopleEnrollment.Record{EnrolledFrom: from, EnrolledUntil: until}
	}
	phase := func(start, end string) *enrollment.Phase {
		return &enrollment.Phase{ServiceStartDate: enrollment.Date(start), ServiceEndDate: enrollment.Date(end)}
	}
	plan := func(phaseID int64, on, end string) enrollment.ActivationPlan {
		return enrollment.ActivationPlan{PhaseID: phaseID, On: enrollment.Date(on), PhaseEnd: enrollment.Date(end)}
	}
	year := phase("2027-08-01", "2028-07-31")
	tests := []struct {
		name      string
		student   peopleEnrollment.Record
		plans     []enrollment.ActivationPlan
		after     *enrollment.Phase
		wantFrom  string
		wantUntil string
	}{
		{
			name: "later start", student: window("2027-08-01", "2028-07-31"),
			plans: []enrollment.ActivationPlan{plan(schoolYear, "2027-08-01", "2028-07-31")},
			after: phase("2027-08-08", "2028-07-31"), wantFrom: "2027-08-08", wantUntil: "2028-07-31",
		},
		{
			name: "earlier start and end", student: window("2027-08-01", "2028-07-31"),
			plans: []enrollment.ActivationPlan{plan(schoolYear, "2027-08-01", "2028-07-31")},
			after: phase("2027-07-25", "2028-07-24"), wantFrom: "2027-07-25", wantUntil: "2028-07-24",
		},
		{
			name: "manual start stays", student: window("2027-09-15", "2028-07-31"),
			plans: []enrollment.ActivationPlan{plan(schoolYear, "2027-08-01", "2028-07-31")},
			after: phase("2027-08-08", "2028-07-31"), wantFrom: "2027-09-15", wantUntil: "2028-07-31",
		},
		{
			name: "earlier holiday approval keeps the start", student: window("2027-07-20", "2028-07-31"),
			plans: []enrollment.ActivationPlan{plan(holiday, "2027-07-20", "2027-07-31"), plan(schoolYear, "2027-08-01", "2028-07-31")},
			after: phase("2027-08-08", "2028-07-31"), wantFrom: "2027-07-20", wantUntil: "2028-07-31",
		},
		{
			name: "another approval inside the window becomes the start", student: window("2027-08-01", "2028-07-31"),
			plans: []enrollment.ActivationPlan{plan(holiday, "2027-08-03", "2027-08-14"), plan(schoolYear, "2027-08-01", "2028-07-31")},
			after: phase("2027-08-08", "2028-07-31"), wantFrom: "2027-08-03", wantUntil: "2028-07-31",
		},
		{
			name: "a superseded earlier approval stays out", student: window("2027-08-01", "2028-07-31"),
			plans: []enrollment.ActivationPlan{plan(holiday, "2027-07-20", "2027-07-31"), plan(schoolYear, "2027-08-01", "2028-07-31")},
			after: phase("2027-08-08", "2028-07-31"), wantFrom: "2027-08-08", wantUntil: "2028-07-31",
		},
		{
			name: "another approval keeps the later end", student: window("2027-08-01", "2028-07-31"),
			plans: []enrollment.ActivationPlan{plan(holiday, "2028-07-10", "2028-07-24"), plan(schoolYear, "2027-08-01", "2028-07-31")},
			after: phase("2027-08-01", "2028-07-17"), wantFrom: "2027-08-01", wantUntil: "2028-07-24",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			after := *tt.after
			after.ID = schoolYear
			before := *year
			before.ID = schoolYear
			from, until := followedWindow(tt.student, tt.plans, &before, &after)
			assert.Equal(t, tt.wantFrom, from)
			assert.Equal(t, tt.wantUntil, until)
		})
	}
}
