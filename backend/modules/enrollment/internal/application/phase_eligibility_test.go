package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collectionSettingsStub answers the two collection toggles independently,
// the tenant grade-level cap, and records lock acquisitions.
type collectionSettingsStub struct {
	collectGrade bool
	collectClass bool
	// gradeMax is enrollment.grade_level_max; the zero value stands for the
	// registry default so tests that only exercise the collection toggles do
	// not have to spell it out.
	gradeMax int
	lockErr  error
	locks    *int
	reads    *int
}

func (s collectionSettingsStub) CollectGradeLevel(context.Context) (bool, error) {
	s.read()
	return s.collectGrade, nil
}

func (s collectionSettingsStub) CollectSchoolClass(context.Context) (bool, error) {
	s.read()
	return s.collectClass, nil
}

func (s collectionSettingsStub) GradeLevelMax(context.Context) (int, error) {
	s.read()
	if s.gradeMax == 0 {
		return defaultGradeLevelMax, nil
	}
	return s.gradeMax, nil
}

func (s collectionSettingsStub) LockClassCollectionPair(context.Context) error {
	if s.locks != nil {
		*s.locks++
	}
	return s.lockErr
}

func (s collectionSettingsStub) read() {
	if s.reads != nil {
		*s.reads++
	}
}

func phasesWithSettings(settings CollectionSettings) *Phases {
	return NewPhases(PhaseDependencies{Settings: settings})
}

// A class-based eligibility restriction is uncheckable when the school does
// not collect a concrete class, so every submission would be rejected with
// class_not_eligible. Create/Update must reject that config up front (#1663).
// The guard is scoped to ACTIVE phases — the same scope the settings side
// enforces from its end — so the fixtures below are active.
func TestValidateEligibleClassesCollectable(t *testing.T) {
	t.Parallel()

	eligible := &enrollment.Phase{EligibleSchoolClasses: []string{"2a"}, IsActive: true}
	empty := &enrollment.Phase{EligibleSchoolClasses: []string{}, IsActive: true}

	t.Run("collection off with eligible classes is rejected", func(t *testing.T) {
		s := phasesWithSettings(collectionSettingsStub{collectGrade: true})
		err := s.validateEligibleClassesCollectable(context.Background(), eligible)
		require.ErrorIs(t, err, enrollment.ErrInvalidPhase)
	})

	t.Run("collection on with eligible classes passes", func(t *testing.T) {
		s := phasesWithSettings(collectionSettingsStub{collectGrade: true, collectClass: true})
		require.NoError(t, s.validateEligibleClassesCollectable(context.Background(), eligible))
	})

	t.Run("empty eligible list is never gated", func(t *testing.T) {
		s := phasesWithSettings(collectionSettingsStub{collectGrade: true})
		require.NoError(t, s.validateEligibleClassesCollectable(context.Background(), empty))
	})

	t.Run("nil settings skips the guard", func(t *testing.T) {
		assert.NoError(t, phasesWithSettings(nil).validateEligibleClassesCollectable(context.Background(), eligible))
	})

	// A phase that is not active accepts no submission at all, so its
	// restriction cannot reject anything. Gating it only made a historical
	// restricted phase unwritable once the school turned class collection off —
	// a name or date correction, and even deactivating the phase, was refused
	// (#1663).
	t.Run("an inactive phase is not gated", func(t *testing.T) {
		s := phasesWithSettings(collectionSettingsStub{collectGrade: true})
		inactive := &enrollment.Phase{EligibleSchoolClasses: []string{"2a"}}
		require.NoError(t, s.validateEligibleClassesCollectable(context.Background(), inactive))
	})
}

// A grade-based eligibility restriction is uncheckable when the school does not
// collect the grade level: every child's grade is nil at submit and the gate
// rejects every submission with grade_not_eligible. Create/Update must reject
// that config up front — but, unlike the class restriction, a whole-grade phase
// must stay valid while concrete-class collection is off (#1663). Like the
// class guard, this one only applies to ACTIVE phases.
func TestValidateEligibleGradeLevelsCollectable(t *testing.T) {
	t.Parallel()

	restricted := &enrollment.Phase{EligibleGradeLevels: []int{3}, IsActive: true}
	unrestricted := &enrollment.Phase{EligibleGradeLevels: []int{}, IsActive: true}

	t.Run("grade collection off with a grade restriction is rejected", func(t *testing.T) {
		s := phasesWithSettings(collectionSettingsStub{collectClass: true})
		require.ErrorIs(t, s.validateEligibleClassesCollectable(context.Background(), restricted), enrollment.ErrInvalidPhase)
	})

	t.Run("grade collection on without concrete classes passes", func(t *testing.T) {
		s := phasesWithSettings(collectionSettingsStub{collectGrade: true})
		require.NoError(t, s.validateEligibleClassesCollectable(context.Background(), restricted),
			"a whole-grade phase must not require concrete-class collection")
	})

	t.Run("empty grade list is never gated", func(t *testing.T) {
		s := phasesWithSettings(collectionSettingsStub{})
		require.NoError(t, s.validateEligibleClassesCollectable(context.Background(), unrestricted))
	})

	t.Run("nil settings skips the guard", func(t *testing.T) {
		assert.NoError(t, phasesWithSettings(nil).validateEligibleClassesCollectable(context.Background(), restricted))
	})

	// Same carve-out as the class guard: an inactive phase takes no
	// submissions, so its grade restriction cannot reject one — and gating it
	// would leave a historical phase unwritable after the grade toggle was
	// turned off (#1663).
	t.Run("an inactive phase is not gated", func(t *testing.T) {
		s := phasesWithSettings(collectionSettingsStub{})
		inactive := &enrollment.Phase{EligibleGradeLevels: []int{3}}
		require.NoError(t, s.validateEligibleClassesCollectable(context.Background(), inactive))
	})
}

// A grade restriction above the tenant's enrollment.grade_level_max is
// unsatisfiable from both ends: the form offers grades 1..cap so the parent can
// never pick the eligible grade, and a hand-crafted submission carrying it is
// rejected by the cap before eligibility is consulted. Create/Update must
// reject that config instead of persisting an active phase no submission can
// reach (#1663).
func TestValidateEligibleGradeLevelsWithinTenantCap(t *testing.T) {
	t.Parallel()

	collecting := func(gradeMax int) *Phases {
		return phasesWithSettings(collectionSettingsStub{collectGrade: true, collectClass: true, gradeMax: gradeMax})
	}
	phaseFor := func(grades ...int) *enrollment.Phase {
		return &enrollment.Phase{EligibleGradeLevels: grades, IsActive: true}
	}

	t.Run("grade above the tenant cap is rejected", func(t *testing.T) {
		err := collecting(4).validateEligibleClassesCollectable(context.Background(), phaseFor(5))
		require.ErrorIs(t, err, enrollment.ErrInvalidPhase)
		assert.Contains(t, err.Error(), "above the tenant maximum 4")
	})

	t.Run("only the above-cap entry of a mixed list trips the guard", func(t *testing.T) {
		err := collecting(4).validateEligibleClassesCollectable(context.Background(), phaseFor(3, 7))
		require.ErrorIs(t, err, enrollment.ErrInvalidPhase)
		assert.Contains(t, err.Error(), "grade 7")
	})

	t.Run("grade at the cap passes", func(t *testing.T) {
		require.NoError(t, collecting(4).validateEligibleClassesCollectable(context.Background(), phaseFor(4)))
	})

	t.Run("a raised cap admits the higher grade", func(t *testing.T) {
		require.NoError(t, collecting(maxGradeLevel).validateEligibleClassesCollectable(context.Background(), phaseFor(9)))
	})

	t.Run("an out-of-range cap fails closed", func(t *testing.T) {
		err := collecting(maxGradeLevel+1).validateEligibleClassesCollectable(context.Background(), phaseFor(3))
		require.Error(t, err, "a corrupt cap must stop the write, not pick a substitute bound")
		assert.NotErrorIs(t, err, enrollment.ErrInvalidPhase, "a corrupt setting is operational, not a client error")
		assert.Equal(t, "resolve enrollment.grade_level_max: value 14 is outside 1..13", err.Error())
	})

	t.Run("an unrestricted phase never reads the cap", func(t *testing.T) {
		require.NoError(t, collecting(maxGradeLevel+1).validateEligibleClassesCollectable(context.Background(), phaseFor()))
	})
}

// The collectability guards and the settings-side collection guard protect two
// halves of one invariant: an active phase restricted to classes or grades must
// never coexist with the collection toggle that feeds the restriction being
// off. Each side validates on a read and then writes, so without a shared lock
// two concurrent writers both pass on a stale read and commit the broken pair.
// These tests pin that the phase side actually takes that lock, and that it
// refuses to proceed when the lock cannot be taken (#1663).
func TestEligibilityGuardsTakeTheSharedLock(t *testing.T) {
	t.Parallel()

	guards := map[string]func(context.Context, CollectionSettings) error{
		"class": func(ctx context.Context, s CollectionSettings) error {
			return ensureEligibleClassesCollectable(ctx, s, []string{"3a"})
		},
		"grade": func(ctx context.Context, s CollectionSettings) error {
			return ensureEligibleGradeLevelsCollectable(ctx, s, []int{3})
		},
	}
	for name, guard := range guards {
		t.Run(name+" acquires the lock before reading the toggles", func(t *testing.T) {
			locks := 0
			require.NoError(t, guard(context.Background(), collectionSettingsStub{collectGrade: true, collectClass: true, locks: &locks}))
			assert.Equal(t, 1, locks, "the guard must serialize against the settings side")
		})

		t.Run(name+" aborts on a failed lock instead of validating on an unlocked read", func(t *testing.T) {
			lockErr := errors.New("lock unavailable")
			reads := 0
			err := guard(context.Background(), collectionSettingsStub{collectGrade: true, collectClass: true, lockErr: lockErr, reads: &reads})
			require.ErrorIs(t, err, lockErr)
			assert.NotErrorIs(t, err, enrollment.ErrInvalidPhase, "an unavailable lock is operational, not a rejection of the phase")
			assert.Zero(t, reads)
		})
	}

	t.Run("no restriction needs no lock", func(t *testing.T) {
		locks := 0
		settings := collectionSettingsStub{locks: &locks}
		require.NoError(t, ensureEligibleClassesCollectable(context.Background(), settings, nil))
		require.NoError(t, ensureEligibleGradeLevelsCollectable(context.Background(), settings, nil))
		assert.Zero(t, locks, "an unrestricted phase touches no shared invariant")
	})
}

// The rollover uses the same guards regardless of whether the successor is
// active yet.
func TestCheckEligibilityCollectableIgnoresTheActiveFlag(t *testing.T) {
	t.Parallel()
	s := phasesWithSettings(collectionSettingsStub{collectGrade: true})
	inactive := &enrollment.Phase{EligibleSchoolClasses: []string{"2a"}}
	require.ErrorIs(t, s.CheckEligibilityCollectable(context.Background(), inactive), enrollment.ErrInvalidPhase)
	assert.NoError(t, phasesWithSettings(nil).CheckEligibilityCollectable(context.Background(), inactive))
}

func singleModeSchemaFields() []enrollment.FormField {
	return []enrollment.FormField{{
		Key: "heimwege", Label: "Erlaubte Heimwege",
		Type:        enrollment.FormFieldWeekdayMultiMode,
		AppliesToCh: true, Target: enrollment.TargetStudentAllowedDepartureModes,
		SingleModeGrades: []int{1},
	}}
}

// The Heimweg-Beschränkung publish guard (#2381): a schema whose
// single_mode_grades rule keys on the target grade level must not be
// publishable while the grade-level collection setting is off — the rule
// would silently never restrict anyone.
func TestEnsureSingleModeGradesCollectable(t *testing.T) {
	t.Parallel()

	t.Run("rejects with collection off", func(t *testing.T) {
		s := NewFormSchemas(FormSchemaDependencies{Settings: collectionSettingsStub{}})
		err := s.ensureSingleModeGradesCollectable(context.Background(), singleModeSchemaFields())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "single_mode_grades")
		assert.Contains(t, err.Error(), "Klassenstufen-Abfrage")
	})

	t.Run("passes with collection on and takes the shared lock", func(t *testing.T) {
		locks := 0
		s := NewFormSchemas(FormSchemaDependencies{Settings: collectionSettingsStub{collectGrade: true, locks: &locks}})
		require.NoError(t, s.ensureSingleModeGradesCollectable(context.Background(), singleModeSchemaFields()))
		assert.Equal(t, 1, locks)
	})

	t.Run("skips without a rule", func(t *testing.T) {
		reads := 0
		s := NewFormSchemas(FormSchemaDependencies{Settings: collectionSettingsStub{reads: &reads}})
		fields := singleModeSchemaFields()
		fields[0].SingleModeGrades = nil
		require.NoError(t, s.ensureSingleModeGradesCollectable(context.Background(), fields))
		assert.Zero(t, reads, "settings must not be consulted without a rule")
	})

	t.Run("nil settings skips", func(t *testing.T) {
		assert.NoError(t, NewFormSchemas(FormSchemaDependencies{}).ensureSingleModeGradesCollectable(context.Background(), singleModeSchemaFields()))
	})
}
