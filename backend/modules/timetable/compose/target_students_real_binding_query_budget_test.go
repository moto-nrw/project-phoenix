//go:build testhooks

package compose_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Both target groups and matching children grow while the real directory stays bulk-loaded.
func TestRealTargetStudentBindingQueryBudgetStaysFlat(t *testing.T) {
	t.Parallel()
	testpkg.OwnTenant(t)
	db := testpkg.SetupIsolatedTestDB(t)
	repositories, err := api.NewTimetableTestRepositories(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	module, ctx := repositories.Timetable, testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	counter := testpkg.CaptureQueriesForContext(t, db)
	class := "2b"
	groupIDs, studentIDs := []int64{}, []int64{}

	measure := func(label string, forTenant int64, ids []int64, expected map[int64][]int64, expectedStatements int) {
		t.Helper()
		var queries []string
		var matched map[int64][]int64
		var writeRows int64
		scoped := testpkg.ContextForTenant(ctx, forTenant)
		callErr := testpkg.WithTenantTx(t, scoped, db, forTenant, func(txCtx context.Context, _ bun.Tx) error {
			counter.Reset()
			var operationErr error
			matched, operationErr = module.ListTargetStudentIDs(counter.Context(txCtx), ids)
			queries, writeRows = counter.Queries(), counter.WriteRows()
			return operationErr
		})
		encoded, encodeErr := json.Marshal(struct {
			Label      string            `json:"label"`
			Matched    map[int64][]int64 `json:"matched"`
			Statements []string          `json:"statements"`
			WriteRows  int64             `json:"driver_dml_rows"`
		}{label, matched, queries, writeRows})
		require.NoError(t, encodeErr)
		t.Logf("real_target_binding observation=%s error=%v", encoded, callErr)
		require.NoError(t, callErr)
		require.NotNil(t, matched)
		require.Equal(t, expected, matched, "exact group keys and sorted unique student IDs")
		require.Zero(t, writeRows)
		require.Len(t, queries, expectedStatements, "business SQL includes both real owners, not transaction setup")
		if label == "groups3_students3" || label == "groups8_students8" {
			querybudgets.AssertQueryBudget(t, "modules.timetable.target_students.real_binding", counter.Measurement(queries))
		}
	}

	measure("empty_ids", tenantID, nil, map[int64][]int64{}, 0)
	withoutTargets := testpkg.CreateTestActivityGroup(t, db, "Target no rules")
	measure("no_target_rules", tenantID, []int64{withoutTargets.ID}, map[int64][]int64{}, 1)
	testpkg.CreateTestStudent(t, db, "Target", "Unmatched class", "9z")
	add := func(from, to int) {
		for i := from; i < to; i++ {
			group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Real target %d", i))
			require.NoError(t, module.ReplaceGroupTargets(ctx, group.ID, []timetable.GroupTargetInput{{
				TargetGroupType: timetable.TargetGroupTypeSchoolClass, TargetSchoolClass: &class,
			}}))
			student := testpkg.CreateTestStudent(t, db, "Target", fmt.Sprintf("Member %d", i), class)
			groupIDs = append(groupIDs, group.ID)
			studentIDs = append(studentIDs, student.ID)
		}
	}
	expected := func() map[int64][]int64 {
		result := make(map[int64][]int64, len(groupIDs))
		for _, id := range groupIDs {
			result[id] = append([]int64{}, studentIDs...)
			slices.Sort(result[id])
		}
		return result
	}
	add(0, 3)
	measure("groups3_students3", tenantID, groupIDs, expected(), 2)
	add(3, 8)
	measure("groups8_students8", tenantID, groupIDs, expected(), 2)

	foreignTenant, _ := testpkg.CreateTestTenant(t, db)
	foreignStudent := testpkg.CreateTestStudentForTenant(t, db, foreignTenant, "Target", "Foreign same class", class)
	foreignGroup := testpkg.CreateTestActivityGroupForTenant(t, db, foreignTenant, "Foreign target")
	foreignCtx := testpkg.ContextForTenant(ctx, foreignTenant)
	require.NoError(t, testpkg.WithTenantTx(t, foreignCtx, db, foreignTenant, func(txCtx context.Context, _ bun.Tx) error {
		return module.ReplaceGroupTargets(txCtx, foreignGroup.ID, []timetable.GroupTargetInput{{
			TargetGroupType: timetable.TargetGroupTypeSchoolClass, TargetSchoolClass: &class,
		}})
	}))
	measure("foreign_group_only", tenantID, []int64{foreignGroup.ID}, map[int64][]int64{}, 1)
	mixedIDs := append(append([]int64{}, groupIDs...), foreignGroup.ID, groupIDs[0])
	measure("mixed_ids_duplicate_and_foreign_student", tenantID, mixedIDs, expected(), 2)
	measure("foreign_tenant_own_result", foreignTenant, []int64{foreignGroup.ID}, map[int64][]int64{foreignGroup.ID: {foreignStudent.ID}}, 2)

	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		counter.Reset()
		matched, callErr := module.ListTargetStudentIDs(counter.Context(txCtx), []int64{-1})
		require.ErrorIs(t, callErr, timetable.ErrInvalidGroupQuery)
		require.Nil(t, matched)
		require.Empty(t, counter.Queries(), "invalid IDs are rejected before persistence")
		require.Zero(t, counter.WriteRows())
		return nil
	}))
	// Cancellation is a separate outer transaction: no failed statement may
	// contaminate another measurement or be mistaken for a clean read result.
	var cancelErr error
	var canceledResult map[int64][]int64
	err = testpkg.WithTenantTx(t, ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		canceled, cancel := context.WithCancel(txCtx)
		cancel()
		counter.Reset()
		canceledResult, cancelErr = module.ListTargetStudentIDs(counter.Context(canceled), groupIDs)
		require.Zero(t, counter.WriteRows())
		return cancelErr
	})
	require.ErrorIs(t, cancelErr, context.Canceled)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, canceledResult)
}
