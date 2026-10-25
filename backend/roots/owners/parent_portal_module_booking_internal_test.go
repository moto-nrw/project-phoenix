package owners

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

type childQueriesStub struct {
	children []*careplan.ParentPortalChild
}

func (s childQueriesStub) FindForAccount(_ context.Context, _, studentID int64) (*careplan.ParentPortalChild, error) {
	for _, child := range s.children {
		if child.StudentID == studentID {
			return child, nil
		}
	}
	return nil, nil
}

func (s childQueriesStub) ListByAccount(context.Context, int64) ([]*careplan.ParentPortalChild, error) {
	return s.children, nil
}

func twoSchoolChildren() childQueriesStub {
	return childQueriesStub{children: []*careplan.ParentPortalChild{
		{StudentID: 1, TenantID: 41},
		{StudentID: 2, TenantID: 42},
	}}
}

// A guardian with children at two schools sees only the children of the
// school that booked Eltern-App (#4895).
func TestParentChildrenOfASchoolWithoutElternAppAreHidden(t *testing.T) {
	t.Parallel()
	reads, err := ParentChildReadsForBookedSchools(NewParentChildReads(twoSchoolChildren()), cancelledModules{off: map[int64]string{42: "module.eltern_app"}})
	require.NoError(t, err)

	children, err := reads.ListByAccount(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, int64(41), children[0].TenantID)

	child, err := reads.FindForAccount(context.Background(), 7, 2)
	require.NoError(t, err)
	assert.Nil(t, child, "a per-child flow treats the child as not linked")

	child, err = reads.FindForAccount(context.Background(), 7, 1)
	require.NoError(t, err)
	assert.NotNil(t, child)
}

func TestParentChildrenFailClosedOnReadErrors(t *testing.T) {
	t.Parallel()
	readErr := errors.New("settings unavailable")
	reads, err := ParentChildReadsForBookedSchools(NewParentChildReads(twoSchoolChildren()), cancelledModules{err: readErr})
	require.NoError(t, err)

	_, err = reads.ListByAccount(context.Background(), 7)
	require.ErrorIs(t, err, readErr)
	_, err = reads.FindForAccount(context.Background(), 7, 1)
	require.ErrorIs(t, err, readErr)
}
