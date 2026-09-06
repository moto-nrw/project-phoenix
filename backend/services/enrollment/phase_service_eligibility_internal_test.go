package enrollment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// A class-based eligibility restriction is uncheckable when the school does
// not collect a concrete class, so every submission would be rejected with
// class_not_eligible. Create/Update must reject that config up front (#1663).
// The guard is scoped to ACTIVE phases — the same scope the settings side
// enforces from its end — so the fixtures below are active.
func TestValidateEligibleClassesCollectable(t *testing.T) {
	t.Parallel()

	eligible := &enrollmentModels.Phase{EligibleSchoolClasses: []string{"2a"}, IsActive: true}
	empty := &enrollmentModels.Phase{EligibleSchoolClasses: []string{}, IsActive: true}

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

	// A phase that is not active accepts no submission at all, so its
	// restriction cannot reject anything. Gating it only made a historical
	// restricted phase unwritable once the school turned class collection off —
	// a name or date correction, and even deactivating the phase, was refused
	// (#1663).
	t.Run("an inactive phase is not gated", func(t *testing.T) {
		s := &phaseService{settings: schoolClassSettingsStub{collect: false}}
		inactive := &enrollmentModels.Phase{EligibleSchoolClasses: []string{"2a"}}
		require.NoError(t, s.validateEligibleClassesCollectable(context.Background(), inactive))
	})
}
