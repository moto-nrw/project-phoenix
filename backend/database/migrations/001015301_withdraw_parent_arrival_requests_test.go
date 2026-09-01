package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithdrawParentArrivalRequestsLeavesPickupRequestsPending(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	arrivalChain := testpkg.CreateTestParentGuardianChain(t, db)
	pickupChain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := context.Background()

	_, err := db.NewRaw(`
		INSERT INTO schedule.care_schedule_change_requests
			(tenant_id, student_id, submitted_by, payload, status)
		VALUES
			(?, ?, ?, '{"weekdays":[{"weekday":1,"arrival":"08:00"}]}'::jsonb, 'pending'),
			(?, ?, ?, '{"weekdays":[{"weekday":1,"pickup":"15:30"}]}'::jsonb, 'pending')
	`, arrivalChain.TenantID, arrivalChain.StudentID, arrivalChain.AccountID,
		pickupChain.TenantID, pickupChain.StudentID, pickupChain.AccountID).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, withdrawParentArrivalRequestsUp(ctx, db))

	type row struct {
		StudentID int64  `bun:"student_id"`
		Status    string `bun:"status"`
		Reason    string `bun:"decision_reason"`
	}
	var rows []row
	require.NoError(t, db.NewRaw(`
		SELECT student_id, status, COALESCE(decision_reason, '') AS decision_reason
		FROM schedule.care_schedule_change_requests
		WHERE student_id IN (?, ?)
		ORDER BY student_id
	`, arrivalChain.StudentID, pickupChain.StudentID).Scan(ctx, &rows))
	require.Len(t, rows, 2)

	byStudent := map[int64]row{rows[0].StudentID: rows[0], rows[1].StudentID: rows[1]}
	assert.Equal(t, "withdrawn", byStudent[arrivalChain.StudentID].Status)
	assert.Equal(t, withdrawParentArrivalRequestsReason, byStudent[arrivalChain.StudentID].Reason)
	assert.Equal(t, "pending", byStudent[pickupChain.StudentID].Status)
}
