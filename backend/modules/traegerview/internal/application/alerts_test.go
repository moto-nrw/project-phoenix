package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/traegerview"
)

func TestSchoolAlertsAppliesEachThreshold(t *testing.T) {
	t.Parallel()
	thresholds := traegerview.AlertThresholds{ChildrenPerStaff: 10, SickPercent: 10, OpenEnrollments: 2}
	cases := []struct {
		name    string
		figures traegerview.SchoolFigures
		open    int
		want    []string
	}{
		{name: "within every threshold", figures: traegerview.SchoolFigures{ChildrenTotal: 40, ChildrenPresent: 20, StaffPresent: 2, ChildrenSick: 3}, open: 1, want: []string{}},
		{name: "exactly the care ratio is no shortage", figures: traegerview.SchoolFigures{ChildrenTotal: 40, ChildrenPresent: 20, StaffPresent: 2}, want: []string{}},
		{name: "one child more than the care ratio", figures: traegerview.SchoolFigures{ChildrenTotal: 40, ChildrenPresent: 21, StaffPresent: 2}, want: []string{traegerview.AlertStaffShortage}},
		{name: "children without staff", figures: traegerview.SchoolFigures{ChildrenTotal: 40, ChildrenPresent: 1}, want: []string{traegerview.AlertStaffShortage}},
		{name: "no children, no staff is quiet", figures: traegerview.SchoolFigures{ChildrenTotal: 40}, want: []string{}},
		{name: "sick share reaches the threshold", figures: traegerview.SchoolFigures{ChildrenTotal: 40, ChildrenSick: 4}, want: []string{traegerview.AlertSickChildren}},
		{name: "sick share below the threshold", figures: traegerview.SchoolFigures{ChildrenTotal: 41, ChildrenSick: 4}, want: []string{}},
		{name: "waiting children reach the threshold", open: 2, want: []string{traegerview.AlertOpenEnrollments}},
		{
			name:    "every hint in the order of the kinds",
			figures: traegerview.SchoolFigures{ChildrenTotal: 10, ChildrenPresent: 5, ChildrenSick: 5},
			open:    7,
			want:    []string{traegerview.AlertStaffShortage, traegerview.AlertSickChildren, traegerview.AlertOpenEnrollments},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, schoolAlerts(tc.figures, tc.open, thresholds))
		})
	}
}

func TestSchoolAlertsZeroSwitchesAHintOff(t *testing.T) {
	t.Parallel()
	figures := traegerview.SchoolFigures{ChildrenTotal: 10, ChildrenPresent: 5, ChildrenSick: 10}

	assert.Equal(t, []string{}, schoolAlerts(figures, 50, traegerview.AlertThresholds{}))
	assert.Equal(t, []string{traegerview.AlertSickChildren},
		schoolAlerts(figures, 50, traegerview.AlertThresholds{SickPercent: 1}))
}
