package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDescribeStudentOwnerUnreconciledNamesBothVerdicts(t *testing.T) {
	t.Parallel()
	require.Equal(t,
		"student owner cutover would refuse: tenant 4: 2 unreconciled guardian values; "+
			"tenant 5: students without a completed backfill pass; "+
			"tenant 7: 3 unreconciled guardian values, students without a completed backfill pass",
		describeStudentOwnerUnreconciled([]studentOwnerUnreconciled{
			{TenantID: 4, Guardian: 2},
			{TenantID: 5, NeverBackfilled: true},
			{TenantID: 7, Guardian: 3, NeverBackfilled: true},
		}))
}

// A nil database is a wiring mistake, not a passing precondition.
func TestStudentOwnerCutoverPreconditionRequiresADatabase(t *testing.T) {
	t.Parallel()
	require.ErrorContains(t, studentOwnerCutoverPrecondition(context.Background(), nil), "database is required")
}
