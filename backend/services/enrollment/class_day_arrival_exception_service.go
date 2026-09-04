package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The one write seam of the class-day view (#2970): a Lehrkraft sets the
// class-wide arrival day exception of #2962 for an assigned class through
// "moto schule". The rows are the ones the OGS writes in the Kindersuche and
// the shared arrival_class_exception_service is the only writer — nothing
// here touches per-child rows (ADR 0005). This service adds what the school
// portal needs on top: the school's setting, the origin stamp, the
// "Unterricht fällt aus" preset, and the live-view broadcast the OGS handler
// emits itself.

// ClassDayArrivalExceptionEntry is one class-wide day exception as the class
// view reads it: plain values, no model types.
type ClassDayArrivalExceptionEntry struct {
	SchoolClass string
	// Date is the ISO calendar day.
	Date string
	// ArrivalTime is "HH:MM".
	ArrivalTime string
	Reason      *string
	// CreatedAt is RFC 3339.
	CreatedAt string
	// Origin is "ogs" or "school".
	Origin string
}

// ClassDayArrivalExceptionWrite is what a Lehrkraft enters for one class and
// date. Origin is always "school"; CreatedBy is her staff row.
type ClassDayArrivalExceptionWrite struct {
	SchoolClass string
	Date        timezone.Date
	// ArrivalTime is a wall-clock value; only hour and minute are used.
	ArrivalTime time.Time
	Reason      *string
	CreatedBy   int64
}

// Sentinels the HTTP layer classifies. They share identity with the schedule
// service's errors, so errors.Is works on either name.
var (
	ErrClassDayArrivalExceptionPastDate      = scheduleService.ErrClassArrivalExceptionPastDate
	ErrClassDayArrivalExceptionWeekend       = scheduleService.ErrClassArrivalExceptionWeekend
	ErrClassDayArrivalExceptionClassNotFound = scheduleService.ErrClassArrivalExceptionClassNotFound
	ErrClassDayArrivalExceptionNotFound      = scheduleService.ErrClassArrivalExceptionNotFound
)

// ClassDayArrivalExceptionOriginSchool marks an entry a Lehrkraft made.
const ClassDayArrivalExceptionOriginSchool = scheduleModel.ClassArrivalExceptionOriginSchool

// ClassDaySettingsReader is the slice of the settings service the seam needs.
type ClassDaySettingsReader interface {
	ResolveString(ctx context.Context, key string) (string, error)
}

// ClassDayBlockStartReader answers the "Unterricht fällt aus" preset.
type ClassDayBlockStartReader interface {
	EarliestPlannedBlockStartForClass(ctx context.Context, schoolClass string, date timezone.Date) (string, error)
}

// ClassDayArrivalExceptionService is the class-day view's write seam for
// class-wide arrival day exceptions.
type ClassDayArrivalExceptionService interface {
	// SchoolMayWrite applies operations.school_portal_write_scope.
	SchoolMayWrite(ctx context.Context) (bool, error)
	// List returns the exceptions of one class with from <= date <= to.
	List(ctx context.Context, schoolClass string, from, to timezone.Date) ([]ClassDayArrivalExceptionEntry, error)
	// Set stores the exception of one class and date as entered by the
	// school and tells the OGS live views to refetch once it committed.
	Set(ctx context.Context, in ClassDayArrivalExceptionWrite) (*ClassDayArrivalExceptionEntry, error)
	// Remove deletes the exception of one class and date.
	Remove(ctx context.Context, schoolClass string, date timezone.Date) error
	// EarliestBlockStart returns the "HH:MM" start of the first block of the
	// date that addresses the class, "" when there is none.
	EarliestBlockStart(ctx context.Context, schoolClass string, date timezone.Date) (string, error)
}

// ClassDayArrivalExceptionConfig wires the seam.
type ClassDayArrivalExceptionConfig struct {
	ArrivalSchedule scheduleService.ArrivalScheduleService
	Settings        ClassDaySettingsReader
	BlockStarts     ClassDayBlockStartReader
	// Broadcaster is optional: without it nothing is announced.
	Broadcaster realtime.Broadcaster
	Logger      *slog.Logger
}

type classDayArrivalExceptionService struct {
	cfg ClassDayArrivalExceptionConfig
}

// NewClassDayArrivalExceptionService builds the seam.
func NewClassDayArrivalExceptionService(cfg ClassDayArrivalExceptionConfig) ClassDayArrivalExceptionService {
	return &classDayArrivalExceptionService{cfg: cfg}
}

func (s *classDayArrivalExceptionService) logger() *slog.Logger {
	if s.cfg.Logger != nil {
		return s.cfg.Logger
	}
	return slog.Default()
}

func (s *classDayArrivalExceptionService) SchoolMayWrite(ctx context.Context) (bool, error) {
	if s.cfg.Settings == nil {
		return false, errors.New("class day arrival exceptions: settings not configured")
	}
	scope, err := s.cfg.Settings.ResolveString(ctx, configModel.KeySchoolPortalWriteScope)
	if err != nil {
		return false, fmt.Errorf("class day arrival exceptions: resolve school portal write scope: %w", err)
	}
	return scope == configModel.SchoolPortalWriteScopeClassArrivalExceptions, nil
}

func (s *classDayArrivalExceptionService) List(ctx context.Context, schoolClass string, from, to timezone.Date) ([]ClassDayArrivalExceptionEntry, error) {
	if s.cfg.ArrivalSchedule == nil {
		return nil, errors.New("class day arrival exceptions: arrival schedule service not configured")
	}
	rows, err := s.cfg.ArrivalSchedule.ListClassArrivalExceptions(ctx, schoolClass, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]ClassDayArrivalExceptionEntry, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		out = append(out, classDayArrivalExceptionEntry(row))
	}
	return out, nil
}

func (s *classDayArrivalExceptionService) Set(ctx context.Context, in ClassDayArrivalExceptionWrite) (*ClassDayArrivalExceptionEntry, error) {
	if s.cfg.ArrivalSchedule == nil {
		return nil, errors.New("class day arrival exceptions: arrival schedule service not configured")
	}
	row, err := s.cfg.ArrivalSchedule.UpsertClassArrivalException(ctx, scheduleService.ClassArrivalExceptionInput{
		SchoolClass: in.SchoolClass,
		Date:        in.Date,
		ArrivalTime: in.ArrivalTime,
		Reason:      in.Reason,
		Origin:      scheduleModel.ClassArrivalExceptionOriginSchool,
	}, in.CreatedBy)
	if err != nil {
		return nil, err
	}
	s.announceAfterCommit(ctx)
	entry := classDayArrivalExceptionEntry(row)
	return &entry, nil
}

func (s *classDayArrivalExceptionService) Remove(ctx context.Context, schoolClass string, date timezone.Date) error {
	if s.cfg.ArrivalSchedule == nil {
		return errors.New("class day arrival exceptions: arrival schedule service not configured")
	}
	if err := s.cfg.ArrivalSchedule.DeleteClassArrivalException(ctx, schoolClass, date); err != nil {
		return err
	}
	s.announceAfterCommit(ctx)
	return nil
}

func (s *classDayArrivalExceptionService) EarliestBlockStart(ctx context.Context, schoolClass string, date timezone.Date) (string, error) {
	if s.cfg.BlockStarts == nil {
		return "", errors.New("class day arrival exceptions: block starts not configured")
	}
	return s.cfg.BlockStarts.EarliestPlannedBlockStartForClass(ctx, schoolClass, date)
}

// announceAfterCommit emits the arrival-schedule event the OGS handler emits
// after its own writes, so Aufsicht and Meine Gruppe refetch. A class-wide
// change concerns every child of the class, so no student ID travels with
// it.
func (s *classDayArrivalExceptionService) announceAfterCommit(ctx context.Context) {
	if s.cfg.Broadcaster == nil {
		return
	}
	tenant.RegisterAfterCommit(ctx, func() {
		source := "school"
		event := realtime.NewEvent(realtime.EventArrivalScheduleChanged, "", realtime.EventData{Source: &source})
		if err := s.cfg.Broadcaster.BroadcastToAll(event); err != nil {
			s.logger().Warn("failed to broadcast arrival schedule change",
				"error", err.Error(),
			)
		}
	})
}

func classDayArrivalExceptionEntry(row *scheduleModel.ClassArrivalException) ClassDayArrivalExceptionEntry {
	origin := strings.TrimSpace(row.Origin)
	if origin == "" {
		origin = scheduleModel.ClassArrivalExceptionOriginOGS
	}
	return ClassDayArrivalExceptionEntry{
		SchoolClass: row.SchoolClass,
		Date:        row.Date.String(),
		ArrivalTime: row.ArrivalTime.Format("15:04"),
		Reason:      row.Reason,
		CreatedAt:   row.CreatedAt.Format(time.RFC3339),
		Origin:      origin,
	}
}
