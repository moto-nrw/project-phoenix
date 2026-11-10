package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// followedWindow proposes the window of a pending child whose bounds still
// equal the phase's old ones. A child with approvals in several phases starts
// with the earliest and ends with the latest approval inside the old window;
// an approval the school-year window replaced stays out (#6505). Every other
// approval goes along, so School Membership can widen a window it shapes.
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
		name      string
		others    []*enrollment.StudentCarePeriod
		after     *enrollment.Phase
		wantFrom  string
		wantUntil string
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
		},
		{
			name:   "another approval keeps the later end",
			others: []*enrollment.StudentCarePeriod{other("2028-07-10", "2028-07-24")},
			after:  phase("2027-08-01", "2028-07-17"), wantFrom: "2027-08-01", wantUntil: "2028-07-24",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			after := *tt.after
			after.ID = schoolYear
			before := *year
			before.ID = schoolYear
			move := followedWindow(tt.others, &before, &after)
			assert.Equal(t, enrollment.Date(tt.wantFrom), move.FromAfter)
			assert.Equal(t, enrollment.Date(tt.wantUntil), move.UntilAfter)
			assert.Equal(t, before.ServiceStartDate, move.FromBefore)
			assert.Equal(t, before.ServiceEndDate, move.UntilBefore)
			assert.Len(t, move.OtherStarts, len(tt.others), "every other approval goes along")
		})
	}
}
