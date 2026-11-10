package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type demoSchoolDatabaseStoreDouble struct {
	ids      []int64
	listErr  error
	outcomes map[int64]demoSchoolErasureOutcome
	erased   []int64
}

type demoSchoolErasureOutcome struct {
	status string
	code   string
	err    error
}

func (s *demoSchoolDatabaseStoreDouble) DueDemoSchoolDatabases(context.Context, int) ([]int64, error) {
	return s.ids, s.listErr
}

func (s *demoSchoolDatabaseStoreDouble) EraseDemoSchoolDatabase(_ context.Context, id int64) (string, string, error) {
	s.erased = append(s.erased, id)
	outcome := s.outcomes[id]
	return outcome.status, outcome.code, outcome.err
}

// A failed school fails the run without stopping the others; a busy school
// and one no longer due do not. Only erased schools count (#5559).
func TestEraseRetiredDemoSchoolDatabasesReportsEveryFailedSchool(t *testing.T) {
	t.Parallel()
	store := &demoSchoolDatabaseStoreDouble{
		ids: []int64{11, 12, 13, 14, 15},
		outcomes: map[int64]demoSchoolErasureOutcome{
			11: {status: ports.DemoSchoolErased},
			12: {status: ports.DemoSchoolFailed, code: "23503"},
			13: {status: ports.DemoSchoolBusy},
			14: {err: errors.New("connection reset")},
			15: {status: ports.DemoSchoolNotDue},
		},
	}
	databases, err := NewDemoSchoolDatabases(store)
	require.NoError(t, err)

	erased, busy, err := databases.EraseRetiredDemoSchoolDatabases(t.Context())

	assert.Equal(t, 1, erased)
	assert.Equal(t, 1, busy)
	assert.Equal(t, []int64{11, 12, 13, 14, 15}, store.erased, "a failure does not stop the batch")
	require.Error(t, err)
	assert.ErrorContains(t, err, "demo school 12")
	assert.ErrorContains(t, err, `"23503"`)
	assert.ErrorContains(t, err, "demo school 14")
	assert.NotContains(t, err.Error(), "demo school 13")
}

func TestEraseRetiredDemoSchoolDatabasesSucceedsWithBusySchoolsOnly(t *testing.T) {
	t.Parallel()
	store := &demoSchoolDatabaseStoreDouble{ids: []int64{21}, outcomes: map[int64]demoSchoolErasureOutcome{21: {status: ports.DemoSchoolBusy}}}
	databases, err := NewDemoSchoolDatabases(store)
	require.NoError(t, err)
	erased, busy, err := databases.EraseRetiredDemoSchoolDatabases(t.Context())
	require.NoError(t, err)
	assert.Zero(t, erased)
	assert.Equal(t, 1, busy)
}
