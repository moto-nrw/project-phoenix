package services

import (
	"log/slog"

	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type UserContextTestModule struct {
	UserContext *CallerRows
}

func NewUserContextTestModule(db *bun.DB, unit tenant.UnitOfWork) (UserContextTestModule, error) {
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
func newCallerRowsForTests(db *bun.DB, settings callerSettings) (*CallerRows, error) {
	r, err := NewUserContextTestRepositories(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return nil, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return nil, err
	}
	tt := r.Timetable
	return newCallerRows(callerContextWiring{
		Accounts: r.Profile, Persons: tt.Person, Membership: membership, StaffGroups: r.StaffGroups,
		SupervisedActivities: timetableCompose.NewCallerActivityGroups(tt.ActivityGroup).SupervisedActivityGroupIDs,
		Presence:             newStudentPresence(db, slog.Default()),
		Settings:             settings,
	}, CallerRowSources{
		Groups: tt.Group, Staff: tt.Staff, Teachers: tt.Teacher, Students: tt.Student,
		Activities: timetableCompose.NewCallerActivityGroups(tt.ActivityGroup), Sessions: tt.ActiveGroup,
	})
}
