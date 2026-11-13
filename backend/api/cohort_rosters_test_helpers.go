//go:build testhooks

package api

import (
	"log/slog"

	careStudentsHTTP "github.com/moto-ogs/project-phoenix/modules/careplan/http"
	transactionruntime "github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
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
