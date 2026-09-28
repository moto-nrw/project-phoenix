package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// newExportAccess validates the acting account, defaults the role (the
// actor_role column is NOT NULL and never carries an empty string), and
// builds the access-log row shared by the export audits. Metadata stays with
// each export: it is the observable audit content and differs per export.
func newExportAccess(errPrefix string, actorAccountID int64, actorRole string, rangeStart, rangeEnd, accessedAt time.Time) (ExportAccess, error) {
	if actorAccountID <= 0 {
		return ExportAccess{}, fmt.Errorf("%s: actor account id required", errPrefix)
	}
	if strings.TrimSpace(actorRole) == "" {
		actorRole = "unknown"
	}
	return ExportAccess{
		ActorAccountID: actorAccountID,
		ActorRole:      actorRole,
		RangeStart:     rangeStart,
		RangeEnd:       rangeEnd,
		AccessedAt:     accessedAt,
		Metadata:       map[string]any{},
	}, nil
}

// recordPhaseExport writes a phase export row over the phase's service
// window, wrapping failures with the export's historical error prefix.
func (s *Reports) recordPhaseExport(ctx context.Context, errPrefix string, phaseID int64, actorAccountID int64, actorRole string, metadata map[string]any) error {
	if s.deps.AccessLog == nil {
		return fmt.Errorf("%s: data access log repo not configured", errPrefix)
	}
	phase, err := s.deps.Phases.Phase(ctx, phaseID)
	if err != nil {
		return fmt.Errorf("%s: phase %d: %w", errPrefix, phaseID, err)
	}
	entry, err := newExportAccess(errPrefix, actorAccountID, actorRole,
		calendar.Date(phase.ServiceStartDate).BerlinMidnight(), calendar.Date(phase.ServiceEndDate).EndOfDay(), time.Now())
	if err != nil {
		return err
	}
	entry.Metadata = metadata
	if err := s.deps.AccessLog.RecordPhaseExport(ctx, entry); err != nil {
		return fmt.Errorf("%s write: %w", errPrefix, err)
	}
	return nil
}

func (s *Reports) recordCareUsageExportAudit(ctx context.Context, report *enrollment.CareUsageReport, actorAccountID int64, actorRole, format string, compact bool) error {
	const errPrefix = "care usage report export audit"
	if s.deps.AccessLog == nil {
		return fmt.Errorf("%s: data access log repo not configured", errPrefix)
	}
	if report == nil {
		return fmt.Errorf("%s: report required", errPrefix)
	}
	layout := "detailed"
	if compact {
		layout = "compact"
	}
	var dayCount any
	if report.Filters.DayCount != nil {
		dayCount = *report.Filters.DayCount
	}
	return s.recordPhaseExport(ctx, errPrefix, report.Phase.ID, actorAccountID, actorRole, map[string]any{
		"phase_id":          report.Phase.ID,
		"report":            "care_usage",
		"format":            format,
		"layout":            layout,
		"status_filter":     report.Filters.Status,
		"care_offering_ids": report.Filters.CareOfferingIDs,
		"day_count":         dayCount,
		"grade_level":       report.Filters.GradeLevel,
		"weekday":           report.Filters.Weekday,
		"pickup_time":       report.Filters.PickupTime,
		"search":            report.Filters.Search,
		"child_count":       report.Totals.Children,
	})
}

func (s *Reports) recordClassRosterExportAudit(ctx context.Context, report *enrollment.ClassRosterReport, actorAccountID int64, actorRole, format string) error {
	const errPrefix = "class roster report export audit"
	if s.deps.AccessLog == nil {
		return fmt.Errorf("%s: data access log repo not configured", errPrefix)
	}
	if report == nil {
		return fmt.Errorf("%s: report required", errPrefix)
	}
	schoolClass := report.Filters.SchoolClass
	if report.Filters.AllClasses {
		schoolClass = "alle"
	}
	return s.recordPhaseExport(ctx, errPrefix, report.Phase.ID, actorAccountID, actorRole, map[string]any{
		"phase_id":         report.Phase.ID,
		"report":           "class_roster",
		"format":           format,
		"school_class":     schoolClass,
		"status_filter":    report.Filters.Status,
		"student_count":    report.Totals.Students,
		"registered_count": report.Totals.Registered,
	})
}
