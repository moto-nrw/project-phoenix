package parentrequestfeedprojection_test

import (
	"context"
	"testing"
	"time"

	feedCompose "github.com/moto-ogs/project-phoenix/modules/parentrequestfeedprojection/compose"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestListNewParentRequestsQueryBudgetGrowth(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	add := func(count int) {
		for range count {
			chain := testpkg.CreateTestParentGuardianChain(t, db)
			request := testpkg.CreateTestEnrollmentRequestAwaitingGuardian(t, db, phase.ID, chain.Email, "feed-growth-"+chain.Email)
			var childID int64
			require.NoError(t, db.NewRaw(`INSERT INTO enrollment.request_children
 (tenant_id, request_id, first_name, last_name, date_of_birth, status, created_student_id)
 VALUES (?, ?, 'Feed', 'Child', '2018-04-15', 'approved', ?) RETURNING id`, chain.TenantID, request.ID, chain.StudentID).Scan(testpkg.Ctx(t), &childID))

			_, err := db.NewRaw(`INSERT INTO users.student_data_change_requests
   (tenant_id, student_id, submitted_by, target, field_key, new_value, status)
   VALUES (?, ?, ?, 'student', 'school_class', '"2a"'::jsonb, 'pending')`, chain.TenantID, chain.StudentID, chain.AccountID).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
			_, err = db.NewRaw(`INSERT INTO schedule.care_schedule_change_requests
   (tenant_id, student_id, submitted_by, payload, status) VALUES (?, ?, ?, '{}'::jsonb, 'pending')`, chain.TenantID, chain.StudentID, chain.AccountID).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
			_, err = db.NewRaw(`INSERT INTO enrollment.offering_change_requests
   (tenant_id, student_id, request_child_id, submitted_by, payload, effective_from, status)
   VALUES (?, ?, ?, ?, '{"offerings":[]}'::jsonb, CURRENT_DATE, 'pending')`, chain.TenantID, chain.StudentID, childID, chain.AccountID).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
			_, err = db.NewRaw(`INSERT INTO active.excused_absence_requests
   (tenant_id, student_id, submitted_by, dates, note, status, absence_status)
   VALUES (?, ?, ?, to_jsonb(ARRAY[CURRENT_DATE::TEXT]), 'Feed growth', 'pending', 'excused')`, chain.TenantID, chain.StudentID, chain.AccountID).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
			_, err = db.NewRaw(`INSERT INTO enrollment.change_requests
   (tenant_id, request_id, request_child_id, origin, status, base_snapshot, proposed_snapshot, diff_json)
   VALUES (?, ?, ?, 'parent', 'pending_review', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb)`, chain.TenantID, request.ID, childID).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
		}
	}
	counter := testpkg.CaptureQueries(t, db)
	run := func(expectedPerKind int) []string {
		var queries []string
		require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
			counter.Reset()
			entries, err := feedCompose.NewReader(func(context.Context) (bun.IDB, error) { return tx, nil }).ListNewParentRequests(ctx, testpkg.Tenant(t), time.Now().Add(-24*time.Hour), true, true)
			require.NoError(t, err)
			queries = counter.Queries()
			byKind := map[string]int{}
			for _, entry := range entries {
				byKind[entry.Kind]++
			}
			require.Equal(t, map[string]int{"master_data": expectedPerKind, "care_schedule": expectedPerKind, "offering": expectedPerKind, "excused_absence": expectedPerKind, "enrollment": expectedPerKind}, byKind)
			return nil
		}))
		return queries
	}
	add(3)
	small := run(3)
	add(5)
	large := run(8)
	require.Equal(t, len(small), len(large))
	t.Logf("five queues: N3=%d N8=%d statements; rows=15/40", len(small), len(large))
	querybudgets.AssertQueryBudget(t, "modules.careplan.request_feed.list", counter.Measurement(large))
}
