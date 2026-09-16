package active

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type checkoutVisitsQuery struct {
	StudentPresence
	visits []studentpresence.Visit
}

func (q checkoutVisitsQuery) ListVisits(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
	return q.visits, nil
}

type checkoutGroupsQuery struct {
	AttendanceEducationGroups
	read    func(context.Context, []int64) (map[int64]int64, error)
	readOne func(context.Context, int64) (*int64, error)
}

func (q checkoutGroupsQuery) StudentGroupID(ctx context.Context, id int64) (*int64, error) {
	return q.readOne(ctx, id)
}

func (q checkoutGroupsQuery) StudentGroupIDs(ctx context.Context, ids []int64) (map[int64]int64, error) {
	return q.read(ctx, ids)
}

func TestCheckoutRoutingFailureKeepsOpenVisitsWithoutPartialGroups(t *testing.T) {
	t.Parallel()
	exited := time.Date(2026, time.July, 1, 12, 0, 0, 0, time.UTC)
	svc := &service{ServiceDependencies: ServiceDependencies{
		SchoolPresence: checkoutVisitsQuery{visits: []studentpresence.Visit{
			{ID: 1, StudentID: 42}, {ID: 2, StudentID: 42}, {ID: 3, StudentID: 43, ExitTime: &exited},
		}},
		EducationGroupRepo: checkoutGroupsQuery{read: func(_ context.Context, ids []int64) (map[int64]int64, error) {
			assert.Equal(t, []int64{42}, ids, "routing is batched and deduplicated for open visits only")
			return map[int64]int64{42: 7}, errors.New("routing lookup failed")
		}},
	}}
	visits, err := svc.collectActiveVisitsForSSE(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, visits, 2)
	for _, visit := range visits {
		assert.Equal(t, int64(42), visit.StudentID)
		assert.Nil(t, visit.EducationGroupID)
	}
}

func TestSingleStudentRoutingPreservesUnknownAndPresentGroups(t *testing.T) {
	t.Parallel()
	zero := int64(0)
	for _, tc := range []struct {
		name  string
		group *int64
		err   error
		want  []string
	}{
		{name: "unknown group"},
		{name: "present zero group", group: &zero, want: []string{"0"}},
		{name: "failed lookup discards partial group", group: &zero, err: errors.New("routing lookup failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := &service{ServiceDependencies: ServiceDependencies{
				EducationGroupRepo: checkoutGroupsQuery{readOne: func(_ context.Context, id int64) (*int64, error) {
					assert.Equal(t, int64(42), id)
					return tc.group, tc.err
				}},
			}}
			group := svc.getEducationGroupForSSE(context.Background(), 42)
			assert.Equal(t, tc.want, eduGroupIDsOf(group))
		})
	}
}
