package presence

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeModel "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type checkoutSupervisions struct {
	StudentPresence
	query func(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
}

func (p checkoutSupervisions) QueryGroupSupervisions(ctx context.Context, filter studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error) {
	return p.query(ctx, filter)
}

func TestDeviceSupervisorReadPreservesFailureClassification(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		rows        []studentpresence.GroupSupervision
		err         error
		unavailable bool
	}{
		{name: "missing", unavailable: true},
		{name: "partial failure", rows: []studentpresence.GroupSupervision{{StaffID: 9, StartDate: "2020-01-01"}}, err: errors.New("storage unavailable")},
		{name: "found", rows: []studentpresence.GroupSupervision{{StaffID: 9, StartDate: "2020-01-01"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			group := &activeModel.Group{}
			group.ID = 7
			s := &service{
				GroupRepo: &mockGroupRepository{findActiveByDeviceIDFunc: func(got context.Context, id int64) (*activeModel.Group, error) {
					assert.Same(t, ctx, got)
					assert.Equal(t, int64(8), id)
					return group, nil
				}},
				SchoolPresence: checkoutSupervisions{query: func(got context.Context, filter studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error) {
					assert.Same(t, ctx, got)
					assert.Equal(t, []int64{group.ID}, filter.GroupIDs)
					require.NotNil(t, filter.ActiveOn)
					return tc.rows, tc.err
				}},
			}
			id, err := s.getDeviceSupervisorID(ctx, 8)
			var unavailable *deviceSupervisorUnavailableError
			if tc.unavailable {
				require.ErrorAs(t, err, &unavailable)
				assert.Zero(t, id)
			} else if tc.err != nil {
				require.ErrorIs(t, err, ErrDatabaseOperation)
				assert.False(t, errors.As(err, &unavailable))
				assert.Zero(t, id)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.rows[0].StaffID, id)
			}
		})
	}
}
