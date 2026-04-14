package students

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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

func (m *mockPickupScheduleService) GetBulkEffectivePickupTimesForDate(_ context.Context, studentIDs []int64, _ time.Time) (map[int64]*scheduleService.EffectivePickupTime, error) {
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

	rs := &Resource{
		PickupScheduleService: mock,
		Logger:                slog.Default(),
	}

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
	rs := &Resource{
		PickupScheduleService: nil,
		Logger:                slog.Default(),
	}

	responses := []StudentResponse{
		{ID: 100, HasFullAccess: true},
	}

	// Should not panic
	rs.enrichWithPickupTimes(context.Background(), responses, []int64{100}, time.Now())
	assert.Nil(t, responses[0].PickupTime, "should be nil when service is nil")
}

func TestEnrichWithPickupTimes_EmptyIDs(t *testing.T) {
	mock := &mockPickupScheduleService{}
	rs := &Resource{
		PickupScheduleService: mock,
		Logger:                slog.Default(),
	}

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

	rs := &Resource{
		PickupScheduleService: mock,
		Logger:                slog.Default(),
	}

	responses := []StudentResponse{
		{ID: 100, HasFullAccess: true},
	}

	rs.enrichWithPickupTimes(context.Background(), responses, []int64{100}, now)

	assert.Nil(t, responses[0].PickupTime, "should be nil when no pickup time is set")
}

func TestEnrichWithPickupTimes_ServiceError(t *testing.T) {
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)

	mock := &mockPickupScheduleService{
		bulkErr: fmt.Errorf("database connection lost"),
	}

	rs := &Resource{
		PickupScheduleService: mock,
		Logger:                slog.Default(),
	}

	responses := []StudentResponse{
		{ID: 100, HasFullAccess: true},
		{ID: 200, HasFullAccess: true},
	}

	rs.enrichWithPickupTimes(context.Background(), responses, []int64{100, 200}, now)

	// No pickup times should be set when the service returns an error
	assert.Nil(t, responses[0].PickupTime, "should be nil when service returns error")
	assert.Nil(t, responses[1].PickupTime, "should be nil when service returns error")
}
