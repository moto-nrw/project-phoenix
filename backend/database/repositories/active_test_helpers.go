package repositories

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
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

func NewActiveTestRepositories(db *bun.DB, clocks ...func() time.Time) (ActiveTestRepositories, error) {
	tt, err := NewTimetableTestRepositories(db, clocks...)
	if err != nil {
		return ActiveTestRepositories{}, err
	}
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return ActiveTestRepositories{}, err
	}
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
	r.bindStaffProjections(lazyStaffLookup{get: func() schoolmembership.Capability { return membership }}, workTime)
	r.BindPeopleDirectory(people)
	r.BindSchoolStructure(groups)
	return ActiveTestRepositories{TimetableTestRepositories: tt,
		SessionStartLock: newStudentPresence(db), GroupMapping: newStudentPresence(db), CrossTenant: r.CrossTenant}, nil
}
