//go:build testhooks

package api

import (
	"log/slog"

	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type UserContextTestModule struct {
	UserContext *CallerRows
}

func NewUserContextTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (UserContextTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return UserContextTestModule{}, err
	}
	rows, err := newCallerRowsForTests(db, settings.Settings)
	if err != nil {
		return UserContextTestModule{}, err
	}
	return UserContextTestModule{UserContext: rows}, nil
}

// newCallerRowsForTests composes the caller context over the same owners as
// production. settings may be nil.
func newCallerRowsForTests(db transactionruntime.Database, settings CallerSettings) (*CallerRows, error) {
	members := MustNewMembershipTestRepositories(db)
	r, err := NewUserContextTestRepositories(db, members)
	if err != nil {
		return nil, err
	}
	presence, err := NewTimetablePresenceRows(db, r.Timetable, members)
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
	tt := r.Timetable
	return NewCallerRowsFromWiring(CallerContextWiring{
		Accounts: r.Profile, Persons: callerPersons{rows: members.Person, people: people}, Membership: membership, StaffGroups: r.StaffGroups,
		SupervisedActivities: timetableCompose.NewCallerActivityGroups(tt.ActivityGroup).SupervisedActivityGroupIDs,
		Presence:             NewLoggedStudentPresence(db, slog.Default()),
		Settings:             settings,
		Tokens:               requestTokenReads,
	}, CallerRowSources{
		Groups: members.Group, Staff: members.Staff, Teachers: members.Teacher, Students: members.Student,
		Activities: timetableCompose.NewCallerActivityGroups(tt.ActivityGroup), Sessions: presence.ActiveGroup,
	})
}
