package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// CheckEligibilityCollectable rejects a grade or class eligibility
// restriction the school cannot collect, whether or not the phase is active.
// The rollover uses it before it copies a source's restriction into an
// active successor; Create/Update apply the same guards to active phases.
// Skipped when no settings are bound.
func (s *Phases) CheckEligibilityCollectable(ctx context.Context, phase *enrollment.Phase) error {
	if s.deps.Settings == nil || phase == nil {
		return nil
	}
	if err := ensureEligibleGradeLevelsCollectable(ctx, s.deps.Settings, phase.EligibleGradeLevels); err != nil {
		return err
	}
	return ensureEligibleClassesCollectable(ctx, s.deps.Settings, phase.EligibleSchoolClasses)
}

// validateEligibleClassesCollectable applies the two collectability guards
// on Create/Update.
//
// Only ACTIVE phases are guarded. The invariant these guards protect is
// "no ACTIVE restricted phase while the matching collection setting is off" —
// exactly the scope the settings side enforces from its end
// (HasActiveClassRestrictedPhase / HasActiveGradeRestrictedPhase only
// look at is_active phases). Applying them to an inactive phase made a
// historical restricted phase unwritable once collection was disabled: a name
// or date correction, or even deactivating the phase, hit the guard and could
// not be saved at all, while clearing the restriction to get past it would
// destroy the record of what that phase was restricted to. Reactivating such a
// phase still goes through Update with is_active=true and is still refused
// (#1663).
func (s *Phases) validateEligibleClassesCollectable(ctx context.Context, phase *enrollment.Phase) error {
	if !phase.IsActive {
		return nil
	}
	return s.CheckEligibilityCollectable(ctx, phase)
}

// ensureEligibleClassesCollectable rejects a class-based eligibility
// restriction the school cannot actually collect. eligible_school_classes
// is checked at submit against each child's declared concrete class, but
// the form only collects that class when BOTH collect_grade_level and
// collect_school_class are on (a class without its grade is ambiguous).
// With collection off, the submit path forces every child's class to nil,
// so a non-empty eligibility list rejects every submission with
// class_not_eligible. Reject the config here so the admin enables class
// collection (or clears the list) first (#1663).
func ensureEligibleClassesCollectable(ctx context.Context, settings CollectionSettings, eligible []string) error {
	if !enrollment.RestrictsEligibleClasses(eligible) {
		return nil
	}
	// Serialize against a concurrent settings write disabling concrete-class
	// collection: the settings side takes the same per-tenant lock, so the
	// two invariant sides can't both pass on a stale read and commit an
	// active restricted phase with collection off (#1663).
	if err := settings.LockClassCollectionPair(ctx); err != nil {
		return fmt.Errorf("lock class-collection pair: %w", err)
	}
	collectGrade, err := settings.CollectGradeLevel(ctx)
	if err != nil {
		return fmt.Errorf("resolve enrollment.collect_grade_level: %w", err)
	}
	collectClass, err := settings.CollectSchoolClass(ctx)
	if err != nil {
		return fmt.Errorf("resolve enrollment.collect_school_class: %w", err)
	}
	if !collectGrade || !collectClass {
		return fmt.Errorf("%w: eligible_school_classes requires the concrete-class collection settings (Klassen-Abfrage) to be active", enrollment.ErrInvalidPhase)
	}
	return nil
}

// ensureEligibleGradeLevelsCollectable is the grade-level counterpart of
// ensureEligibleClassesCollectable. A grade restriction is checked at submit
// against each child's declared grade level, which the form collects only
// while collect_grade_level is on — with it off, the submit path nils every
// child's grade and the restriction rejects every submission with
// grade_not_eligible. Note the weaker requirement than the class guard: a
// whole-grade phase needs the grade toggle ALONE, which is exactly why
// enumerating concrete classes is not a substitute for it (#1663).
func ensureEligibleGradeLevelsCollectable(ctx context.Context, settings CollectionSettings, eligible []int) error {
	if len(eligible) == 0 {
		return nil
	}
	// Same per-tenant lock as the class guard: the settings side takes it
	// before deciding whether collect_grade_level may be turned off, so the two
	// invariant sides cannot both pass on a stale read (#1663).
	if err := settings.LockClassCollectionPair(ctx); err != nil {
		return fmt.Errorf("lock class-collection pair: %w", err)
	}
	collectGrade, err := settings.CollectGradeLevel(ctx)
	if err != nil {
		return fmt.Errorf("resolve enrollment.collect_grade_level: %w", err)
	}
	if !collectGrade {
		return fmt.Errorf("%w: eligible_grade_levels requires the grade-level collection setting (Klassenstufen-Abfrage) to be active", enrollment.ErrInvalidPhase)
	}
	return ensureEligibleGradeLevelsWithinTenantCap(ctx, settings, eligible)
}

// ensureEligibleGradeLevelsWithinTenantCap rejects a grade restriction the
// tenant's own form can never satisfy. The public form offers grades
// 1..enrollment.grade_level_max and the submit path re-checks the same cap, so
// a phase restricted to a grade ABOVE it is unsatisfiable from both ends: the
// select never offers the grade, and a hand-crafted submission carrying it is
// rejected by the cap before eligibility is even consulted. Without this check
// an admin can save eligible_grade_levels [5] under a cap of 4 and end up with
// an active phase that accepts no submission at all (#1663).
//
// Runs under the lock the caller already holds, which the settings side takes
// too — so lowering the cap and adding an above-cap restriction cannot both
// pass on a stale read. Values below the minimum grade are already rejected by
// Phase.Validate; the cap is the tenant-specific half it cannot know about.
func ensureEligibleGradeLevelsWithinTenantCap(ctx context.Context, settings CollectionSettings, eligible []int) error {
	gradeMax, err := settings.GradeLevelMax(ctx)
	if err != nil {
		return fmt.Errorf("resolve enrollment.grade_level_max: %w", err)
	}
	// A corrupt or out-of-range cap must stop the write rather than silently
	// pick a substitute bound — the same fail-closed reasoning as the submit
	// and rollover paths.
	if gradeMax < minGradeLevel || gradeMax > maxGradeLevel {
		return fmt.Errorf(
			"resolve enrollment.grade_level_max: value %d is outside %d..%d",
			gradeMax,
			minGradeLevel,
			maxGradeLevel,
		)
	}
	for _, level := range eligible {
		if level > gradeMax {
			return fmt.Errorf(
				"%w: eligible_grade_levels contains grade %d above the tenant maximum %d (Höchste Klassenstufe im Formular); the form never offers that grade, so every submission would be rejected",
				enrollment.ErrInvalidPhase,
				level,
				gradeMax,
			)
		}
	}
	return nil
}
