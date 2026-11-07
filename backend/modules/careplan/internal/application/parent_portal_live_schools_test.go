package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	parentModels "github.com/moto-nrw/project-phoenix/modules/careplan/internal/rows"
	carePlanBindingPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

const (
	liveSchoolID     = 4101
	inactiveSchoolID = 4102
	deletedSchoolID  = 4103
	parentAccountID  = 5101
)

type stubParentChildren struct{ children []*parentModels.ChildSummary }

func (s stubParentChildren) ListByAccount(context.Context, int64) ([]*parentModels.ChildSummary, error) {
	out := make([]*parentModels.ChildSummary, 0, len(s.children))
	for _, child := range s.children {
		copied := *child
		out = append(out, &copied)
	}
	return out, nil
}

func (s stubParentChildren) FindForAccount(_ context.Context, _, studentID int64) (*parentModels.ChildSummary, error) {
	for _, child := range s.children {
		if child.StudentID == studentID {
			copied := *child
			return &copied, nil
		}
	}
	return nil, nil
}

type stubParentSchools []carePlanBindingPorts.ParentPortalSchool

func (s stubParentSchools) ListParentPortalSchools(context.Context, []int64) ([]carePlanBindingPorts.ParentPortalSchool, error) {
	return s, nil
}

func liveSchoolChildren() SchoolChildRepository {
	children := stubParentChildren{children: []*parentModels.ChildSummary{
		{StudentID: 6101, TenantID: liveSchoolID, FirstName: "Ada"},
		{StudentID: 6102, TenantID: inactiveSchoolID, FirstName: "Ben"},
		{StudentID: 6103, TenantID: deletedSchoolID, FirstName: "Cem"},
	}}
	schools := stubParentSchools{
		{ID: liveSchoolID, Name: "Offen", Active: true},
		{ID: inactiveSchoolID, Name: "Abgeschaltet", Active: false},
		{ID: deletedSchoolID, Name: "Geloescht", Active: true, Deleted: true},
	}
	return ParentPortalRepositories{ParentChild: children}.WithSchools(schools).ParentChild.(SchoolChildRepository)
}

// A school the operator switched inactive or deleted locks its staff out;
// its children leave the parents portal with it, so no sick note, care
// exception or message reaches a school nobody reads.
func TestSchoolChildRepositoryKeepsOnlyChildrenOfLiveSchools(t *testing.T) {
	t.Parallel()
	repository := liveSchoolChildren()

	children, err := repository.ListByAccount(context.Background(), parentAccountID)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, int64(6101), children[0].StudentID)
	assert.Equal(t, "Offen", children[0].SchoolName)

	live, err := repository.FindForAccount(context.Background(), parentAccountID, 6101)
	require.NoError(t, err)
	require.NotNil(t, live)
	assert.Equal(t, "Offen", live.SchoolName)

	for _, studentID := range []int64{6102, 6103} {
		child, err := repository.FindForAccount(context.Background(), parentAccountID, studentID)
		require.NoError(t, err)
		assert.Nil(t, child, "student %d of a school that is not live must read as not linked", studentID)
	}
}
