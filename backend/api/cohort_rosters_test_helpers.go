//go:build testhooks

package api

import (
	"context"
	"log/slog"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-ogs/project-phoenix/modules/careplan/compose"
	careStudentsHTTP "github.com/moto-ogs/project-phoenix/modules/careplan/http"
	transactionruntime "github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	timezone "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// NewStudentTestCohortRosters composes Timetable's cohort rosters (#6841) as
// the student edit's port, over the real People Directory and timetable rows
// of the test database, the way the production root binds them.
func NewStudentTestCohortRosters(db transactionruntime.Database) (careStudentsHTTP.FollowCohortChanges, error) {
	tt, err := NewTimetableTestRepositories(db)
	if err != nil {
		return nil, err
	}
	people, err := owners.NewPeopleDirectory(db)
	if err != nil {
		return nil, err
	}
	rosters, err := owners.NewTimetableCohortRosters(tt.Repositories, people, nil, slog.Default())
	if err != nil {
		return nil, err
	}
	return studentCohortChanges(rosters), nil
}

// MustEnrollmentTestClassFollowUp binds the class follow-up an Enrollment
// decision runs (#6841) over the test database, as the production root does.
func MustEnrollmentTestClassFollowUp(db transactionruntime.Database) func(context.Context, map[int64]string, timezone.Date) error {
	tt, err := NewTimetableTestRepositories(db)
	if err != nil {
		panic(err)
	}
	people, err := owners.NewPeopleDirectory(db)
	if err != nil {
		panic(err)
	}
	rosters, err := owners.NewTimetableCohortRosters(tt.Repositories, people, nil, slog.Default())
	if err != nil {
		panic(err)
	}
	return owners.FollowClassChanges(rosters)
}

// NewMasterDataTestClassChanges binds the Stammdaten decision's class
// follow-up (#6951) over the test database: the recurrence gate and
// Timetable's cohort rosters. The offering resync needs the booking
// materialization and stays out; its suites cover it.
func NewMasterDataTestClassChanges(db transactionruntime.Database) (carePlanCompose.MasterDataClassChanges, error) {
	lock, err := owners.NewTimetableTestRecurrenceLock(db)
	if err != nil {
		return carePlanCompose.MasterDataClassChanges{}, err
	}
	follow := MustEnrollmentTestClassFollowUp(db)
	return carePlanCompose.MasterDataClassChanges{
		Lock: lock.LockRecurrenceWrites,
		Follow: func(ctx context.Context, previousClasses map[int64]string, from careplan.Date) error {
			return follow(ctx, previousClasses, timezone.Date(from))
		},
	}, nil
}

// importTestCohorts binds the student import's Klasse/Gruppe follow-up
// (#6951) over the test database when the options ask for it, without the
// offering resync.
func importTestCohorts(db transactionruntime.Database, options ImportTestOptions) importCohorts {
	if !options.FollowCohorts {
		return importCohorts{}
	}
	lock, err := owners.NewTimetableTestRecurrenceLock(db)
	if err != nil {
		panic(err)
	}
	tt, err := NewTimetableTestRepositories(db)
	if err != nil {
		panic(err)
	}
	people, err := owners.NewPeopleDirectory(db)
	if err != nil {
		panic(err)
	}
	rosters, err := owners.NewTimetableCohortRosters(tt.Repositories, people, nil, slog.Default())
	if err != nil {
		panic(err)
	}
	return importCohorts{Lock: lock.LockRecurrenceWrites, Follow: importCohortChanges(rosters)}
}
