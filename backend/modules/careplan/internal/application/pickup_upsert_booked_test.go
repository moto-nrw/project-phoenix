package application

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// recordingPickupRows keeps the stored weekly rows of one student in memory.
type recordingPickupRows struct {
	ports.PickupScheduleRepository
	rows    []*careplan.PickupSchedule
	upserts []*careplan.PickupSchedule
	deleted []int64
}

func (r *recordingPickupRows) FindByStudentID(context.Context, int64) ([]*careplan.PickupSchedule, error) {
	return r.rows, nil
}

func (r *recordingPickupRows) UpsertSchedule(_ context.Context, row *careplan.PickupSchedule) error {
	r.upserts = append(r.upserts, row)
	return nil
}

func (r *recordingPickupRows) Delete(_ context.Context, id int64) error {
	r.deleted = append(r.deleted, id)
	return nil
}

// bookedPickup projects the booked offering's pickup time on every date.
type bookedPickup struct {
	careplan.PickupBaselineReader
	clock string
}

func (b bookedPickup) Project(_ context.Context, ids []int64, from, _ calendar.Date) (*careplan.PickupBaselineProjection, error) {
	offerings := careplan.PickupPlansByStudent{}
	if b.clock != "" {
		for _, id := range ids {
			offerings[id] = careplan.PickupPlanByDate{from: careplan.PickupWeek{
				domain.ISOWeekday(from): {StudentID: id, PickupTime: wallClock(b.clock), Source: careplan.ScheduleSourceCareOffering},
			}}
		}
	}
	return &careplan.PickupBaselineProjection{OfferingByStudentDate: offerings}, nil
}

func wallClock(hhmm string) time.Time {
	parsed, err := time.Parse("15:04", hhmm)
	if err != nil {
		panic(err)
	}
	return calendar.NormalizeWallClock(parsed)
}

// A single weekday write (an approved parent request) that only repeats the
// booked offering's pickup time must not become a staff override: the
// override would keep the old time over a later booking change.
func TestUpsertStudentPickupScheduleKeepsBookedTimeAProjection(t *testing.T) {
	t.Parallel()
	notes := "Fußball"
	for name, tc := range map[string]struct {
		booked     string
		stored     []*careplan.PickupSchedule
		row        *careplan.PickupSchedule
		wantUpsert bool
		wantDelete []int64
	}{
		"booked time without override stores nothing": {
			booked: "14:30",
			row:    &careplan.PickupSchedule{StudentID: 7, Weekday: 1, PickupTime: wallClock("14:30"), CreatedBy: 3},
		},
		"booked time replaces an older override": {
			booked:     "14:30",
			stored:     []*careplan.PickupSchedule{{ID: 11, StudentID: 7, Weekday: 1, PickupTime: wallClock("15:15"), Source: careplan.ScheduleSourceStaff}},
			row:        &careplan.PickupSchedule{StudentID: 7, Weekday: 1, PickupTime: wallClock("14:30"), CreatedBy: 3},
			wantDelete: []int64{11},
		},
		"legacy offering row stays untouched": {
			booked: "14:30",
			stored: []*careplan.PickupSchedule{{ID: 12, StudentID: 7, Weekday: 1, PickupTime: wallClock("16:00"), Source: careplan.ScheduleSourceCareOffering}},
			row:    &careplan.PickupSchedule{StudentID: 7, Weekday: 1, PickupTime: wallClock("14:30"), CreatedBy: 3},
		},
		"different time stays an override": {
			booked:     "14:30",
			row:        &careplan.PickupSchedule{StudentID: 7, Weekday: 1, PickupTime: wallClock("15:00"), CreatedBy: 3},
			wantUpsert: true,
		},
		"time with notes stays an override": {
			booked:     "14:30",
			row:        &careplan.PickupSchedule{StudentID: 7, Weekday: 1, PickupTime: wallClock("14:30"), Notes: &notes, CreatedBy: 3},
			wantUpsert: true,
		},
		"no booked offering stores the time": {
			row:        &careplan.PickupSchedule{StudentID: 7, Weekday: 1, PickupTime: wallClock("14:30"), CreatedBy: 3},
			wantUpsert: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rows := &recordingPickupRows{rows: tc.stored}
			service := NewPickupSchedules(rows, nil, nil, passThroughTransaction{}, bookedPickup{clock: tc.booked}, nil, nil, nil, nil)

			require.NoError(t, service.UpsertStudentPickupSchedule(context.Background(), tc.row))

			if tc.wantUpsert {
				require.Len(t, rows.upserts, 1)
				assert.Same(t, tc.row, rows.upserts[0])
			} else {
				assert.Empty(t, rows.upserts)
			}
			assert.Equal(t, tc.wantDelete, rows.deleted)
		})
	}
}

// A row the plain upsert would reject must not delete the stored override
// just because its time repeats the booking.
func TestUpsertStudentPickupScheduleValidatesBeforeRemovingAnOverride(t *testing.T) {
	t.Parallel()
	rows := &recordingPickupRows{rows: []*careplan.PickupSchedule{
		{ID: 11, StudentID: 7, Weekday: 1, PickupTime: wallClock("15:15"), Source: careplan.ScheduleSourceStaff},
	}}
	service := NewPickupSchedules(rows, nil, nil, passThroughTransaction{}, bookedPickup{clock: "14:30"}, nil, nil, nil, nil)

	err := service.UpsertStudentPickupSchedule(context.Background(),
		&careplan.PickupSchedule{StudentID: 7, Weekday: 1, PickupTime: wallClock("14:30")})

	require.ErrorContains(t, err, "created_by is required")
	assert.Empty(t, rows.deleted)
	assert.Empty(t, rows.upserts)
}

func TestNextISOWeekday(t *testing.T) {
	t.Parallel()
	wednesday := calendar.NewDate(2026, 11, 4)
	assert.Equal(t, wednesday, nextISOWeekday(wednesday, 3))
	assert.Equal(t, calendar.NewDate(2026, 11, 5), nextISOWeekday(wednesday, 4))
	assert.Equal(t, calendar.NewDate(2026, 11, 9), nextISOWeekday(wednesday, 1))
}
