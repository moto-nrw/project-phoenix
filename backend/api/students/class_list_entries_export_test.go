package students

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/stretchr/testify/assert"
)

// Explicit "all" values are the student side's "no filter" sentinels
// (isActiveFilterValue): a request carrying them keeps every student, so it
// must keep the class-list entries too (#2399 review round 9).
func TestClassListEntryExportEligibleTreatsAllAsInactive(t *testing.T) {
	eligible := classListEntryExportEligible(listexport.PresetClassRoster, studentExportFilters{
		Status:       "all",
		Bus:          "all",
		PhotoConsent: "all",
		PickupStatus: "all",
		DayStatus:    DayPlanningStatusAll,
	})
	assert.True(t, eligible, "explicit 'all' filters keep every student and must keep the entries")

	assert.True(t, classListEntryExportEligible(listexport.PresetClassRoster, studentExportFilters{}),
		"an unfiltered class roster carries the entries")
}

// Any genuinely active filter targets a property an entry does not have — the
// entries drop out, and a non-class-roster preset never carries them.
func TestClassListEntryExportEligibleActiveFilterExcludesEntries(t *testing.T) {
	tests := []struct {
		name    string
		filters studentExportFilters
	}{
		{"status", studentExportFilters{Status: "klassenfahrt"}},
		{"bus", studentExportFilters{Bus: "yes"}},
		{"photo_consent", studentExportFilters{PhotoConsent: "no"}},
		{"pickup_status", studentExportFilters{PickupStatus: "self"}},
		{"day_status", studentExportFilters{DayStatus: DayPlanningStatusComesToday}},
		{"group", studentExportFilters{GroupID: "5"}},
		{"room", studentExportFilters{RoomID: "7"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, classListEntryExportEligible(listexport.PresetClassRoster, tt.filters))
		})
	}

	assert.False(t, classListEntryExportEligible(listexport.PresetOGSWeekly, studentExportFilters{}),
		"only the class roster preset carries entries")
}
