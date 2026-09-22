package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type UserContextTestModule struct {
	UserContext *repositories.CallerRows
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
func newCallerRowsForTests(db *bun.DB, settings callerSettings) (*repositories.CallerRows, error) {
	r, err := repositories.NewUserContextTestRepositories(db)
	if err != nil {
		return nil, err
	}
	membership, err := repositories.NewSchoolMembership(db)
	if err != nil {
		return nil, err
	}
	tt := r.Timetable
	return newCallerRows(callerContextWiring{
		Accounts: r.Profile, Persons: tt.Person, Membership: membership, StaffGroups: r.StaffGroups,
		SupervisedActivities: supervisedActivityGroupIDs(tt.ActivityGroup),
		Presence:             newStudentPresence(db, slog.Default()),
		Settings:             settings,
	}, repositories.CallerRowSources{
		Groups: tt.Group, Staff: tt.Staff, Teachers: tt.Teacher, Students: tt.Student,
		Activities: tt.ActivityGroup, Sessions: tt.ActiveGroup,
	})
}
