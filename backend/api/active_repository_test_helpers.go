//go:build testhooks

package api

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// ActiveTestRepositories are the Student Presence rows of the active
// suites: the live sessions and supervisions, the session-start lock, the
// group mapping and the cross-tenant query. The People Directory, Timetable,
// Care Plan and Facilities rows come from the members the suite passes and
// NewActiveTestTimetableRows, NewActiveTestCarePlanRows and
// NewActiveTestRoomRows (#4080).
type ActiveTestRepositories struct {
	TimetablePresenceRows
	SessionStartLock interface {
		LockSessionStart(context.Context, int64) error
	}
	GroupMapping *studentpresence.Module
	CrossTenant  CrossTenantQuery
}

// NewActiveTestRepositories composes the active suites' presence rows over
// the People Directory and School Membership repositories in members.
func NewActiveTestRepositories(db transactionruntime.Database, members MembershipTestRepositories, clocks ...func() time.Time) (ActiveTestRepositories, error) {
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members, clocks...)
	if err != nil {
		return ActiveTestRepositories{}, err
	}
	presence, err := NewTimetablePresenceRows(db, tt, members, clocks...)
	if err != nil {
		return ActiveTestRepositories{}, err
	}
	groups, err := NewSchoolStructure(db)
	if err != nil {
		return ActiveTestRepositories{}, err
	}
	return ActiveTestRepositories{TimetablePresenceRows: presence,
		SessionStartLock: NewUnloggedStudentPresence(db), GroupMapping: NewUnloggedStudentPresence(db),
		CrossTenant: NewCrossTenantQuery(NewUnloggedStudentPresence(db), members.People, groups)}, nil
}

// NewActiveTestPeopleRows are the People Directory rows of the active suites:
// the members they pass.
func NewActiveTestPeopleRows(_ transactionruntime.Database, members MembershipTestRepositories, _ ...func() time.Time) MembershipTestRepositories {
	return members
}

// NewActiveTestTimetableRows are the Timetable rows of the active suites over
// members.
func NewActiveTestTimetableRows(db transactionruntime.Database, members MembershipTestRepositories, clocks ...func() time.Time) TimetableTestRepositories {
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members, clocks...)
	if err != nil {
		panic(fmt.Sprintf("active test rows: compose timetable: %v", err))
	}
	return tt
}

// NewActiveTestCarePlanRows are the Care Plan rows of the active suites over
// members.
func NewActiveTestCarePlanRows(db transactionruntime.Database, members MembershipTestRepositories, clocks ...func() time.Time) TimetableCarePlanRows {
	rows, err := NewTimetableCarePlanRows(db, NewActiveTestTimetableRows(db, members, clocks...), members)
	if err != nil {
		panic(fmt.Sprintf("active test rows: compose care plan: %v", err))
	}
	return rows
}

// NewActiveTestRoomRows are the Facilities rooms of the active suites.
func NewActiveTestRoomRows(db transactionruntime.Database, _ MembershipTestRepositories, _ ...func() time.Time) TimetableRoomRows {
	return MustNewTimetableRoomRows(db)
}
