package api

import (
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	shiftplansyncCompose "github.com/moto-nrw/project-phoenix/workflows/shiftplansync/compose"
)

// buildWorkSessionSide binds the time-tracking side of Workforce (#3413)
// without the absences: stamps, history and edits, month balances and their
// close, overview, exports, the audit feed and Student Presence's NFC
// auto-check-in. The absences need the #1843 sick cascade, which needs the
// Timetable lifecycle, which ends sessions through Student Presence, so
// Workforce completes them once the cascade exists.
func buildWorkSessionSide(
	db transactionruntime.Database, env RootEnvironment, transport MailTransport, mailIdentity email.ReplyToResolver,
	workTime workforceModule.Capability, membership schoolmembership.Capability, persons peopledirectory.Capability,
	organizations organizationtenancy.Query, people PeopleRows, users peoplePorts.PersonDirectory, audit AuditOwner,
	supervisions studentpresence.SupervisionRecords, sessions *presenceCompose.SessionRecords, calendar schoolcalendar.Days,
	settings settingsCompose.BatchSettingsService, payroll settingsCompose.PayrollStatusGetter, durable DurableMailOutbox,
) (*workforceCompose.WorkSessionSide, error) {
	logger := env.Logger.With("service", "active")
	employment := workforceCompose.MustNewStaffEmployment(db.Pool())
	audits := audit.Persistence
	ports := timeTrackingPorts(workTime, workforceCompose.TimeTrackingPorts{
		Absences: NewStaffAbsenceRepository(workTime, membership, persons), AbsenceAudits: workforceCompose.NewStaffAbsenceAuditRepository(workTime),
		Staff:        WorkSessionStaff(people.Staff, employment, membership),
		Supervisions: supervisions, SessionRecords: sessions, Holidays: schoolCalendarCompose.NonWorkingDays{Calendar: calendar},
		Events:       timeTrackingEvents(transport.Hub),
		AbsenceEmail: absenceEmailDependencies(settings, transport, mailIdentity, env.Links.FrontendURL, logger, people, organizations, durable),
		Logger:       logger,
	}, timeTrackingAudits{
		WorkSessionEdit: audits.WorkSessionEdit, TimeTrackingDeletion: audits.TimeTrackingDeletion,
		DataAccessLog: audits.DataAccessLog, TimeTrackingAuditLog: audits.TimeTrackingAuditLog,
	}, people.Staff, employment, NewStaffWorkScheduleRepository(workTime), NewWorkTimeModelRepository(workTime), settings, payroll, db)
	side, err := workforceCompose.NewWorkSessionSide(workTime, ports, staffScheduleBindings(users))
	if err != nil {
		return nil, fmt.Errorf("compose time tracking: %w", err)
	}
	return side, nil
}

// absenceEmailDependencies send the absence request mails through the tenant
// mail, to the People Directory's staff contacts, linking the school's
// subdomain.
func absenceEmailDependencies(
	settings settingsCompose.SettingsService, transport MailTransport, mailIdentity email.ReplyToResolver, frontendURL string,
	logger *slog.Logger, people PeopleRows, organizations organizationtenancy.Query, durable DurableMailOutbox,
) *workforceCompose.AbsenceEmailDeps {
	return &workforceCompose.AbsenceEmailDeps{
		Settings: settingsCompose.PresenceSettings(settings),
		Dispatcher: absenceEmailDispatcher{
			durable: durable, from: transport.DefaultFrom, identity: mailIdentity, logger: logger,
		},
		StaffRepo:   absenceEmailStaffDirectory{source: people.Staff},
		SchoolRepo:  absenceEmailSchoolDirectory{schools: organizations},
		FrontendURL: frontendURL,
		Logger:      logger,
	}
}

// WorkforcePlanning is the Dienstplan side of Workforce (#3418) with the
// time tracking it completes: the shift planning, the staff-shift
// capability over the printable weekly plans, the time tracking with the
// #1843 sick cascade and the time-tracking retention sweep (Tranche 0b).
type WorkforcePlanning struct {
	Shifts              *workforceCompose.ShiftPlanning
	StaffShifts         workforceModule.StaffShiftPlanning
	TimeTracking        *workforceCompose.TimeTracking
	TimeTrackingCleanup workforceCompose.TimeTrackingCleanupService
}

// buildWorkforcePlanning completes the time tracking with the sick cascade,
// which runs over the staff shifts, the deviation writes and the timetable
// rows.
func buildWorkforcePlanning(
	db transactionruntime.Database, env RootEnvironment, transport MailTransport, workTime workforceModule.Capability,
	membership schoolmembership.Capability, persons peopledirectory.Capability, shifts *workforceCompose.ShiftPlanning,
	planExport planexport.Service, side *workforceCompose.WorkSessionSide, lifecycle TimetableLifecycle, owner TimetableOwner,
	audit AuditOwner, settings settingsCompose.BatchSettingsService,
) (WorkforcePlanning, error) {
	staffShifts := shifts.Planning(planExport)
	cascade, err := shiftplansyncCompose.NewSickCascade(shiftplansyncCompose.SickCascadeDependencies{
		Planning: staffShifts, Workforce: workTime, LockStaffShifts: workforceCompose.NewStaffShiftLock(db.Pool()),
		Deviations:    lifecycle.Deviations,
		TimetableData: lifecycle.Rows.SickCascadeRows(arrivalTimetable.SubstituteDayLock(db.Pool())),
		InstanceStaff: arrivalTimetable.NewTimetableInstanceStaffReads(owner.Repositories.InstanceStaff),
		Broadcaster:   transport.Hub, Logger: env.Logger.With("service", "shift_plan_sync"), Today: env.Today,
	})
	if err != nil {
		return WorkforcePlanning{}, err
	}
	timeTracking, err := side.WithSickCascade(cascade)
	if err != nil {
		return WorkforcePlanning{}, fmt.Errorf("compose time tracking: %w", err)
	}
	cleanup, err := workforceCompose.NewTimeTrackingCleanup(workforceCompose.TimeTrackingCleanupDependencies{
		Workforce: workTime, Absences: NewStaffAbsenceRepository(workTime, membership, persons),
		Audit:    NewTimeTrackingRetentionAudit(audit.Persistence.DataDeletion),
		Settings: settingsCompose.PresenceSettings(settings), Logger: env.Logger.With("service", "time-tracking-cleanup"),
	})
	if err != nil {
		return WorkforcePlanning{}, fmt.Errorf("compose time tracking cleanup: %w", err)
	}
	return WorkforcePlanning{Shifts: shifts, StaffShifts: staffShifts, TimeTracking: timeTracking, TimeTrackingCleanup: cleanup}, nil
}
