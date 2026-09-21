package behavior_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staffIdentityStub struct {
	identityaccess.CallerIdentities
	staffID int64
	err     error
}

func (s staffIdentityStub) StaffID(context.Context) (int64, error) { return s.staffID, s.err }

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
			rows := repositories.NewCallerRows(identityaccess.CallerContext{
				CallerIdentities: staffIdentityStub{staffID: tc.staffID, err: tc.err},
			}, repositories.CallerRowSources{})
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
