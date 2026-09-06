package enrollment

import (
	"context"
	"errors"
	"testing"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/stretchr/testify/require"
)

type targetedSchemaFailure struct {
	SchemaReader
	err error
}

func (s targetedSchemaFailure) Schema(context.Context, int64) (*capability.FormSchema, error) {
	return nil, s.err
}

func TestTargetedFieldsPreservePinnedSchemaReadFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("pinned schema unavailable")
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{
		Schemas: targetedSchemaFailure{err: failure},
	}}
	var schemaID int64 // The fake does not access storage; only a non-nil pin is needed.
	request := &enrollmentModels.Request{SchemaID: &schemaID}
	changed, err := svc.applyTargetedFields(t.Context(), request, nil, nil, nil, 0, targetedFieldSyncOptions{})
	require.ErrorIs(t, err, failure)
	require.False(t, changed)
}
