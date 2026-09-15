package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
)

const (
	bookingConsistencyAuditStartupDelay = 30 * time.Second
	bookingConsistencyAuditInterval     = 24 * time.Hour
	bookingConsistencyAuditTimeout      = 15 * time.Minute
)

func (s *Scheduler) scheduleBookingConsistencyAuditTask() {
	if s.bookingConsistency == nil {
		s.getLogger().Info("booking consistency audit not configured")
		return
	}

	s.registerTask("booking-consistency-audit", "24h-poll", s.runBookingConsistencyAuditTask)
}

func (s *Scheduler) runBookingConsistencyAuditTask(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in booking consistency audit task",
		"booking consistency audit using interval polling",
		bookingConsistencyAuditStartupDelay,
		func() time.Duration { return bookingConsistencyAuditInterval },
		s.checkAndRunBookingConsistencyAudit,
	)
}

func (s *Scheduler) checkAndRunBookingConsistencyAudit(ctx context.Context, task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		return
	}
	task.Running = true
	task.mu.Unlock()
	defer func() {
		task.mu.Lock()
		task.Running = false
		task.mu.Unlock()
	}()

	ctx, cancel := s.taskContext(ctx, bookingConsistencyAuditTimeout)
	defer cancel()

	auditDate := auditModel.Date(timezone.TodayDate())
	if err := s.forEachTenant(ctx, "booking-consistency-audit", func(tenantCtx context.Context) error {
		report, err := s.bookingConsistency.Audit(tenantCtx, auditDate)
		if err != nil {
			return err
		}
		if report == nil {
			return errors.New("booking consistency audit returned nil report")
		}
		s.logBookingConsistencyReport(report)
		return nil
	}); err != nil {
		s.getLogger().Error("booking consistency audit failed",
			slog.String("error", err.Error()),
		)
	}
}

func (s *Scheduler) logBookingConsistencyReport(report *auditModel.BookingConsistencyReport) {
	attrs := []slog.Attr{
		slog.Int64("tenant_id", report.TenantID),
		slog.String("audit_date", report.AuditDate.String()),
		slog.Int("pickup_projection_missing_days", report.PickupProjectionMissingDays),
		slog.Int("approved_without_required_offering", report.ApprovedWithoutRequiredOffering),
		slog.Int("approved_without_optional_offering", report.ApprovedWithoutOptionalOffering),
		slog.Int("total_findings", report.TotalFindings()),
	}
	if report.TotalFindings() > 0 {
		s.getLogger().LogAttrs(context.Background(), slog.LevelWarn,
			"booking consistency audit found drift", attrs...)
		return
	}
	s.getLogger().LogAttrs(context.Background(), slog.LevelInfo,
		"booking consistency audit passed", attrs...)
}
