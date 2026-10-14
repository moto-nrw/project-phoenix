package api

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

type ActiveTestRepositories struct {
	TimetableTestRepositories
	SessionStartLock interface {
		LockSessionStart(context.Context, int64) error
	}
	GroupMapping *studentpresence.Module
	CrossTenant  CrossTenantQuery
}

// NewActiveTestRepositories composes the active suites' repository graph over
// the People Directory and School Membership repositories in members.
func NewActiveTestRepositories(db transactionruntime.Database, members MembershipTestRepositories, clocks ...func() time.Time) (ActiveTestRepositories, error) {
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members, clocks...)
	if err != nil {
		return ActiveTestRepositories{}, err
	}
	people := members.People
	groups, err := NewSchoolStructure(db)
	if err != nil {
		return ActiveTestRepositories{}, err
	}
	return ActiveTestRepositories{TimetableTestRepositories: tt,
		SessionStartLock: services.NewUnloggedStudentPresence(db.Pool()), GroupMapping: services.NewUnloggedStudentPresence(db.Pool()),
		CrossTenant: NewCrossTenantQuery(services.NewUnloggedStudentPresence(db.Pool()), people, groups)}, nil
}
