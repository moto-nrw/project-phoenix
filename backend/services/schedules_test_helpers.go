package services

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type ScheduleTestModule struct{ Schedule schedule.Service }

func NewScheduleTestModule(db *bun.DB, unit tenant.UnitOfWork) (ScheduleTestModule, error) {
	r, err := repositories.NewTimetableTestRepositories(db)
	if err != nil {
		return ScheduleTestModule{}, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return ScheduleTestModule{}, err
	}
	lock := func(ctx context.Context) error { return schedule.LockTenantRecurrenceWrites(ctx, db) }
	offerings := enrollment.NewCareOfferingService(enrollment.CareOfferingServiceConfig{
		Repo: r.CareOffering, Bookings: r.Enrollment(), ActivityGroupRepo: r.ActivityGroup,
		ActivityScheduleRepo: r.ActivitySchedule, CalendarPeriodRepo: r.CalendarPeriod, TimeframeRepo: r.Timeframe,
		ActivityExceptionRepo: r.ActivityException, Phases: r.Enrollment(), Settings: settings.Settings,
		Today: timezone.TodayDate, LockTemplateRecurrence: lock, Logger: slog.Default(),
	})
	service := schedule.NewServiceWithConfig(schedule.ServiceConfig{
		RecurrenceEvents: r.Timetable,
		DateframeRepo:    r.Dateframe, TimeframeRepo: r.Timeframe, RecurrenceRuleRepo: r.RecurrenceRule,
		LockTemplateRecurrence:              lock,
		ValidateCareOfferingTimeframeChange: offerings.(enrollment.CareOfferingMaterializationResourceValidator).ValidateTimeframeChange,
	})
	return ScheduleTestModule{Schedule: service}, nil
}
