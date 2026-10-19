//go:build testhooks

package api

import (
	"log/slog"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewUserContextTestModule composes the caller context over the test
// database with the tenant settings of unit.
func NewUserContextTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (*owners.CallerRows, error) {
	settings, err := owners.NewSettingsTestModule(db, unit)
	if err != nil {
		return nil, err
	}
	return newCallerRowsForTests(db, settings.Settings)
}

// newCallerRowsForTests composes the caller context over the same owners as
// production. settings may be nil.
func newCallerRowsForTests(db transactionruntime.Database, settings owners.CallerSettings) (*owners.CallerRows, error) {
	members := owners.MustNewMembershipTestRepositories(db)
	r, staffGroups, err := NewUserContextTestRepositories(db, members)
	if err != nil {
		return nil, err
	}
	tt, err := owners.NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return nil, err
	}
	presence, err := NewTimetablePresenceRows(db, tt, members)
	if err != nil {
		return nil, err
	}
	membership, err := owners.NewSchoolMembership(db)
	if err != nil {
		return nil, err
	}
	people, err := owners.NewPeopleDirectory(db)
	if err != nil {
		return nil, err
	}
	activities := timetableCompose.NewCallerActivityGroups(tt.ActivityGroup)
	caller, err := owners.NewCallerContext(requestTokenReads, r.Profile, owners.CallerPersons{Rows: members.Person, People: people}, membership, staffGroups,
		activities, owners.NewLoggedStudentPresence(db, slog.Default()), settings, slog.Default())
	if err != nil {
		return nil, err
	}
	return owners.NewCallerRows(caller, owners.NewSchoolStructureSuiteRepositories(db).Group, members.Staff, members.Teacher, members.Student, activities, presence.ActiveGroup, slog.Default())
}
