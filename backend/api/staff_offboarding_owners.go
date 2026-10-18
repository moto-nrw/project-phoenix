package api

import (
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	offboardingcompose "github.com/moto-nrw/project-phoenix/workflows/staffoffboarding/compose"
)

// NewStaffOffboardingOwners binds the owners the staff offboarding workflow
// writes through: School Membership (its staff reads and the retirement),
// Timetable's offboarding and Student Presence's supervision lock, built over
// db without observation as the workflow composition did before (#4004, ADR
// 0059 point 6). The caller adds the access, authorization and cleanup steps.
func NewStaffOffboardingOwners(db transactionruntime.Database, people staffoffboarding.People, employment schoolMembershipCompose.StaffEmployment) (offboardingcompose.Dependencies, error) {
	memberDeps := schoolMembershipCompose.Dependencies{DB: db.Pool(), Observe: func(schoolMembershipCompose.Observation) {}, Employment: employment}
	membership, err := schoolMembershipCompose.New(memberDeps)
	if err != nil {
		return offboardingcompose.Dependencies{}, err
	}
	retirement, err := schoolMembershipCompose.NewOffboarding(memberDeps)
	if err != nil {
		return offboardingcompose.Dependencies{}, err
	}
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: db.Pool(), Observe: func(presenceCompose.Observation) {}})
	if err != nil {
		return offboardingcompose.Dependencies{}, err
	}
	timetable, err := timetableCompose.NewOffboarding(timetableCompose.OffboardingDependencies{DB: db.Pool(), Sessions: timetableCompose.NewPresenceSessionFacts(presence), Observe: func(timetableCompose.Observation) {}})
	if err != nil {
		return offboardingcompose.Dependencies{}, err
	}
	return offboardingcompose.Dependencies{
		DB: db.Pool(), People: people, Membership: membership, Retirement: retirement, Timetable: timetable,
		LockSupervision: presence.LockStaffSupervision,
	}, nil
}
