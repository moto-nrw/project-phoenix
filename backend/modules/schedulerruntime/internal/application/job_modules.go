package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
)

// The module switches of the Settings Platform the Worker reads (#4895). A
// switch is on by default; the moto team turns it off for a school that has
// not booked the module.
const (
	settingModuleElternApp       = "module.eltern_app"
	settingModuleMittagessen     = "module.mittagessen"
	settingModuleOnlineAnmeldung = "module.online_anmeldung"
	settingModuleZeiterfassung   = "module.zeiterfassung"
)

// jobScope says to which part of moto a worker job belongs. A job of an
// additional module names its switch; Basis jobs and the retention jobs name
// none. Retention keeps running for a school that cancelled a module: its
// data stays until its retention window ends (#4812, decision 4).
type jobScope struct {
	moduleSettingKey string
	retention        bool
}

var (
	jobScopeBasis     = jobScope{}
	jobScopeRetention = jobScope{retention: true}
)

// workerJobScopes assigns every worker job to an additional module, to Basis
// or to retention. A job of a module skips each school that has not booked
// it. TestEveryWorkerJobHasAScope fails until a new job is assigned here.
var workerJobScopes = map[ports.JobID]jobScope{
	// Retention and file cleanup: run regardless of the booked modules.
	"visit-cleanup":                 jobScopeRetention,
	"timetable-cleanup":             jobScopeRetention,
	"time-tracking-cleanup":         jobScopeRetention,
	"student-change-log-cleanup":    jobScopeRetention,
	"pwa-usage-cleanup":             jobScopeRetention,
	"staff-message-cleanup":         jobScopeRetention,
	"token-cleanup":                 jobScopeRetention,
	"staff-document-file-cleanup":   jobScopeRetention,
	"student-document-file-cleanup": jobScopeRetention,
	"file-store-cleanup":            jobScopeRetention,

	// moto Basis: day sessions, statuses, the timetable, pickup and activity
	// reminders for the team, the care plan audit and the platform jobs.
	"booking-consistency-audit": jobScopeBasis,
	"session-end":               jobScopeBasis,
	"session-cleanup":           jobScopeBasis,
	"status-flag-clear":         jobScopeBasis,
	"timetable-materialization": jobScopeBasis,
	"instance-overdue":          jobScopeBasis,
	"reminder-notifications":    jobScopeBasis,
	"timetable-auto-start":      jobScopeBasis,
	"timetable-auto-end":        jobScopeBasis,
	"activate-students":         jobScopeBasis,
	"billing-key-dates":         jobScopeBasis,
	"analytics-school-names":    jobScopeBasis,
	// The outbox delivers what a producer already enqueued; a producer of a
	// module that is not booked enqueues nothing.
	"email-outbox": jobScopeBasis,
	// The holiday guests of a hosted week belong to moto Basis like its
	// participant list (ADR 0116, ADR 0120).
	"holiday-guest-reconciliation": jobScopeBasis,
	// A planned staff exit is the staff administration of moto Basis
	// (#4809).
	"planned-staff-exits": jobScopeBasis,

	// Zeiterfassung: breaks and work sessions of the time clock.
	"break-auto-end": {moduleSettingKey: settingModuleZeiterfassung},
	"auto-checkout":  {moduleSettingKey: settingModuleZeiterfassung},

	// Eltern-App: reminders that reach parents by mail and push.
	"appointment-reminders":  {moduleSettingKey: settingModuleElternApp},
	"announcement-reminders": {moduleSettingKey: settingModuleElternApp},

	// Mittagessen: freeze orders and record late changes for the caterer.
	"meal-orders": {moduleSettingKey: settingModuleMittagessen},

	// Online-Anmeldung: deadlines of the re-enrollment phases.
	"rollover-deadline": {moduleSettingKey: settingModuleOnlineAnmeldung},
}

var errModuleSettingsRequired = errors.New("module switch cannot be read without settings")

// jobModuleBooked reports whether the school of ctx booked the module of the
// running worker job. Jobs without a module answer true. A failed read is an
// error: the scheduler skips the school as failed and tries again on the
// next tick, never treating an unreadable switch as booked.
func (s *Scheduler) jobModuleBooked(ctx context.Context, opName string) (bool, error) {
	jobID := workerJobID(ctx, ports.JobID(opName))
	scope := workerJobScopes[jobID]
	if scope.moduleSettingKey == "" {
		return true, nil
	}
	if s.settings == nil {
		return false, fmt.Errorf("%s: %w", jobID, errModuleSettingsRequired)
	}
	booked, err := s.settings.ResolveBool(ctx, scope.moduleSettingKey)
	if err != nil {
		return false, fmt.Errorf("resolve %s for %s: %w", scope.moduleSettingKey, jobID, err)
	}
	return booked, nil
}

// runIfModuleBooked runs fn for one school unless the running job belongs to
// a module the school has not booked.
func (s *Scheduler) runIfModuleBooked(ctx context.Context, opName string, tenantID int64, fn func() error) error {
	booked, err := s.jobModuleBooked(ctx, opName)
	if err != nil {
		return err
	}
	if !booked {
		s.getLogger().Debug("worker job skipped: module not booked",
			slog.String("operation", opName),
			slog.Int64("tenant_id", tenantID),
		)
		return nil
	}
	return fn()
}
