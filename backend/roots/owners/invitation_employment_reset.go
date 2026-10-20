package owners

import (
	"context"

	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

func BuildInvitationEmploymentReset(db transactionruntime.Database, env RootEnvironment, workTime workforce.Capability, persons peopledirectory.Capability, membership schoolmembership.Capability, people PeopleRows, audit AuditOwner, settings settingsCompose.BatchSettingsService, calendar schoolcalendar.Days) (*identityPorts.EmploymentResetCommands, error) {
	admin, err := BuildWorkforceStaffAdmin(db, env, persons, membership, people, audit)
	if err != nil {
		return nil, err
	}
	employment := workforceCompose.MustNewStaffEmployment(db.Pool())
	schedules := NewStaffWorkScheduleRepository(workTime)
	models := NewWorkTimeModelRepository(workTime)
	reset := workforceCompose.NewEmploymentReset(admin, workTime, employment, staffAdminAudit{masterData: audit.Persistence.StaffMasterDataChange, personnelNumber: audit.Persistence.PersonnelNumberChange, accessLog: audit.Persistence.DataAccessLog}, workforceCompose.TimeTrackingPorts{
		Absences:        NewStaffAbsenceRepository(workTime, membership, persons),
		StaffSchedules:  workforceCompose.StaffScheduleAssignments(employment),
		ScheduleTargets: NewWorkScheduleTargets(schedules), TargetModels: NewWorkTimeTargetModels(models),
		Shifts: workforceCompose.NewTimeTrackingShifts(workTime), Holidays: schoolCalendarCompose.NewNonWorkingDays(calendar),
		Settings: settingsCompose.PresenceSettings(settings), Logger: env.Logger,
		DeletionAudit: NewTimeTrackingDeletionAudit(audit.Persistence.TimeTrackingDeletion), Today: env.Today,
	})
	return &identityPorts.EmploymentResetCommands{
		Prepare: func(ctx context.Context, in identityPorts.EmploymentReset) (*string, error) {
			return reset.PrepareEmploymentReset(ctx, workforce.EmploymentReset{StaffID: in.StaffID, WorkTime: in.WorkTime})
		},
		Execute: func(ctx context.Context, in identityPorts.EmploymentReset) error {
			return reset.ResetEmployment(ctx, workforce.EmploymentReset{StaffID: in.StaffID, ActorAccountID: in.AccountID, InvitationID: in.InvitationID, PreviousPersonnelNumber: in.PreviousPersonnelNumber, MasterData: in.MasterData, Qualifications: in.Qualifications, WorkTime: in.WorkTime})
		},
	}, nil
}
