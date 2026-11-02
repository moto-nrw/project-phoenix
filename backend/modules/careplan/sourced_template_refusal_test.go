package careplan_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// Each problem of a refused Regeltermin answers with its own code, and the
// refusal names the edited offering and the Regeltermin (#5313).
func TestCareOfferingSourceConflict(t *testing.T) {
	t.Parallel()
	inner := errors.New("refused")
	for problem, code := range map[careplan.SourcedTemplateProblem]string{
		careplan.SourcedTemplateNoActiveSource: careplan.CodeCareOfferingTemplateNoActiveSource,
		careplan.SourcedTemplateMixedPhases:    careplan.CodeCareOfferingTemplateMixedPhases,
		careplan.SourcedTemplateOutsidePeriod:  careplan.CodeCareOfferingTemplateOutsidePeriod,
		careplan.SourcedTemplateSourceInvalid:  careplan.CodeCareOfferingTemplateSourceInvalid,
	} {
		refusal := careplan.NewSourcedTemplateRefusal("Mittagessen Mo", problem, inner)
		err := careplan.CareOfferingSourceConflict("Spätbetreuung",
			fmt.Errorf("%w: %w", careplan.ErrCareOfferingConfigInvalid, refusal))
		var conflict *careplan.CareOfferingSourceConflictError
		require.ErrorAs(t, err, &conflict)
		assert.Equal(t, code, conflict.ErrorCode())
		assert.Equal(t, careplan.CareOfferingSourceConflictValues{Offering: "Spätbetreuung", Template: "Mittagessen Mo"}, conflict.ErrorDetails())
		assert.ErrorIs(t, err, careplan.ErrCareOfferingSourceConflict)
		assert.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
		assert.ErrorIs(t, err, inner, "the refusal chain stays")
	}
	assert.NoError(t, careplan.CareOfferingSourceConflict("Spätbetreuung", inner), "without a Regeltermin refusal there is no conflict")
}
