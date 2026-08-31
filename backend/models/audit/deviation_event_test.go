package audit

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviationEventValidate(t *testing.T) {
	t.Parallel()

	groupID := int64(11)
	instanceID := int64(12)
	shiftID := int64(13)
	date := timezone.NewDate(2026, 5, 4)

	tests := []struct {
		name  string
		event DeviationEvent
		want  string
	}{
		{name: "valid template-backed event", event: DeviationEvent{ActivityGroupID: &groupID, OccurrenceDate: date, EventType: DeviationEventAbsence}},
		{name: "valid spontaneous event", event: DeviationEvent{InstanceID: &instanceID, OccurrenceDate: date, EventType: DeviationEventCancellation}},
		// #1884: a Dienstplan shift move anchors via staff_shift_id — neither
		// activity slot pointer exists for a shift.
		{name: "valid shift-anchored event", event: DeviationEvent{StaffShiftID: &shiftID, OccurrenceDate: date, EventType: DeviationEventShiftMoved}},
		{name: "requires anchor", event: DeviationEvent{OccurrenceDate: date, EventType: DeviationEventAbsence}, want: "activity_group_id, instance_id or staff_shift_id is required"},
		{name: "rejects zero anchor", event: DeviationEvent{ActivityGroupID: new(int64), OccurrenceDate: date, EventType: DeviationEventAbsence}, want: "activity_group_id, instance_id or staff_shift_id is required"},
		{name: "rejects zero shift anchor", event: DeviationEvent{StaffShiftID: new(int64), OccurrenceDate: date, EventType: DeviationEventShiftMoved}, want: "activity_group_id, instance_id or staff_shift_id is required"},
		{name: "requires occurrence date", event: DeviationEvent{InstanceID: &instanceID, EventType: DeviationEventAbsence}, want: "occurrence_date is required"},
		{name: "requires event type", event: DeviationEvent{InstanceID: &instanceID, OccurrenceDate: date}, want: "event_type is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.want)
		})
	}
}

// TestShiftScopedDeviationEventTypesNonEmpty guards the slot-history filter:
// ListByRange renders the list as `event_type NOT IN (?)` — an empty list
// would produce `NOT IN ()`, which is invalid SQL and breaks the
// Betreuungsplan history query at runtime. Whoever removes the last entry
// must also remove the exclusion filter in the repository.
func TestShiftScopedDeviationEventTypesNonEmpty(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, ShiftScopedDeviationEventTypes)
}
