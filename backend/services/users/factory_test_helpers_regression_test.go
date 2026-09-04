package users_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/timetable/timetabletest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFactoryWithPeopleDirectoryBindsTimetableForCareExit(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory, err := repositories.NewFactoryWithPeopleDirectory(db, timetabletest.New(t, db))
	require.NoError(t, err)

	student := testpkg.CreateTestStudent(t, db, "Care", "Exit", "1a")
	affected, err := factory.CareExitCleanup.CapByStudentIDs(testpkg.Ctx(t), []int64{student.ID}, timezone.TodayDate())

	require.NoError(t, err)
	assert.Zero(t, affected)
}
