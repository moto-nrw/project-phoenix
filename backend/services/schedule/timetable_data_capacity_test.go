package schedule

import (
	"database/sql"
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

func TestUnionCountDeduplicatesRosterAndTargetStudents(t *testing.T) {
	assert.Equal(t, 3, unionCount([]int64{1, 2}, []int64{2, 3}))
	assert.Zero(t, unionCount(nil, nil))
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
			// nil override => derived Betreuungsschlüssel scoring (unchanged
			// behaviour these cases assert); the override path is covered
			// separately in TestWorstTemplateOccurrence_WithOverride.
			got, ok := worstTemplateOccurrence(tt.occurrences, nil, 12)
			require.True(t, ok)
			assert.Equal(t, tt.wantDate, got.OccurrenceDate)
		})
	}
}

func TestWorstTemplateOccurrence_WithOverride(t *testing.T) {
	jan1 := timezone.NewDate(2026, 1, 1)
	// A manual override of 3 applies uniformly to every occurrence. A
	// fully-staffed 3/3 day must NOT hide a 0/3 day that only looks fine under
	// the derived requirement (#1839).
	occurrences := []activities.TemplateCapacityOccurrence{
		capacityOccurrence(jan1, 30, 3),           // 3/3 under override -> fine
		capacityOccurrence(jan1.AddDays(1), 0, 0), // 0/3 under override -> danger
	}

	// Derived scoring (ratio 10) scores the empty day as required 0 and skips
	// it as zero-demand, so it wrongly reports the 3/3 day as the worst — the
	// exact false "all good" state the reviewer flagged.
	derived, ok := worstTemplateOccurrence(occurrences, nil, 10)
	require.True(t, ok)
	assert.Equal(t, jan1, derived.OccurrenceDate, "derived: empty day skipped, 3/3 day looks worst")

	// With the override the empty 0/3 day is correctly the worst.
	overridden, ok := worstTemplateOccurrence(occurrences, intPtr(3), 10)
	require.True(t, ok)
	assert.Equal(t, jan1.AddDays(1), overridden.OccurrenceDate, "override: 0/3 day is the worst")
}

func TestApplyWorstTemplateCapacity_OverrideParticipatesInScoring(t *testing.T) {
	// Template 1 has a manual override of 3; its two occurrences are a
	// fully-staffed 3/3 day and an empty 0/3 day. The override must steer the
	// worst-occurrence pick to the 0/3 day so the list shows the true shortfall.
	rows := []activities.TemplateListRow{
		{TemplateID: 1, RequiredStaff: sql.NullInt64{Int64: 3, Valid: true}},
	}
	occurrences := []activities.TemplateCapacityOccurrence{
		{TemplateID: 1, OccurrenceDate: timezone.NewDate(2026, 1, 1), EnrollmentCount: 30, SupervisorCount: 3},
		{TemplateID: 1, OccurrenceDate: timezone.NewDate(2026, 1, 2), EnrollmentCount: 0, SupervisorCount: 0},
	}

	applyWorstTemplateCapacity(rows, []int64{1}, occurrences, 10)

	assert.Equal(t, 0, rows[0].CapacityEnrollmentCount, "worst = the empty 0/3 day")
	assert.Equal(t, 0, rows[0].CapacitySupervisorCount)
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

func TestApplyWorstTemplateCapacity_MarksOccurrenceFound(t *testing.T) {
	// Template 1 has a real occurrence, template 2 has none (every date
	// cancelled / AB-week-filtered / unmaterializable). The flag lets the
	// handler suppress the manual required_staff override for template 2 so a
	// never-occurring block cannot show a false 0/N understaffing indicator.
	rows := []activities.TemplateListRow{
		{TemplateID: 1, RequiredStaff: sql.NullInt64{Int64: 3, Valid: true}},
		{TemplateID: 2, RequiredStaff: sql.NullInt64{Int64: 3, Valid: true}},
	}
	occurrences := []activities.TemplateCapacityOccurrence{
		{TemplateID: 1, OccurrenceDate: timezone.NewDate(2026, 1, 5), EnrollmentCount: 4, SupervisorCount: 1},
	}

	applyWorstTemplateCapacity(rows, []int64{1, 2}, occurrences, 12)

	assert.True(t, rows[0].CapacityOccurrenceFound)
	assert.False(t, rows[1].CapacityOccurrenceFound)
}
