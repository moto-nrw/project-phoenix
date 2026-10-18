package integration

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This source reaches the real factory binding, not a fake permission gate.
type parentChildReadRecords struct{ row *parentModels.ChildSummary }

func (s parentChildReadRecords) FindForAccount(context.Context, int64, int64) (*parentModels.ChildSummary, error) {
	return s.row, nil
}
func (s parentChildReadRecords) ListByAccount(context.Context, int64) ([]*parentModels.ChildSummary, error) {
	return []*parentModels.ChildSummary{s.row}, nil
}

func TestParentChildBindingPreservesSharedStoredValuePolicy(t *testing.T) {
	t.Parallel()
	var typedNil *bool
	factory := &api.RepositoryTestFactory{}
	factory.ParentChild = parentChildReadRecords{row: &parentModels.ChildSummary{
		GuardianPermissions: map[string]interface{}{
			"true": true, "false": false, "nil": nil, "legacy string": "false", "zero": 0, "typed nil": typedNil,
		},
	}}
	reader := factory.ParentChildReads()
	child, err := reader.FindForAccount(context.Background(), 11, 23)
	require.NoError(t, err)
	require.NotNil(t, child)
	assert.Equal(t, map[string]bool{
		"true": true, "false": false, "nil": false, "legacy string": true, "zero": true, "typed nil": true,
	}, child.GuardianPermissions)
}
