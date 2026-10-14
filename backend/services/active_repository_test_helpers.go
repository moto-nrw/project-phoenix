package services

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/uptrace/bun"
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
func NewActiveTestRepositories(db *bun.DB, members MembershipTestRepositories, clocks ...func() time.Time) (ActiveTestRepositories, error) {
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
		SessionStartLock: NewUnloggedStudentPresence(db), GroupMapping: NewUnloggedStudentPresence(db),
		CrossTenant: NewCrossTenantQuery(NewUnloggedStudentPresence(db), people, groups)}, nil
}
