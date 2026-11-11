package timetablehttp

import (
	"context"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gateTakingMaterialization takes the recurrence and grade-transition gates
// in the create's transaction, as the real materialization does, and then
// reports the window it was asked for.
type gateTakingMaterialization struct {
	*mockMaterializationService
	lock timetable.RecurrenceWriteLock
	err  error
}

func (m *gateTakingMaterialization) MaterializeForTenant(ctx context.Context, from, to calendar.Date, source timetable.MaterializationSource) (*timetable.MaterializationResult, error) {
	if m.err = m.lock.LockRecurrenceWritesThenGradeTransitions(ctx); m.err != nil {
		return nil, m.err
	}
	return m.mockMaterializationService.MaterializeForTenant(ctx, from, to, source)
}

// A template create writes its supervisor rows, which take their staff rows,
// and then materializes in the same transaction. Both tenant gates come
// before the staff rows (#6583, tenant.LockStaffRows): otherwise the lock
// order refuses the materialization's grade-transition gate and the new
// template yields no occurrences.
func TestTemplateCreateMaterializesAfterItsSupervisorRows(t *testing.T) {
	t.Parallel()

	mat := &gateTakingMaterialization{
		mockMaterializationService: &mockMaterializationService{result: &timetable.MaterializationResult{InstancesCreated: 2}},
		lock:                       testTimetableData(testpkg.SetupTestDB(t)).RecurrenceLock(),
	}
	s := buildTemplateModule(t, mat)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res, s.db)

	body := createTemplateBody(s, "Tpl-Create-Lock-Order")
	body["materialize_from"] = "2026-05-04"
	body["materialize_to"] = "2026-05-08"
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	require.NoError(t, mat.err, "the materialization takes its gates after the supervisor rows")
	assert.Equal(t, 2, decodeTemplateData[createTemplateResponse](t, w).InstancesCreated)
}
