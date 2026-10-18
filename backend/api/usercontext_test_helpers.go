//go:build testhooks

package api

import (
	"log/slog"

	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewUserContextTestModule composes the caller context over the test
// database with the tenant settings of unit.
func NewUserContextTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (*CallerRows, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return nil, err
	}
	return newCallerRowsForTests(db, settings.Settings)
}

// newCallerRowsForTests composes the caller context over the same owners as
// production. settings may be nil.
func newCallerRowsForTests(db transactionruntime.Database, settings CallerSettings) (*CallerRows, error) {
	members := MustNewMembershipTestRepositories(db)
	r, staffGroups, err := NewUserContextTestRepositories(db, members)
	if err != nil {
		return nil, err
	}
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return nil, err
	}
	presence, err := NewTimetablePresenceRows(db, tt, members)
	if err != nil {
		return nil, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return nil, err
	}
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return nil, err
	}
	activities := timetableCompose.NewCallerActivityGroups(tt.ActivityGroup)
	caller, err := newCallerContext(requestTokenReads, r.Profile, callerPersons{rows: members.Person, people: people}, membership, staffGroups,
		activities, NewLoggedStudentPresence(db, slog.Default()), settings, slog.Default())
	if err != nil {
		return nil, err
	}
	return NewCallerRows(caller, NewSchoolStructureSuiteRepositories(db).Group, members.Staff, members.Teacher, members.Student, activities, presence.ActiveGroup, slog.Default())
}
