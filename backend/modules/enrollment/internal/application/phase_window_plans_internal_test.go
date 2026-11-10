package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// followedWindow proposes the window of a pending child whose bounds still
// equal the phase's old ones. A child with approvals in several phases starts
// with the earliest and ends with the latest approval inside the old window;
// an approval the school-year window replaced stays out (#6505). Only other
// bounds outside the old phase go along for widening the student's window.
func TestFollowedWindow(t *testing.T) {
	t.Parallel()

	const schoolYear, holiday int64 = 1, 2
	phase := func(start, end string) *enrollment.Phase {
		return &enrollment.Phase{ServiceStartDate: enrollment.Date(start), ServiceEndDate: enrollment.Date(end)}
	}
	other := func(start, end string) *enrollment.StudentCarePeriod {
		return &enrollment.StudentCarePeriod{PhaseID: holiday, ServiceStartDate: enrollment.Date(start), ServiceEndDate: enrollment.Date(end)}
	}
	year := phase("2027-08-01", "2028-07-31")
	tests := []struct {
		name            string
		others          []*enrollment.StudentCarePeriod
		before          *enrollment.Phase
		after           *enrollment.Phase
		wantFrom        string
		wantUntil       string
		wantOtherStarts []enrollment.Date
		wantOtherEnds   []enrollment.Date
	}{
		{
			name:  "later start",
			after: phase("2027-08-08", "2028-07-31"), wantFrom: "2027-08-08", wantUntil: "2028-07-31",
		},
		{
			name:  "earlier start and end",
			after: phase("2027-07-25", "2028-07-24"), wantFrom: "2027-07-25", wantUntil: "2028-07-24",
		},
		{
			name:   "another approval inside the window becomes the start",
			others: []*enrollment.StudentCarePeriod{other("2027-08-03", "2027-08-14")},
			after:  phase("2027-08-08", "2028-07-31"), wantFrom: "2027-08-03", wantUntil: "2028-07-31",
		},
		{
			name:   "a superseded earlier approval stays out",
			others: []*enrollment.StudentCarePeriod{other("2027-07-20", "2027-07-31")},
			after:  phase("2027-08-08", "2028-07-31"), wantFrom: "2027-08-08", wantUntil: "2028-07-31",
			wantOtherStarts: []enrollment.Date{"2027-07-20"},
		},
		{
			name:   "another approval keeps the later end",
			others: []*enrollment.StudentCarePeriod{other("2028-07-10", "2028-07-24")},
			after:  phase("2027-08-01", "2028-07-17"), wantFrom: "2027-08-01", wantUntil: "2028-07-24",
		},
		{
			name:   "editing a superseded earlier phase cannot widen the later start",
			others: []*enrollment.StudentCarePeriod{other("2027-08-01", "2028-07-31")},
			before: phase("2027-07-20", "2027-07-31"),
			after:  phase("2027-07-21", "2027-07-31"), wantFrom: "2027-07-21", wantUntil: "2027-07-31",
			wantOtherEnds: []enrollment.Date{"2028-07-31"},
		},
		{
			name:   "editing a superseded later phase cannot widen the earlier end",
			others: []*enrollment.StudentCarePeriod{other("2027-08-01", "2028-07-31")},
			before: phase("2028-08-01", "2028-08-14"),
			after:  phase("2028-08-01", "2028-08-15"), wantFrom: "2028-08-01", wantUntil: "2028-08-15",
			wantOtherStarts: []enrollment.Date{"2027-08-01"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			after := *tt.after
			after.ID = schoolYear
			before := *year
			if tt.before != nil {
				before = *tt.before
			}
			before.ID = schoolYear
			move := followedWindow(tt.others, &before, &after)
			assert.Equal(t, enrollment.Date(tt.wantFrom), move.FromAfter)
			assert.Equal(t, enrollment.Date(tt.wantUntil), move.UntilAfter)
			assert.Equal(t, before.ServiceStartDate, move.FromBefore)
			assert.Equal(t, before.ServiceEndDate, move.UntilBefore)
			assert.Equal(t, tt.wantOtherStarts, move.OtherStarts)
			assert.Equal(t, tt.wantOtherEnds, move.OtherEnds)
		})
	}
}
