package timetracking

import (
	"context"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HasTarget tells "no Soll maintained" apart from a Soll of zero (#6887). It
// comes from the same resolver that prices the month, so it follows the same
// source order: Sonderarbeitszeit, schedule, and the work-time model only
// while no schedule was ever written.
func TestWTMMonthSummary_HasTarget(t *testing.T) {
	t.Parallel()

	modelID := int64(9)
	mondayModel := &WorkTimeTemplate{
		RotationLength:     1,
		RotationAnchorDate: NewDate(2020, time.January, 1),
		Entries:            []*WorkTimeTemplateEntry{{WeekIndex: 0, DayOfWeek: DayMonday, TargetMinutes: 300}},
	}
	withModel := func(f *wtmFixture) {
		f.svc.staffRepo = &wtmMockStaffReader{staff: &StaffScheduleAssignment{WorkTimeModelID: &modelID}}
		f.models.model = mondayModel
	}

	cases := []struct {
		name    string
		arrange func(f *wtmFixture)
		want    bool
	}{
		{name: "personal schedule", arrange: func(*wtmFixture) {}, want: true},
		{name: "schedule with zero minutes is still maintained", arrange: func(f *wtmFixture) {
			f.schedules.entries[0].TargetMinutes = 0
		}, want: true},
		{name: "nothing maintained", arrange: func(f *wtmFixture) {
			f.schedules.entries = nil
		}, want: false},
		{name: "work-time model without schedule history", arrange: func(f *wtmFixture) {
			f.schedules.entries = nil
			withModel(f)
		}, want: true},
		{name: "schedule history without coverage ignores the model", arrange: func(f *wtmFixture) {
			f.schedules.entries = nil
			f.schedules.hasHistory = true
			withModel(f)
		}, want: false},
		{name: "schedule ended before the month", arrange: func(f *wtmFixture) {
			validUntil := NewDate(2026, time.May, 1)
			f.schedules.entries[0].ValidUntil = &validUntil
		}, want: false},
		{name: "Sonderarbeitszeit only", arrange: func(f *wtmFixture) {
			f.schedules.entries = nil
			f.svc.overrideReader = &wtmMockOverrideReader{days: workforce.TargetOverrideDays{
				wtmStaffID: {NewDate(2026, time.June, 10).String(): 240},
			}}
		}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newWTMFixture()
			tc.arrange(f)

			summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
			require.NoError(t, err)
			assert.Equal(t, tc.want, summary.HasTarget)
		})
	}
}
