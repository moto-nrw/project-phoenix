//go:build testhooks

package api

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

type ActiveTestRepositories struct {
	TimetableTestRepositories
	// The other owners' rows beside the Timetable rows (#4080).
	MembershipTestRepositories
	TimetablePresenceRows
	TimetableCarePlanRows
	TimetableRoomRows
	TimetableDeviationAudit
	TimetableSchoolCalendar
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
	presence, care, rooms, calendar, err := newTimetableTestCompanions(db, tt, members, clocks...)
	if err != nil {
		return ActiveTestRepositories{}, err
	}
	people := members.People
	groups, err := NewSchoolStructure(db)
	if err != nil {
		return ActiveTestRepositories{}, err
	}
	return ActiveTestRepositories{TimetableTestRepositories: tt, MembershipTestRepositories: members, TimetablePresenceRows: presence, TimetableCarePlanRows: care, TimetableRoomRows: rooms, TimetableSchoolCalendar: calendar,
		SessionStartLock: NewUnloggedStudentPresence(db), GroupMapping: NewUnloggedStudentPresence(db),
		CrossTenant: NewCrossTenantQuery(NewUnloggedStudentPresence(db), people, groups)}, nil
}
