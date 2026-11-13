package operator

import (
	"context"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type erasureOrderRecordsDouble struct{}

func (erasureOrderRecordsDouble) RecordTraegerErasureOrderRecords(context.Context, organizationtenancy.RecordTraegerErasureOrderRecords) (organizationtenancy.TraegerErasureOrder, error) {
	return organizationtenancy.TraegerErasureOrder{}, nil
}

// The records route depends on its narrow capability, injected at
// construction (ADR 0068 point 5, ADR 0178): without it nothing is built.
func TestNewTraegerErasureOrderRecordsResourceRequiresTheCapability(t *testing.T) {
	t.Parallel()
	resource, err := NewTraegerErasureOrderRecordsResource(nil)
	require.Error(t, err)
	assert.Nil(t, resource)

	resource, err = NewTraegerErasureOrderRecordsResource(erasureOrderRecordsDouble{})
	require.NoError(t, err)
	assert.NotNil(t, resource)
}
