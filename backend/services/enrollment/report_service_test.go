package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	baseModels "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

func TestCareUsageRowCountsEffectiveDaysAsUnion(t *testing.T) {
	req := &enrollmentModels.Request{
		Model:             baseModels.Model{ID: 10},
		GuardianFirstName: "Eva",
		GuardianLastName:  "Muster",
		GuardianEmail:     "eva@example.test",
	}
	child := &enrollmentModels.RequestChild{
		Model:     baseModels.Model{ID: 20},
		RequestID: 10,
		FirstName: "Lina",
		LastName:  "Muster",
		Status:    enrollmentModels.ChildStatusApproved,
	}
	offerings := map[int64]*enrollmentModels.CareOffering{
		1: {
			Name:           "Regelbetreuung",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		},
		2: {
			Name:           "AG",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
			AvailableDays:  []string{"tue", "wed"},
		},
	}
	links := []*enrollmentModels.RequestChildOffering{
		{RequestChildID: 20, CareOfferingID: 1, SelectedDays: []string{"mon", "tue"}},
		{RequestChildID: 20, CareOfferingID: 2},
	}

	row := careUsageRow(req, child, links, offerings)

	require.Len(t, row.Offerings, 2)
	assert.Equal(t, []string{"mon", "tue", "wed"}, row.EffectiveDays)
	assert.Equal(t, 3, row.DayCount)
	assert.Equal(t, []string{"tue", "wed"}, row.Offerings[0].Days)
	assert.Equal(t, "available", row.Offerings[0].DaysSource)
	assert.Equal(t, []string{"mon", "tue"}, row.Offerings[1].Days)
	assert.Equal(t, "selected", row.Offerings[1].DaysSource)
}

func TestSortedDayCodesDedupesAndOrdersWeekdays(t *testing.T) {
	got := sortedDayCodes([]string{"fri", "mon", "mon", "wed", "tue"})
	assert.Equal(t, []string{"mon", "tue", "wed", "fri"}, got)
}
