package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// database/sql's Commit returns the ended context's error without sending
// COMMIT, so that transaction rolled back: its outcome is known (Sentry
// BACKEND-1C). Only a COMMIT that may have reached the server stays unknown.
func TestCommitOutcome(t *testing.T) {
	t.Parallel()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		name    string
		ctx     context.Context
		err     error
		want    error
		unknown bool
	}{
		{"committed", context.Background(), nil, nil, false},
		{"cancelled_before_commit", canceled, context.Canceled, context.Canceled, false},
		{"deadline_before_commit", canceled, context.DeadlineExceeded, context.DeadlineExceeded, false},
		{"automatic_rollback_won", canceled, sql.ErrTxDone, context.Canceled, false},
		{"bad_conn_after_commit_sent", context.Background(), driver.ErrBadConn, driver.ErrBadConn, true},
		{"bad_conn_after_cancellation", canceled, driver.ErrBadConn, driver.ErrBadConn, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := commitOutcome(c.ctx, c.err)
			var outcome interface{ CommitOutcomeUnknown() }
			require.Equal(t, c.unknown, errors.As(err, &outcome), "commit outcome unknown: %v", err)
			if c.want == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, c.want)
		})
	}
}
