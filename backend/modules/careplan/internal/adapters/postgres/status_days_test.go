package postgres

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type archiveStudentDirectory struct {
	listCalls int
	calls     []string
}

func (d *archiveStudentDirectory) ListEnrolledStudents(context.Context) ([]StatusStudent, error) {
	return nil, nil
}

func (d *archiveStudentDirectory) ListStudentsWithStatusFlag(context.Context, string) ([]StatusStudent, error) {
	d.listCalls++
	d.calls = append(d.calls, "list")
	if d.listCalls == 1 {
		return []StatusStudent{{ID: 2}, {ID: 1}}, nil
	}
	return []StatusStudent{}, nil
}

func (d *archiveStudentDirectory) ClearStudentStatusFlags(context.Context, []int64, string) (int64, error) {
	return 0, nil
}

func (d *archiveStudentDirectory) LockStudent(_ context.Context, studentID int64) error {
	d.calls = append(d.calls, "lock:"+strconv.FormatInt(studentID, 10))
	return nil
}

func TestArchiveStatusFlagsLocksAndRereadsCandidates(t *testing.T) {
	t.Parallel()

	students := &archiveStudentDirectory{}
	store := &statusDayStore{students: students}
	affected, _, err := store.ArchiveStudentStatusFlags(context.Background(), careplan.StatusFlagArchive{
		FlagColumn: "sick", SinceColumn: "sick_since", Status: "sick",
		ReportedFallback: time.Now(), Source: "end_of_day",
	})
	require.NoError(t, err)
	assert.Zero(t, affected)
	assert.Equal(t, []string{"list", "lock:1", "lock:2", "list"}, students.calls)
}
