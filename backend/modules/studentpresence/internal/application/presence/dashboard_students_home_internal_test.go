package presence

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

type dashboardCountsRepo struct {
	StudentStatusDayRepository
	counts *absencerecords.StudentStatusCounts
}

func (r dashboardCountsRepo) CountEffectiveDashboardAbsences(context.Context, calendar.Date) (*absencerecords.StudentStatusCounts, error) {
	return r.counts, nil
}

// The "Zuhause" tile must name the same children as "Alle Kinder" (#4588):
// active, not checked in, neither sick nor excused nor on a class trip. A
// child checked in despite a sick or excused day sits in the presence AND an
// absence bucket, so the former total-minus-buckets arithmetic subtracted it
// twice and the tile showed one child fewer than the list.
func TestApplyDashboardAbsencesCountsHomeFromCandidates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		counts       absencerecords.StudentStatusCounts
		present      map[int64]bool
		wantHome     int
		wantHomeIDs  []int64
		wantSick     int
		wantExcused  int
		presentCount int
	}{
		{
			name: "at home, present, sick and excused children",
			// 1, 2 at home; 3 present; 4 sick; 5 excused or on a class trip.
			counts:   absencerecords.StudentStatusCounts{Sick: 1, Excused: 1, Total: 5, UnaccountedIDs: []int64{1, 2, 3}},
			present:  map[int64]bool{3: true},
			wantHome: 2, wantHomeIDs: []int64{1, 2}, wantSick: 1, wantExcused: 1, presentCount: 1,
		},
		{
			name: "a sick child checked in is not subtracted from home twice",
			// 1, 2 at home; 3 present; 4 sick AND checked in.
			counts:   absencerecords.StudentStatusCounts{Sick: 1, Total: 4, UnaccountedIDs: []int64{1, 2, 3}},
			present:  map[int64]bool{3: true, 4: true},
			wantHome: 2, wantHomeIDs: []int64{1, 2}, wantSick: 1, presentCount: 2,
		},
		{
			name:     "an excused child checked in is not subtracted from home twice",
			counts:   absencerecords.StudentStatusCounts{Excused: 1, Total: 3, UnaccountedIDs: []int64{1, 2}},
			present:  map[int64]bool{3: true},
			wantHome: 2, wantHomeIDs: []int64{1, 2}, wantExcused: 1, presentCount: 1,
		},
		{
			name:     "everyone present leaves nobody at home",
			counts:   absencerecords.StudentStatusCounts{Total: 2, UnaccountedIDs: []int64{1, 2}},
			present:  map[int64]bool{1: true, 2: true},
			wantHome: 0, wantHomeIDs: []int64{}, presentCount: 2,
		},
		{
			name:     "no students at all",
			counts:   absencerecords.StudentStatusCounts{},
			wantHome: 0, wantHomeIDs: []int64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			counts := tt.counts
			s := &service{StudentStatusRepo: dashboardCountsRepo{counts: &counts}}
			analytics := &DashboardAnalytics{StudentsPresent: tt.presentCount}

			s.applyDashboardAbsences(context.Background(), analytics, calendar.NewDate(2026, 10, 22), tt.present)

			assert.Equal(t, tt.wantHome, analytics.StudentsHome)
			assert.Equal(t, tt.wantHomeIDs, analytics.HomeCandidateIDs)
			assert.Equal(t, tt.wantSick, analytics.StudentsSick)
			assert.Equal(t, tt.wantExcused, analytics.StudentsExcused)
		})
	}
}

// The "Schule" split (#3260) only looks at children the "Zuhause" tile
// counts, so present children must not reach the day-plan check.
func TestHomeCandidateIDsDropsPresentStudents(t *testing.T) {
	t.Parallel()

	got := homeCandidateIDs([]int64{4, 1, 3}, map[int64]bool{1: true, 9: true})
	if len(got) != 2 || got[0] != 4 || got[1] != 3 {
		t.Errorf("homeCandidateIDs = %v, want [4 3]", got)
	}
	if got := homeCandidateIDs(nil, nil); len(got) != 0 {
		t.Errorf("homeCandidateIDs(nil, nil) = %v, want empty", got)
	}
}
