package contracttest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestOfferingSourceRefusalsCarryTheirCode pins the wire code and values a
// refused offering-source selection carries to the Regeltermin editor
// (#2516); the refusal stays an ErrOfferingSourceInvalid for every
// classifier and keeps its diagnostic text.
func TestOfferingSourceRefusalsCarryTheirCode(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	template := createCareOfferingTemplateGroup(t, env.db, "CodeTermin")
	ids := make([]int64, timetable.MaxOfferingSourcesPerTemplate+1)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	err := offeringResyncer(t, env).ResyncTemplateOfferingRoster(ctx, timetable.OfferingRosterResyncInput{
		TemplateID: template.ID, OfferingIDs: ids, EffectiveFrom: offeringResyncToday,
	})
	require.ErrorIs(t, err, timetable.ErrOfferingSourceInvalid)
	assert.ErrorContains(t, err, "at most")
	coded, ok := timetable.AsCoded(err)
	require.True(t, ok)
	assert.Equal(t, timetable.CodeOfferingSourceTooMany, coded.Code)
	assert.Equal(t, timetable.RefusalValues{Max: timetable.MaxOfferingSourcesPerTemplate, Given: len(ids)}, coded.Values)

	offering := createSourceOffering(t, env, "CodeQuelle", nil)
	missing := int64(999999999)
	err = env.offeringCatalog.ValidateTemplateOfferingSource(ctx, []int64{offering.ID, missing}, nil, nil)
	require.ErrorIs(t, err, timetable.ErrOfferingSourceInvalid)
	coded, ok = timetable.AsCoded(err)
	require.True(t, ok)
	assert.Equal(t, timetable.CodeOfferingSourceNotFound, coded.Code)
}
