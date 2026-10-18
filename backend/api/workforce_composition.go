package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// workforceLiveStaffIDs answers Workforce's one question to School
// Membership: which of the memberships bound to a work-time template are still
// live. Workforce never reads users.staff_school_memberships itself.
func workforceLiveStaffIDs(membership schoolmembership.Query) func(context.Context, []int64) ([]int64, error) {
	return func(ctx context.Context, ids []int64) ([]int64, error) {
		members, err := membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: ids, MembershipOnly: true})
		if err != nil {
			return nil, err
		}
		live := make([]int64, 0, len(members))
		for _, member := range members {
			live = append(live, member.ID)
		}
		return live, nil
	}
}

// workforceStaffAssignmentLock answers Workforce's lock on a staff
// assignment: the School Membership row of the staff member is locked for
// the mutation the caller's unit of work is about to make.
func workforceStaffAssignmentLock(membership schoolmembership.Query) func(context.Context, int64) error {
	return func(ctx context.Context, staffID int64) error {
		_, err := membership.FindStaffForMutation(ctx, staffID)
		return err
	}
}

// NewUnobservedWorkforce composes the Workforce owner over an unobserved
// School Membership for the command roots, which record no runtime evidence.
func NewUnobservedWorkforce(db transactionruntime.Database) (workforceModule.Capability, error) {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return nil, err
	}
	return workforceCompose.New(workforceCompose.Dependencies{
		LockStaffAssignment: workforceStaffAssignmentLock(membership),
		DB:                  db.Pool(),
		LiveStaffIDs:        workforceLiveStaffIDs(membership),
		Observe:             func(workforceCompose.Observation) {},
	})
}

// newWorkforceWiring binds Workforce into the service graph (#4008, ADR
// 0053): the adapters over the owners the root composes come built, and the
// bindings over the staff rows, audit trails, settings and mail only the
// service graph builds come as functions the graph calls with them.
func newWorkforceWiring(db transactionruntime.Database, workTime workforceModule.Capability, membership schoolmembership.Capability, persons peopledirectory.Capability, organizations organizationtenancy.Query, staff staffWithPersonReader) WorkforceWiring {
	staffAbsences := NewStaffAbsenceRepository(workTime, membership, persons)
	return WorkforceWiring{
		ShiftRows:       newWorkforceShiftRows(workTime),
		ShiftTypeRows:   newWorkforceShiftTypeRows(workTime),
		PlanExportStaff: planExportStaffNames{staff: staff},
		StaffAdmin: func(sources WorkforceStaffAdminSources) (*workforceModule.StaffAdmin, error) {
			return composeStaffAdmin(workforceCompose.StaffAdminDependencies{
				DB: db.Pool(), Logger: sources.Logger, Now: sources.Now,
			}, staffAdminOwners{
				Staff: sources.Staff, Persons: staffAdminPersons{rows: sources.Persons, details: persons}, Membership: membership,
				MasterDataAudit: sources.Audits.StaffMasterDataChange, PersonnelNumber: sources.Audits.PersonnelNumberChange, DataAccessLog: sources.Audits.DataAccessLog,
			})
		},
		TimeTracking: func(sources WorkforceTimeTrackingSources) (*workforceCompose.TimeTracking, error) {
			employment := workforceCompose.MustNewStaffEmployment(db.Pool())
			return newTimeTracking(timeTrackingPortDependencies{
				Workforce: workTime,
				Bound: workforceCompose.TimeTrackingPorts{
					Absences: staffAbsences, AbsenceAudits: workforceCompose.NewStaffAbsenceAuditRepository(workTime),
					Staff:        WorkSessionStaff(sources.Staff, employment, membership),
					Supervisions: sources.Supervisions, SessionRecords: sources.SessionRecords, Holidays: sources.Holidays,
					Events: timeTrackingEvents(sources.Broadcaster), AbsenceEmail: timeTrackingAbsenceEmail(sources, organizations),
					ShiftPlanSync: sources.ShiftPlanSync,
				},
				Audits: timeTrackingAudits{
					WorkSessionEdit: sources.Audits.WorkSessionEdit, TimeTrackingDeletion: sources.Audits.TimeTrackingDeletion,
					DataAccessLog: sources.Audits.DataAccessLog, TimeTrackingAuditLog: sources.Audits.TimeTrackingAuditLog,
				},
				Staff: sources.Staff, People: sources.People, StaffEmployment: employment,
				WorkSchedules:  NewStaffWorkScheduleRepository(workTime),
				WorkTimeModels: NewWorkTimeModelRepository(workTime),
				Settings:       sources.Settings, Payroll: sources.Payroll,
				DB: db, Logger: sources.Logger,
			})
		},
	}
}

// timeTrackingAbsenceEmail sends the absence request mails through the
// graph's tenant mail, to the People Directory's staff contacts, linking the
// school's subdomain.
func timeTrackingAbsenceEmail(sources WorkforceTimeTrackingSources, organizations organizationtenancy.Query) *workforceCompose.AbsenceEmailDeps {
	return &workforceCompose.AbsenceEmailDeps{
		Settings: settingsCompose.PresenceSettings(sources.Settings),
		Dispatcher: absenceEmailDispatcher{
			dispatcher: sources.Mail.Dispatcher, from: sources.Mail.From, identity: sources.Mail.ReplyTo, logger: sources.Logger,
		},
		StaffRepo:   absenceEmailStaffDirectory{source: sources.Staff},
		SchoolRepo:  absenceEmailSchoolDirectory{schools: organizations},
		FrontendURL: sources.Mail.FrontendURL,
		Logger:      sources.Logger,
	}
}
