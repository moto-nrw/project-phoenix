package schedule

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func capacityOccurrence(date timezone.Date, children, staff int) activities.TemplateCapacityOccurrence {
	return activities.TemplateCapacityOccurrence{
		TemplateID:      42,
		OccurrenceDate:  date,
		EnrollmentCount: children,
		SupervisorCount: staff,
	}
}

func TestWorstTemplateOccurrenceRanking(t *testing.T) {
	jan1 := timezone.NewDate(2026, 1, 1)

	tests := []struct {
		name        string
		occurrences []activities.TemplateCapacityOccurrence
		wantDate    timezone.Date
	}{
		{
			name: "danger outranks a warning even with a smaller shortage",
			occurrences: []activities.TemplateCapacityOccurrence{
				capacityOccurrence(jan1, 12, 0),
				capacityOccurrence(jan1.AddDays(1), 36, 1),
			},
			wantDate: jan1,
		},
		{
			name: "larger shortage wins within one severity",
			occurrences: []activities.TemplateCapacityOccurrence{
				capacityOccurrence(jan1, 12, 0),
				capacityOccurrence(jan1.AddDays(1), 25, 0),
			},
			wantDate: jan1.AddDays(1),
		},
		{
			name: "smaller surplus wins among covered occurrences",
			occurrences: []activities.TemplateCapacityOccurrence{
				capacityOccurrence(jan1, 12, 3),
				capacityOccurrence(jan1.AddDays(1), 12, 1),
			},
			wantDate: jan1.AddDays(1),
		},
		{
			name: "higher requirement breaks equal coverage ties",
			occurrences: []activities.TemplateCapacityOccurrence{
				capacityOccurrence(jan1, 12, 1),
				capacityOccurrence(jan1.AddDays(1), 24, 2),
			},
			wantDate: jan1.AddDays(1),
		},
		{
			name: "earliest date is deterministic final tie breaker",
			occurrences: []activities.TemplateCapacityOccurrence{
				capacityOccurrence(jan1.AddDays(7), 12, 0),
				capacityOccurrence(jan1, 12, 0),
			},
			wantDate: jan1,
		},
		{
			name: "zero demand is ignored when a demanded occurrence exists",
			occurrences: []activities.TemplateCapacityOccurrence{
				capacityOccurrence(jan1, 0, 0),
				capacityOccurrence(jan1.AddDays(1), 12, 1),
			},
			wantDate: jan1.AddDays(1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := worstTemplateOccurrence(tt.occurrences, 12)
			require.True(t, ok)
			assert.Equal(t, tt.wantDate, got.OccurrenceDate)
		})
	}
}

func TestApplyWorstTemplateCapacityClearsTemplatesWithoutOccurrences(t *testing.T) {
	rows := []activities.TemplateListRow{
		{TemplateID: 1, CapacityEnrollmentCount: 8, CapacitySupervisorCount: 2},
		{TemplateID: 2, CapacityEnrollmentCount: 9, CapacitySupervisorCount: 3},
	}
	occurrences := []activities.TemplateCapacityOccurrence{
		{TemplateID: 1, OccurrenceDate: timezone.NewDate(2026, 1, 5), EnrollmentCount: 4, SupervisorCount: 1},
	}

	applyWorstTemplateCapacity(rows, []int64{1, 2}, occurrences, 12)

	assert.Equal(t, 4, rows[0].CapacityEnrollmentCount)
	assert.Equal(t, 1, rows[0].CapacitySupervisorCount)
	assert.Zero(t, rows[1].CapacityEnrollmentCount)
	assert.Zero(t, rows[1].CapacitySupervisorCount)
}
