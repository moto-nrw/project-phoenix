package presence

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVisitAccessTeacherGroupsForwardsContextAndFailure(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wantErr := errors.New("group lookup failed")
	for _, lookupErr := range []error{nil, wantErr} {
		query := visitAccessQuery{resource: resourceForTest(Resource{
			EducationService: func(received context.Context, teacherID int64) ([]int64, error) {
				require.Same(t, ctx, received)
				assert.EqualValues(t, 42, teacherID)
				if lookupErr != nil {
					return nil, lookupErr
				}
				return []int64{17, 23}, nil
			},
		})}
		ids, err := query.TeacherGroupIDs(ctx, 42)
		if lookupErr != nil {
			require.ErrorIs(t, err, lookupErr)
			assert.Nil(t, ids)
		} else {
			require.NoError(t, err)
			assert.Equal(t, []int64{17, 23}, ids)
		}
	}
}
