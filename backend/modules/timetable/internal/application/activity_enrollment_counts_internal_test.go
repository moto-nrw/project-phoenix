package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Only the faulting port methods are replaced; the existing interfaces supply
// the unrelated method set rather than another full-interface mock.
type countEnrollmentRows struct {
	timetable.Capability
	rows []timetable.StudentEnrollment
}

func (s countEnrollmentRows) ListStudentEnrollments(_ context.Context, filter timetable.StudentEnrollmentFilter) ([]timetable.StudentEnrollment, error) {
	var rows []timetable.StudentEnrollment
	for _, row := range s.rows {
		for _, groupID := range filter.ActivityGroupIDs {
			if row.ActivityGroupID == groupID {
				rows = append(rows, row)
				break
			}
		}
	}
	return rows, nil
}

type countStudentReader struct {
	ActivityStudents
	read func([]int64) (map[int64]*timetable.OperationStudent, error)
}

func (s countStudentReader) FindByIDs(_ context.Context, ids []int64) (map[int64]*timetable.OperationStudent, error) {
	return s.read(ids)
}

func TestEnrollmentCountsKeepOrderedReaderFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	first := testpkg.CreateTestActivityGroup(t, db, "First count fault")
	second := testpkg.CreateTestActivityGroup(t, db, "Second count fault")
	child := testpkg.CreateTestStudent(t, db, "First", "Child", "1a")
	later := testpkg.CreateTestStudent(t, db, "Later", "Child", "1b")
	wantErr, laterErr := errors.New("first roster student reader failed"), errors.New("later roster names failed")
	for _, emptyFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "nonempty", true: "empty"}[emptyFirst], func(t *testing.T) {
			rows := []timetable.StudentEnrollment{{ActivityGroupID: second.ID, StudentID: later.ID}}
			if !emptyFirst {
				rows = append(rows, timetable.StudentEnrollment{ActivityGroupID: first.ID, StudentID: child.ID})
			}
			directories := ports.RepositoryDirectories{
				StudentAlumni: func(context.Context, []int64) (map[int64]bool, error) { return nil, nil },
				StudentNames: func(_ context.Context, ids []int64) (map[int64]ports.PersonName, error) {
					for _, id := range ids {
						if id == later.ID {
							return nil, laterErr
						}
					}
					return nil, nil
				},
			}
			students := countStudentReader{read: func([]int64) (map[int64]*timetable.OperationStudent, error) { return nil, wantErr }}
			service, err := NewActivityService(countEnrollmentRows{rows: rows}, nil, directories, nil, nil, students)
			require.NoError(t, err)
			_, scalarErr := service.GetEnrolledStudents(testpkg.Ctx(t), first.ID)
			_, err = service.GetEnrollmentCounts(testpkg.Ctx(t), []int64{first.ID, second.ID})
			require.ErrorIs(t, err, wantErr)
			require.EqualError(t, err, scalarErr.Error())
		})
	}
}

func TestEnrollmentCountsDoNotDropEmptyRosterReaderFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	empty := testpkg.CreateTestActivityGroup(t, db, "Empty count fault")
	populated := testpkg.CreateTestActivityGroup(t, db, "Populated count")
	child := testpkg.CreateTestStudent(t, db, "Later", "Child", "1a")
	wantErr := errors.New("empty roster student reader failed")
	students := countStudentReader{read: func(ids []int64) (map[int64]*timetable.OperationStudent, error) {
		if len(ids) == 0 {
			return nil, wantErr
		}
		return map[int64]*timetable.OperationStudent{}, nil
	}}
	directories := ports.RepositoryDirectories{
		StudentAlumni: func(context.Context, []int64) (map[int64]bool, error) { return nil, nil },
		StudentNames:  func(context.Context, []int64) (map[int64]ports.PersonName, error) { return nil, nil },
	}
	service, err := NewActivityService(countEnrollmentRows{rows: []timetable.StudentEnrollment{{ActivityGroupID: populated.ID, StudentID: child.ID}}}, nil, directories, nil, nil, students)
	require.NoError(t, err)
	_, scalarErr := service.GetEnrolledStudents(testpkg.Ctx(t), empty.ID)
	_, err = service.GetEnrollmentCounts(testpkg.Ctx(t), []int64{empty.ID, populated.ID})
	require.ErrorIs(t, err, wantErr)
	require.EqualError(t, err, scalarErr.Error())
}
