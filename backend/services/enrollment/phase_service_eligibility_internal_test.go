package enrollment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// A class-based eligibility restriction is uncheckable when the school does
// not collect a concrete class, so every submission would be rejected with
// class_not_eligible. Create/Update must reject that config up front (#1663).
func TestValidateEligibleClassesCollectable(t *testing.T) {
	eligible := &enrollmentModels.Phase{EligibleSchoolClasses: []string{"2a"}}
	empty := &enrollmentModels.Phase{EligibleSchoolClasses: []string{}}

	t.Run("collection off with eligible classes is rejected", func(t *testing.T) {
		s := &phaseService{settings: schoolClassSettingsStub{collect: false}}
		err := s.validateEligibleClassesCollectable(context.Background(), eligible)
		require.ErrorIs(t, err, ErrInvalidPhase)
	})

	t.Run("collection on with eligible classes passes", func(t *testing.T) {
		s := &phaseService{settings: schoolClassSettingsStub{collect: true}}
		require.NoError(t, s.validateEligibleClassesCollectable(context.Background(), eligible))
	})

	t.Run("empty eligible list is never gated", func(t *testing.T) {
		s := &phaseService{settings: schoolClassSettingsStub{collect: false}}
		require.NoError(t, s.validateEligibleClassesCollectable(context.Background(), empty))
	})

	t.Run("nil settings skips the guard", func(t *testing.T) {
		s := &phaseService{}
		assert.NoError(t, s.validateEligibleClassesCollectable(context.Background(), eligible))
	})
}
