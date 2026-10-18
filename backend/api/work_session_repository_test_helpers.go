//go:build testhooks

package api

import (
	"time"

	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// WorkSessionTestRepositories are the Workforce owner's work-session rows.
// The Timetable rows, the people and the live sessions the time tracking
// reads come from the timetable fixtures beside it, the edit trail from the
// Audit persistence.
type WorkSessionTestRepositories struct {
	// Workforce is the capability the time-tracking rows are served from.
	Workforce    workforce.Capability
	StaffAbsence workforceCompose.StaffAbsenceRepository
}

func NewWorkSessionTestRepositoriesWithMembers(db transactionruntime.Database, members MembershipTestRepositories, clocks ...func() time.Time) (WorkSessionTestRepositories, error) {
	people := members.People
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return WorkSessionTestRepositories{}, err
	}
	var now func() time.Time
	if len(clocks) > 0 {
		now = clocks[0]
	}
	workTime, err := NewWorkforceWithClock(db, membership, now)
	if err != nil {
		return WorkSessionTestRepositories{}, err
	}
	return WorkSessionTestRepositories{
		Workforce:    workTime,
		StaffAbsence: NewStaffAbsenceRepository(workTime, membership, people),
	}, nil
}
