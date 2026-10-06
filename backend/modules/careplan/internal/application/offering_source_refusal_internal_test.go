package application

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// recordingSourceRules records the refusals the catalog raises.
type recordingSourceRules struct{ refusals []ports.OfferingSourceRefusal }

func (r *recordingSourceRules) MaxSourcesPerTemplate() int { return 2 }
func (r *recordingSourceRules) Reject(reason string) error { return errors.New(reason) }
func (r *recordingSourceRules) RejectWith(refusal ports.OfferingSourceRefusal) error {
	r.refusals = append(r.refusals, refusal)
	return errors.New(refusal.Reason)
}
func (r *recordingSourceRules) IsRejection(error) bool { return true }

// TestOfferingSourceRefusalKinds pins the kind and values each offering-source
// refusal hands to the Timetable owner's rules (#2516), which turn them into
// the editor's codes.
func TestOfferingSourceRefusalKinds(t *testing.T) {
	t.Parallel()

	rules := &recordingSourceRules{}
	catalog := &CareOfferingCatalog{deps: CareOfferingCatalogDependencies{SourceRules: rules}}

	require.Error(t, catalog.checkOfferingSourceIDs([]int64{11, 12, 13}))
	inactive := careplan.CareOffering{ID: 11, Name: "Ferienbetreuung"}
	require.Error(t, catalog.acceptOfferingSource(t.Context(), &careplan.OfferingSources{}, inactive, nil, false))

	require.Len(t, rules.refusals, 2)
	assert.Equal(t, ports.OfferingSourceTooMany, rules.refusals[0].Kind)
	assert.Equal(t, 2, rules.refusals[0].Max)
	assert.Equal(t, 3, rules.refusals[0].Given)
	assert.Equal(t, ports.OfferingSourceInactive, rules.refusals[1].Kind)
	assert.Equal(t, "Ferienbetreuung", rules.refusals[1].Offering)
}
