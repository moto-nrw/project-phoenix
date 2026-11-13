//go:build testhooks

package api

import (
	"log/slog"
	"time"

	"github.com/spf13/viper"

	auditCompose "github.com/moto-ogs/project-phoenix/modules/auditlog/compose"
	identityPorts "github.com/moto-ogs/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-ogs/project-phoenix/modules/peopledirectory/compose"
	schoolCalendarCompose "github.com/moto-ogs/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-ogs/project-phoenix/modules/schoolmembership"
	settingsCompose "github.com/moto-ogs/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// WithInvitationResetClock pins the return day in invitation integration tests.
func WithInvitationResetClock(now func() time.Time) AuthTestOption {
	return func(settings *authTestSettings, _ *authTestMail) { settings.invitationResetClock = now }
}

func authTestInvitationReset(db transactionruntime.Database, members owners.MembershipTestRepositories, persons peopledirectory.Capability, membership schoolmembership.Capability, audits *auditCompose.Persistence, settings settingsCompose.BatchSettingsService, now func() time.Time) (*identityPorts.EmploymentResetCommands, error) {
	workforce, err := owners.NewWorkforceWithClock(db, membership, now)
	if err != nil {
		return nil, err
	}
	days, err := owners.BuildSchoolCalendarRecords(db, func(schoolCalendarCompose.Observation) {}, settings)
	if err != nil {
		return nil, err
	}
	bankData, err := BankDataCipher(viper.GetString("bank_data_keys"), "")
	if err != nil {
		return nil, err
	}
	env := owners.RootEnvironment{Logger: slog.Default(), Now: now, BankData: bankData}
	if now != nil {
		env.Today = func() calendar.Date { return calendar.DateFromTime(now()) }
	}
	people := owners.PeopleRows{Repositories: peopleCompose.Repositories{Staff: members.Staff, Persons: members.Person}}
	return owners.BuildInvitationEmploymentReset(db, env, workforce, persons, membership, people, owners.AuditOwner{Persistence: audits}, settings, days)
}
