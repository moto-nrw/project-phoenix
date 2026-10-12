package repositories

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
	tt, err := NewTimetableTestRepositories(db, members, clocks...)
	if err != nil {
		return ActiveTestRepositories{}, err
	}
	people := members.People
	groups, err := NewSchoolStructure(db)
	if err != nil {
		return ActiveTestRepositories{}, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return ActiveTestRepositories{}, err
	}
	workTime, err := NewWorkforce(db, membership)
	if err != nil {
		return ActiveTestRepositories{}, err
	}
	r := &Factory{db: db, CrossTenant: &visitorProjection{visits: newStudentPresence(db)}}
	r.bindStaffProjections(membership, workTime)
	r.bindStudentDirectories(people, people)
	r.bindGuardianDirectories(people)
	r.bindPersonProjections(people)
	r.CrossTenant = groupCrossTenantRepository{CrossTenantQuery: r.CrossTenant, groups: groups}
	return ActiveTestRepositories{TimetableTestRepositories: tt,
		SessionStartLock: newStudentPresence(db), GroupMapping: newStudentPresence(db), CrossTenant: r.CrossTenant}, nil
}
