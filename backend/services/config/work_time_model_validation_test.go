package config_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validModel returns a minimally valid work-time model (no entries).
func validModel() *configModels.WorkTimeModel {
	return &configModels.WorkTimeModel{
		Name:               "Validation test",
		RotationLength:     1,
		RotationAnchorDate: timezone.NewDate(2026, time.June, 1),
	}
}

func TestValidateModelWithEntries_RejectsInvalidEntries(t *testing.T) {
	svc := &configSvc.WorkTimeModelService{}

	tests := []struct {
		name  string
		entry *configModels.WorkTimeModelEntry
	}{
		{
			name: "day outside week",
			entry: &configModels.WorkTimeModelEntry{
				WeekIndex:     0,
				DayOfWeek:     7,
				TargetMinutes: 300,
			},
		},
		{
			name: "week outside rotation",
			entry: &configModels.WorkTimeModelEntry{
				WeekIndex:     1,
				DayOfWeek:     configModels.DayMonday,
				TargetMinutes: 300,
			},
		},
		{
			name: "minutes too large",
			entry: &configModels.WorkTimeModelEntry{
				WeekIndex:     0,
				DayOfWeek:     configModels.DayMonday,
				TargetMinutes: 721,
			},
		},
		{
			name: "negative minutes",
			entry: &configModels.WorkTimeModelEntry{
				WeekIndex:     0,
				DayOfWeek:     configModels.DayMonday,
				TargetMinutes: -1,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.ValidateModelWithEntries(validModel(), []*configModels.WorkTimeModelEntry{tc.entry})
			require.Error(t, err)
		})
	}
}

func TestValidateModelWithEntries_RejectsDuplicateEntries(t *testing.T) {
	svc := &configSvc.WorkTimeModelService{}

	err := svc.ValidateModelWithEntries(validModel(), []*configModels.WorkTimeModelEntry{
		{WeekIndex: 0, DayOfWeek: configModels.DayMonday, TargetMinutes: 300},
		{WeekIndex: 0, DayOfWeek: configModels.DayMonday, TargetMinutes: 240},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}
