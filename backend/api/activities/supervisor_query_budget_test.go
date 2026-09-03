package activities

import (
	"context"
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAvailableSupervisorsBySpecializationQueryBudget(t *testing.T) {
	t.Parallel()
	db, resource := setupActivitiesInternalRoute(t)
	specialization := fmt.Sprintf("Budget-%d", time.Now().UnixNano())
	add := func(count int) {
		for i := range count {
			teacher := testpkg.CreateTestTeacher(t, db, fmt.Sprintf("Budget%d", i), "Supervisor")
			_, err := db.NewUpdate().Model(teacher).ModelTableExpr(`users.teachers AS "teacher"`).
				Set("specialization = ?", specialization).Where("id = ?", teacher.ID).Exec(context.Background())
			require.NoError(t, err)
		}
	}
	add(3)
	counter := testpkg.CaptureQueriesForContext(t, db)
	ctx := counter.Context(testpkg.Ctx(t))
	run := func() []string {
		counter.Reset()
		rows, err := resource.fetchSupervisorsBySpecialization(ctx, specialization)
		require.NoError(t, err)
		require.NotEmpty(t, rows)
		return counter.Operation("SELECT")
	}
	small := run()
	add(5)
	large := run()
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "api.activities.available_supervisors.specialization.reads", large)
}
