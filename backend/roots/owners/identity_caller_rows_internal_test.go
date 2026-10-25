package owners

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The caller rows and the caller context refuse to compose without their
// dependencies (ADR 0059): a missing one is a construction error, not a nil
// dereference of the first request.
func TestNewCallerRowsRequiresEveryDependency(t *testing.T) {
	t.Parallel()

	_, err := NewCallerRows(identityaccess.CallerContext{}, nil, nil, nil, nil, nil, nil, slog.Default())
	require.EqualError(t, err, "caller rows: the caller context, the group, staff, teacher, student, activity and session rows and a logger are required")
}

func TestNewCallerContextRequiresEveryDependency(t *testing.T) {
	t.Parallel()

	_, err := NewCallerContext(TokenAdapter{}, nil, CallerPersons{}, nil, nil, nil, nil, nil, nil, slog.Default())
	require.EqualError(t, err, "compose caller context: the accounts, persons, membership, staff groups, school classes, supervised activities, presence and a logger are required")
}

type staffIdentityStub struct {
	identityaccess.CallerIdentities
	staffID int64
	err     error
}

func (s staffIdentityStub) StaffID(context.Context) (int64, error) { return s.staffID, s.err }

// callerRowsOver serves caller over row ports a test never reads.
func callerRowsOver(t *testing.T, caller identityaccess.CallerContext) *CallerRows {
	t.Helper()
	rows, err := NewCallerRows(caller, struct{ groupRows }{}, struct{ staffRows }{}, struct{ teacherRows }{}, struct{ studentRows }{},
		struct{ callerActivityRows }{}, struct{ callerSessionRows }{}, slog.Default())
	require.NoError(t, err)
	return rows
}

func TestCallerRowsCurrentStaffIDClassifiesUnlinkedCallers(t *testing.T) {
	t.Parallel()
	lookupErr := errors.New("database unavailable")
	for _, tc := range []struct {
		name      string
		staffID   int64
		err       error
		wantID    int64
		wantFound bool
		wantErr   error
	}{
		{name: "unlinked person", err: &identityaccess.CallerError{Op: "get current person", Err: identityaccess.ErrCallerNotLinkedToPerson}},
		{name: "wrapped unlinked staff", err: errors.Join(errors.New("lookup"), identityaccess.ErrCallerNotLinkedToStaff)},
		{name: "lookup failure", err: lookupErr, wantErr: lookupErr},
		{name: "staff member", staffID: 42, wantID: 42, wantFound: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rows := callerRowsOver(t, identityaccess.CallerContext{
				CallerIdentities: staffIdentityStub{staffID: tc.staffID, err: tc.err},
			})
			staffID, found, err := rows.CurrentStaffID(context.Background())
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantID, staffID)
			assert.Equal(t, tc.wantFound, found)
		})
	}
}
