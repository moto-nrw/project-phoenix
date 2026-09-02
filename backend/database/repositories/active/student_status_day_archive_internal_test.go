package active

import (
	"context"
	"strconv"
	"testing"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type archiveStudentDirectory struct {
	listCalls int
	calls     []string
}

func (d *archiveStudentDirectory) ListStudentsByID(context.Context, []int64) ([]DirectoryStudent, error) {
	return nil, nil
}

func (d *archiveStudentDirectory) ListStudentsAcrossTenantsByID(context.Context, []int64) ([]DirectoryStudent, error) {
	return nil, nil
}

func (d *archiveStudentDirectory) ListActiveStudents(context.Context) ([]DirectoryStudent, error) {
	return nil, nil
}

func (d *archiveStudentDirectory) ListStudentsWithStatusFlag(context.Context, string) ([]DirectoryStudent, error) {
	d.listCalls++
	d.calls = append(d.calls, "list")
	if d.listCalls == 1 {
		return []DirectoryStudent{{ID: 2}, {ID: 1}}, nil
	}
	return []DirectoryStudent{}, nil
}

func (d *archiveStudentDirectory) ClearStudentStatusFlags(context.Context, []int64, string) (int64, error) {
	return 0, nil
}

func (d *archiveStudentDirectory) LockStudent(_ context.Context, studentID int64) error {
	d.calls = append(d.calls, "lock:"+strconv.FormatInt(studentID, 10))
	return nil
}

func TestArchiveAndClearStatusFlagLocksAndRereadsCandidates(t *testing.T) {
	t.Parallel()

	students := &archiveStudentDirectory{}
	repo := &StudentStatusDayRepository{students: students}
	date := activeModels.StudentStatusDay{}.Date
	affected, err := repo.ArchiveAndClearStatusFlag(
		context.Background(), "sick", "sick_since", activeModels.StudentStatusDaySick,
		date, time.Now(), activeModels.StudentStatusSourceEndOfDay,
	)
	require.NoError(t, err)
	assert.Zero(t, affected)
	assert.Equal(t, []string{"list", "lock:1", "lock:2", "list"}, students.calls)
}
