package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubBookingConsistencyAudit struct {
	report *BookingConsistencyReport
	err    error
	calls  int
}

// audit serves the scheduler's booking consistency port.
func (s *stubBookingConsistencyAudit) audit(_ context.Context, auditDate calendar.Date) (*BookingConsistencyReport, error) {
	s.calls++
	if s.report != nil {
		s.report.AuditDate = auditDate
	}
	return s.report, s.err
}

func TestBookingConsistencyAuditLogsDriftCounts(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	auditor := &stubBookingConsistencyAudit{report: &BookingConsistencyReport{
		TenantID:                    42,
		PickupProjectionMissingDays: 3,
		TotalFindings:               3,
	}}
	s := unitScheduler(&Scheduler{
		bookingConsistency: auditor.audit,
		tasks:              make(map[string]*ScheduledTask),
		logger: slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))})

	s.checkAndRunBookingConsistencyAudit(context.Background(), &ScheduledTask{Name: "booking-consistency-audit"})

	require.Equal(t, 1, auditor.calls)
	logOutput := output.String()
	assert.Contains(t, logOutput, `"msg":"booking consistency audit found drift"`)
	assert.Contains(t, logOutput, `"tenant_id":42`)
	assert.Contains(t, logOutput, `"pickup_projection_missing_days":3`)
	assert.NotContains(t, logOutput, `"arrival_without_booking_days"`)
	assert.NotContains(t, logOutput, `"booking_without_arrival_days"`)
	assert.NotContains(t, logOutput, `"planned_without_booking_rows"`)
	assert.Contains(t, logOutput, `"total_findings":3`)
}

func TestBookingConsistencyAuditLogsRepositoryError(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	want := errors.New("query failed")
	auditor := &stubBookingConsistencyAudit{err: want}
	s := unitScheduler(&Scheduler{
		bookingConsistency: auditor.audit,
		tasks:              make(map[string]*ScheduledTask),
		logger: slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))})

	s.checkAndRunBookingConsistencyAudit(context.Background(), &ScheduledTask{Name: "booking-consistency-audit"})

	require.Equal(t, 1, auditor.calls)
	assert.Contains(t, output.String(), `"msg":"tenant operation failed, continuing to next tenant"`)
	assert.Contains(t, output.String(), `"error":"query failed"`)
}

func TestBookingConsistencyAuditTreatsOptionalNoOfferingAsReview(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	// The Audit Platform counts an optional-offering phase for review only,
	// so the bound report carries no finding for it.
	auditor := &stubBookingConsistencyAudit{report: &BookingConsistencyReport{
		TenantID:                        42,
		ApprovedWithoutOptionalOffering: 2,
	}}
	s := unitScheduler(&Scheduler{
		bookingConsistency: auditor.audit,
		tasks:              make(map[string]*ScheduledTask),
		logger: slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))})

	s.checkAndRunBookingConsistencyAudit(context.Background(), &ScheduledTask{Name: "booking-consistency-audit"})

	assert.Contains(t, output.String(), `"msg":"booking consistency audit passed"`)
	assert.Contains(t, output.String(), `"approved_without_optional_offering":2`)
	assert.Contains(t, output.String(), `"total_findings":0`)
}
