package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type targetedSchemaFailure struct {
	DecisionSchemas
	err error
}

func (s targetedSchemaFailure) Schema(context.Context, int64) (*enrollment.FormSchema, error) {
	return nil, s.err
}

func TestTargetedFieldsPreservePinnedSchemaReadFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("pinned schema unavailable")
	decisions := &Decisions{deps: DecisionDependencies{
		Schemas: targetedSchemaFailure{err: failure},
	}}
	var schemaID int64 // The fake does not access storage; only a non-nil pin is needed.
	request := &enrollmentModels.Request{SchemaID: &schemaID}
	changed, err := decisions.applyTargetedFields(t.Context(), request, nil, nil, nil, 0, targetedFieldSyncOptions{})
	require.ErrorIs(t, err, failure)
	require.False(t, changed)
}
