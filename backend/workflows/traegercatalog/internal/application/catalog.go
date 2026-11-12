// Package application holds the rollout of a Träger phase into its schools
// (ADR 0195 section 5): preview, release, apply per school, tariff and
// contract template, outcome.
package application

import (
	"context"
	"errors"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/traegercatalog"
	"github.com/moto-nrw/project-phoenix/workflows/traegercatalog/ports"
)

// errPreviewDiscard ends a preview's school transaction without writing:
// the preview runs the same commands as the rollout and rolls them back.
var errPreviewDiscard = errors.New("träger catalog: preview discards its school transaction")

// errStale ends a rollout's school transaction whose state moved since the
// preview.
var errStale = errors.New("träger catalog: the school changed since the preview")

// Catalog is the rollout workflow.
type Catalog struct {
	deps ports.Dependencies
}

var _ traegercatalog.Catalog = (*Catalog)(nil)

// New builds the rollout; every port is required.
func New(deps ports.Dependencies) (*Catalog, error) {
	if deps.Schools == nil || deps.Phases == nil || deps.Offerings == nil || deps.SchoolPhases == nil ||
		deps.SchoolOfferings == nil || deps.Tariffs == nil || deps.ContractTemplates == nil || deps.Runtime == nil || deps.Today == nil ||
		deps.Fingerprint == nil {
		return nil, errors.New("träger catalog: every port is required")
	}
	return &Catalog{deps: deps}, nil
}

// snapshot is a Träger phase with its offerings as one Träger transaction
// read them, and the schools of the Träger.
type snapshot struct {
	month     calendar.Date
	phase     enrollment.TraegerPhase
	offerings []careplan.TraegerOffering
	schools   []int64
}

// target is one school of a rollout.
type target struct {
	schoolID int64
	action   string
}

// release is the version a school gets: the phase and offering fields and
// their versions.
type release struct {
	month        calendar.Date
	phaseVersion int
	phase        enrollment.TraegerPhaseFields
	offerings    map[int64]offeringRelease
}

type offeringRelease struct {
	version int
	draft   careplan.TraegerOfferingDraft
}

func (c *Catalog) read(ctx context.Context, actor traegercatalog.Actor, id int64, lock bool) (snapshot, error) {
	schools, err := c.deps.Schools.ListTraegerSchoolIDs(ctx, actor.OrganizationID)
	if err != nil {
		return snapshot{}, err
	}
	snap := snapshot{schools: schools, month: monthStart(c.deps.Today())}
	err = c.deps.Runtime.WithinTraeger(ctx, actor.OrganizationID, func(ctx context.Context) error {
		var err error
		if lock {
			snap.phase, err = c.deps.Phases.LockTraegerPhase(ctx, actor.OrganizationID, id)
		} else {
			snap.phase, err = c.deps.Phases.TraegerPhase(ctx, actor.OrganizationID, id)
		}
		if err != nil {
			return err
		}
		snap.offerings, err = c.deps.Offerings.ListTraegerOfferings(ctx, actor.OrganizationID, id)
		return err
	})
	return snap, err
}

// targets are the schools a rollout writes in, in ID order: every selected
// school of the Träger takes the version, and every school that reports a
// copy but left the selection, or a withdrawn phase, withdraws it. A school
// that no longer belongs to the Träger is skipped (ADR 0195 section 2).
func (s snapshot) targets() []target {
	own := make(map[int64]bool, len(s.schools))
	for _, id := range s.schools {
		own[id] = true
	}
	selected := s.selected(own)
	var targets []target
	for id, take := range selected {
		if take {
			targets = append(targets, target{schoolID: id, action: traegercatalog.ActionUpdate})
		}
	}
	for _, school := range s.phase.Schools {
		copied := (school.AppliedVersion != nil || school.Outcome != "") && school.Outcome != enrollment.TraegerOutcomeWithdrawn
		if copied && own[school.SchoolID] && !selected[school.SchoolID] {
			targets = append(targets, target{schoolID: school.SchoolID, action: traegercatalog.ActionWithdraw})
		}
	}
	slices.SortFunc(targets, func(a, b target) int { return int(a.schoolID - b.schoolID) })
	return targets
}

// selected are the schools of the Träger the phase applies to; a withdrawn
// phase applies to none.
func (s snapshot) selected(own map[int64]bool) map[int64]bool {
	selected := map[int64]bool{}
	if s.phase.WithdrawnAt != nil {
		return selected
	}
	for _, school := range s.phase.Schools {
		selected[school.SchoolID] = school.Selected && own[school.SchoolID]
	}
	if s.phase.AllSchools {
		for _, id := range s.schools {
			selected[id] = true
		}
	}
	return selected
}

// prospective is the release the draft would become: the released version
// where the draft did not change, else the next one.
func (s snapshot) prospective() release {
	next := release{month: s.month, phase: s.phase.Draft, phaseVersion: nextVersion(s.phase.ReleasedVersion, s.phase.Released != nil && sameJSON(*s.phase.Released, s.phase.Draft)), offerings: map[int64]offeringRelease{}}
	for _, offering := range s.offerings {
		unchanged := offering.Released != nil && sameJSON(*offering.Released, offering.Draft)
		next.offerings[offering.ID] = offeringRelease{version: nextVersion(offering.ReleasedVersion, unchanged), draft: offering.Draft}
	}
	return next
}

func nextVersion(released *int, unchanged bool) int {
	switch {
	case released == nil:
		return 1
	case unchanged:
		return *released
	default:
		return *released + 1
	}
}
