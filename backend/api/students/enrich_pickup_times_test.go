package students

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// mockPickupScheduleService implements only GetBulkEffectivePickupTimesForDate.
// All other methods panic — they are not used by enrichWithPickupTimes.
type mockPickupScheduleService struct {
	scheduleService.PickupScheduleService // embed interface to satisfy compiler for unused methods
	bulkResult                            map[int64]*scheduleService.EffectivePickupTime
	bulkErr                               error
	calledWithIDs                         []int64
}

type mockArrivalScheduleService struct {
	scheduleService.ArrivalScheduleService // embed interface to satisfy compiler for unused methods
	bulkResult                             map[int64]*scheduleService.EffectiveArrivalTime
	bulkErr                                error
	calledWithIDs                          []int64
}

func (m *mockPickupScheduleService) GetBulkEffectivePickupTimesForDate(_ context.Context, studentIDs []int64, _ timezone.Date) (map[int64]*scheduleService.EffectivePickupTime, error) {
	m.calledWithIDs = studentIDs
	return m.bulkResult, m.bulkErr
}

func (m *mockArrivalScheduleService) GetBulkEffectiveArrivalTimesForDate(_ context.Context, studentIDs []int64, _ timezone.Date) (map[int64]*scheduleService.EffectiveArrivalTime, error) {
	m.calledWithIDs = studentIDs
	return m.bulkResult, m.bulkErr
}

// Stub out the embedded interface methods that would panic on nil receiver.
func (m *mockPickupScheduleService) GetStudentPickupSchedules(_ context.Context, _ int64) ([]*schedule.StudentPickupSchedule, error) {
	return nil, nil
}

func TestEnrichWithPickupTimes_FullAccessGating(t *testing.T) {
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)
	pickupAt := time.Date(2026, 4, 14, 15, 30, 0, 0, time.UTC)

	mock := &mockPickupScheduleService{
		bulkResult: map[int64]*scheduleService.EffectivePickupTime{
			100: {PickupTime: &pickupAt},
			200: {PickupTime: &pickupAt},
			300: {PickupTime: &pickupAt},
		},
	}

	rs := &Resource{ResourceConfig: ResourceConfig{PickupScheduleService: mock, Logger: slog.Default()}}

	responses := []StudentResponse{
		{ID: 100, HasFullAccess: true},
		{ID: 200, HasFullAccess: false}, // Should NOT get pickup time
		{ID: 300, HasFullAccess: true},
	}

	// Only pass full-access IDs (mirrors the caller in listStudents)
	fullAccessIDs := []int64{100, 300}

	rs.enrichWithPickupTimes(context.Background(), responses, fullAccessIDs, now)

	// Full-access students get their pickup time
	assert.NotNil(t, responses[0].PickupTime, "full-access student 100 should have pickup time")
	assert.Equal(t, "15:30", *responses[0].PickupTime)

	// Non-full-access student must NOT have pickup time (GDPR)
	assert.Nil(t, responses[1].PickupTime, "non-full-access student 200 must not have pickup time")

	// Second full-access student also gets pickup time
	assert.NotNil(t, responses[2].PickupTime, "full-access student 300 should have pickup time")
	assert.Equal(t, "15:30", *responses[2].PickupTime)

	// Verify the service was called only with full-access IDs
	assert.Equal(t, []int64{100, 300}, mock.calledWithIDs, "should only query pickup times for full-access students")
}

func TestEnrichWithPickupTimes_NilService(t *testing.T) {
	rs := &Resource{ResourceConfig: ResourceConfig{PickupScheduleService: nil, Logger: slog.Default()}}

	responses := []StudentResponse{
		{ID: 100, HasFullAccess: true},
	}

	// Should not panic
	rs.enrichWithPickupTimes(context.Background(), responses, []int64{100}, time.Now())
	assert.Nil(t, responses[0].PickupTime, "should be nil when service is nil")
}

func TestEnrichWithPickupTimes_EmptyIDs(t *testing.T) {
	mock := &mockPickupScheduleService{}
	rs := &Resource{ResourceConfig: ResourceConfig{PickupScheduleService: mock, Logger: slog.Default()}}

	responses := []StudentResponse{}
	rs.enrichWithPickupTimes(context.Background(), responses, []int64{}, time.Now())

	// Should not have called the service
	assert.Nil(t, mock.calledWithIDs, "should not call service with empty IDs")
}

func TestEnrichWithPickupTimes_NoPickupTime(t *testing.T) {
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)

	mock := &mockPickupScheduleService{
		bulkResult: map[int64]*scheduleService.EffectivePickupTime{
			100: {PickupTime: nil}, // Student has no pickup schedule
		},
	}

	rs := &Resource{ResourceConfig: ResourceConfig{PickupScheduleService: mock, Logger: slog.Default()}}

	responses := []StudentResponse{
		{ID: 100, HasFullAccess: true},
	}

	rs.enrichWithPickupTimes(context.Background(), responses, []int64{100}, now)

	assert.Nil(t, responses[0].PickupTime, "should be nil when no pickup time is set")
}

func TestEnrichWithPickupTimes_ExceptionWithNotes(t *testing.T) {
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)
	pickupAt := time.Date(2026, 4, 14, 14, 0, 0, 0, time.UTC)

	mock := &mockPickupScheduleService{
		bulkResult: map[int64]*scheduleService.EffectivePickupTime{
			100: {
				PickupTime:  &pickupAt,
				IsException: true,
				Notes:       "Arzttermin",
				DayNotes: []scheduleService.NoteData{
					{ID: 1, Content: "Früher abholen"},
				},
			},
		},
	}

	rs := &Resource{ResourceConfig: ResourceConfig{PickupScheduleService: mock, Logger: slog.Default()}}

	responses := []StudentResponse{
		{ID: 100, HasFullAccess: true},
	}

	rs.enrichWithPickupTimes(context.Background(), responses, []int64{100}, now)

	assert.NotNil(t, responses[0].PickupTime)
	assert.Equal(t, "14:00", *responses[0].PickupTime)
	assert.True(t, responses[0].PickupIsException, "should be marked as exception")
	assert.Equal(t, "Arzttermin, Früher abholen", responses[0].PickupNotes, "should combine notes and day notes")
}

func TestEnrichWithPickupTimes_ExceptionWithoutPickupTime(t *testing.T) {
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)

	mock := &mockPickupScheduleService{
		bulkResult: map[int64]*scheduleService.EffectivePickupTime{
			100: {
				PickupTime:  nil,
				IsException: true,
				Notes:       "Ganztägig abwesend",
			},
		},
	}

	rs := &Resource{ResourceConfig: ResourceConfig{PickupScheduleService: mock, Logger: slog.Default()}}

	responses := []StudentResponse{
		{ID: 100, HasFullAccess: true},
	}

	rs.enrichWithPickupTimes(context.Background(), responses, []int64{100}, now)

	assert.Nil(t, responses[0].PickupTime, "should be nil when exception has no pickup time")
	assert.True(t, responses[0].PickupIsException, "should be marked as exception")
	assert.Equal(t, "Ganztägig abwesend", responses[0].PickupNotes)
}

func TestBuildPickupNotes(t *testing.T) {
	tests := []struct {
		name     string
		ept      *scheduleService.EffectivePickupTime
		expected string
	}{
		{
			name:     "empty when no notes",
			ept:      &scheduleService.EffectivePickupTime{},
			expected: "",
		},
		{
			name:     "notes only",
			ept:      &scheduleService.EffectivePickupTime{Notes: "Arzttermin"},
			expected: "Arzttermin",
		},
		{
			name: "day notes only",
			ept: &scheduleService.EffectivePickupTime{
				DayNotes: []scheduleService.NoteData{
					{ID: 1, Content: "Früher abholen"},
				},
			},
			expected: "Früher abholen",
		},
		{
			name: "notes and day notes combined",
			ept: &scheduleService.EffectivePickupTime{
				Notes: "Arzttermin",
				DayNotes: []scheduleService.NoteData{
					{ID: 1, Content: "Früher abholen"},
					{ID: 2, Content: "Oma holt ab"},
				},
			},
			expected: "Arzttermin, Früher abholen, Oma holt ab",
		},
		{
			name: "skips empty day note content",
			ept: &scheduleService.EffectivePickupTime{
				Notes: "Termin",
				DayNotes: []scheduleService.NoteData{
					{ID: 1, Content: ""},
					{ID: 2, Content: "Wichtig"},
				},
			},
			expected: "Termin, Wichtig",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildPickupNotes(tt.ept)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEnrichWithPickupTimes_ServiceError(t *testing.T) {
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)

	mock := &mockPickupScheduleService{
		bulkErr: fmt.Errorf("database connection lost"),
	}

	rs := &Resource{ResourceConfig: ResourceConfig{PickupScheduleService: mock, Logger: slog.Default()}}

	responses := []StudentResponse{
		{ID: 100, HasFullAccess: true},
		{ID: 200, HasFullAccess: true},
	}

	rs.enrichWithPickupTimes(context.Background(), responses, []int64{100, 200}, now)

	// No pickup times should be set when the service returns an error
	assert.Nil(t, responses[0].PickupTime, "should be nil when service returns error")
	assert.Nil(t, responses[1].PickupTime, "should be nil when service returns error")
}

func TestEnrichWithArrivalTimes_FullAccessGating(t *testing.T) {
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)
	arrivalAt := time.Date(2026, 4, 14, 8, 15, 0, 0, time.UTC)

	mock := &mockArrivalScheduleService{
		bulkResult: map[int64]*scheduleService.EffectiveArrivalTime{
			100: {ArrivalTime: &arrivalAt},
			200: {ArrivalTime: &arrivalAt},
			300: {ArrivalTime: &arrivalAt},
		},
	}

	rs := &Resource{ResourceConfig: ResourceConfig{ArrivalScheduleService: mock, Logger: slog.Default()}}

	responses := []StudentResponse{
		{ID: 100, HasFullAccess: true},
		{ID: 200, HasFullAccess: false},
		{ID: 300, HasFullAccess: true},
	}

	rs.enrichWithArrivalTimes(context.Background(), responses, []int64{100, 300}, now)

	assert.NotNil(t, responses[0].ArrivalTime)
	assert.Equal(t, "08:15", *responses[0].ArrivalTime)
	assert.Nil(t, responses[1].ArrivalTime, "non-full-access student must not have arrival time")
	assert.NotNil(t, responses[2].ArrivalTime)
	assert.Equal(t, "08:15", *responses[2].ArrivalTime)
	assert.Equal(t, []int64{100, 300}, mock.calledWithIDs)
}

func TestEnrichWithArrivalTimes_ExceptionWithoutArrivalTime(t *testing.T) {
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)

	mock := &mockArrivalScheduleService{
		bulkResult: map[int64]*scheduleService.EffectiveArrivalTime{
			100: {
				ArrivalTime: nil,
				IsException: true,
				Notes:       "Kommt heute nicht",
			},
		},
	}

	rs := &Resource{ResourceConfig: ResourceConfig{ArrivalScheduleService: mock, Logger: slog.Default()}}

	responses := []StudentResponse{{ID: 100, HasFullAccess: true}}
	rs.enrichWithArrivalTimes(context.Background(), responses, []int64{100}, now)

	assert.Nil(t, responses[0].ArrivalTime)
	assert.True(t, responses[0].ArrivalIsException)
	assert.Equal(t, "Kommt heute nicht", responses[0].ArrivalNotes)
}

func TestBuildArrivalNotes(t *testing.T) {
	eat := &scheduleService.EffectiveArrivalTime{
		Notes: "Später",
		DayNotes: []scheduleService.ArrivalNoteData{
			{ID: 11, Content: "Bitte anrufen"},
			{ID: 12, Content: ""},
		},
	}

	assert.Equal(t, "Später, Bitte anrufen", buildArrivalNotes(eat))
}
