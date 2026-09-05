package schedule

import (
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommonScheduleValidityEnvelope(t *testing.T) {
	t.Parallel()

	validFrom := timezone.NewDate(2026, time.July, 6)
	validUntil := timezone.NewDate(2026, time.August, 17)

	tests := []struct {
		name      string
		schedules []*activitiesModel.Schedule
		wantFrom  *timezone.Date
		wantUntil *timezone.Date
	}{
		{name: "no schedules is an open envelope"},
		{
			name:      "open start and end",
			schedules: []*activitiesModel.Schedule{{}, {}},
		},
		{
			name:      "start bounded successor",
			schedules: []*activitiesModel.Schedule{{ValidFrom: activityDatePtr(&validFrom)}, {ValidFrom: activityDatePtr(&validFrom)}},
			wantFrom:  &validFrom,
		},
		{
			name:      "end bounded predecessor",
			schedules: []*activitiesModel.Schedule{{ValidUntil: activityDatePtr(&validUntil)}, {ValidUntil: activityDatePtr(&validUntil)}},
			wantUntil: &validUntil,
		},
		{
			name: "fully bounded segment",
			schedules: []*activitiesModel.Schedule{
				{ValidFrom: activityDatePtr(&validFrom), ValidUntil: activityDatePtr(&validUntil)},
				{ValidFrom: activityDatePtr(&validFrom), ValidUntil: activityDatePtr(&validUntil)},
			},
			wantFrom:  &validFrom,
			wantUntil: &validUntil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFrom, gotUntil, err := commonScheduleValidityEnvelope(tt.schedules)
			require.NoError(t, err)
			assert.Equal(t, tt.wantFrom, gotFrom)
			assert.Equal(t, tt.wantUntil, gotUntil)
			if gotFrom != nil && tt.wantFrom != nil {
				assert.NotSame(t, tt.wantFrom, gotFrom, "the preserved envelope must own its date pointers")
			}
			if gotUntil != nil && tt.wantUntil != nil {
				assert.NotSame(t, tt.wantUntil, gotUntil, "the preserved envelope must own its date pointers")
			}
		})
	}
}

func TestCommonScheduleValidityEnvelopeRejectsInconsistentRows(t *testing.T) {
	t.Parallel()

	validFrom := timezone.NewDate(2026, time.July, 6)
	validUntil := timezone.NewDate(2026, time.August, 17)
	invertedFrom := validUntil.AddDays(1)

	tests := []struct {
		name      string
		schedules []*activitiesModel.Schedule
	}{
		{
			name:      "different start",
			schedules: []*activitiesModel.Schedule{{ValidFrom: activityDatePtr(&validFrom)}, {}},
		},
		{
			name:      "different end",
			schedules: []*activitiesModel.Schedule{{ValidUntil: activityDatePtr(&validUntil)}, {}},
		},
		{
			name:      "nil row",
			schedules: []*activitiesModel.Schedule{{}, nil},
		},
		{
			name: "inverted envelope",
			schedules: []*activitiesModel.Schedule{{
				ValidFrom:  activityDatePtr(&invertedFrom),
				ValidUntil: activityDatePtr(&validUntil),
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := commonScheduleValidityEnvelope(tt.schedules)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInconsistentTemplateScheduleValidity))
		})
	}
}
