package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/workflows/traegercatalog"
	"github.com/moto-nrw/project-phoenix/workflows/traegercatalog/ports"
)

// schoolRun is one school of a rollout.
type schoolRun struct {
	target target
	result traegercatalog.SchoolResult
	copies map[int64]int64
	// offeringCodes names an offering whose tariff or template a school
	// refused.
	offeringCodes map[int64]string
}

// Rollout releases the draft under the lock of the Träger phase and brings
// every target school to the release in a tenant transaction of its own
// (ADR 0195 section 5): release, then each school with its tariffs and
// contract templates, then the outcomes. No transaction carries both.
func (c *Catalog) Rollout(ctx context.Context, actor traegercatalog.Actor, traegerPhaseID int64, token string) (traegercatalog.Rollout, error) {
	print, err := decodeFingerprint(token)
	if err != nil {
		return traegercatalog.Rollout{}, err
	}
	snap, next, err := c.release(ctx, actor, traegerPhaseID, print)
	if err != nil {
		return traegercatalog.Rollout{}, err
	}
	rollout := traegercatalog.Rollout{TraegerPhaseID: traegerPhaseID, Version: next.phaseVersion, Schools: []traegercatalog.SchoolResult{}}
	runs := make([]schoolRun, 0, len(snap.targets()))
	for _, t := range snap.targets() {
		run, err := c.rollOutSchool(ctx, actor, snap, next, t, print.Schools[t.schoolID])
		if err != nil {
			return traegercatalog.Rollout{}, err
		}
		runs = append(runs, run)
		rollout.Schools = append(rollout.Schools, run.result)
	}
	return rollout, c.record(ctx, actor, snap, next, runs)
}

// release checks the draft against the preview under the lock of the Träger
// phase and writes the versions. The lock ends with this transaction,
// before any school transaction begins.
func (c *Catalog) release(ctx context.Context, actor traegercatalog.Actor, id int64, print fingerprint) (snapshot, release, error) {
	schools, err := c.deps.Schools.ListTraegerSchoolIDs(ctx, actor.OrganizationID)
	if err != nil {
		return snapshot{}, release{}, err
	}
	snap := snapshot{schools: schools}
	var next release
	err = c.deps.Runtime.WithinTraeger(ctx, actor.OrganizationID, func(ctx context.Context) error {
		var err error
		if snap.phase, err = c.deps.Phases.LockTraegerPhase(ctx, actor.OrganizationID, id); err != nil {
			return err
		}
		if snap.offerings, err = c.deps.Offerings.ListTraegerOfferings(ctx, actor.OrganizationID, id); err != nil {
			return err
		}
		if !c.samePlan(snap, print) {
			return traegercatalog.ErrPreviewStale
		}
		next, err = c.writeVersions(ctx, actor, snap)
		return err
	})
	return snap, next, err
}

func (c *Catalog) samePlan(snap snapshot, print fingerprint) bool {
	targets := snap.targets()
	if c.draftHash(snap, targets) != print.Draft || len(targets) != len(print.Schools) {
		return false
	}
	for _, t := range targets {
		if _, ok := print.Schools[t.schoolID]; !ok {
			return false
		}
	}
	return true
}

func (c *Catalog) writeVersions(ctx context.Context, actor traegercatalog.Actor, snap snapshot) (release, error) {
	version, err := c.deps.Phases.ReleaseTraegerPhase(ctx, actor.OrganizationID, snap.phase.ID, actor.AccountID)
	if err != nil {
		return release{}, err
	}
	released, err := c.deps.Offerings.ReleaseTraegerOfferings(ctx, actor.OrganizationID, snap.phase.ID, actor.AccountID)
	if err != nil {
		return release{}, err
	}
	next := release{phaseVersion: version, phase: snap.phase.Draft, offerings: map[int64]offeringRelease{}}
	for _, offering := range released {
		if offering.ReleasedVersion != nil && offering.Released != nil {
			next.offerings[offering.ID] = offeringRelease{version: *offering.ReleasedVersion, draft: *offering.Released}
		}
	}
	return next, nil
}

// rollOutSchool writes one school in its own tenant transaction: under the
// locks of its copies it compares their state with the preview and stops
// as stale when it moved; otherwise it writes phase and offerings together
// or nothing. Its tariffs and templates follow after the commit.
func (c *Catalog) rollOutSchool(ctx context.Context, actor traegercatalog.Actor, snap snapshot, next release, t target, previewed string) (schoolRun, error) {
	run := schoolRun{target: t, result: traegercatalog.SchoolResult{SchoolID: t.schoolID}, offeringCodes: map[int64]string{}}
	err := c.deps.Runtime.WithinSchool(ctx, t.schoolID, func(ctx context.Context) error {
		state, err := c.schoolState(ctx, snap, true)
		if err != nil {
			return err
		}
		if state != previewed {
			return errStale
		}
		_, run.copies, err = c.applySchool(ctx, snap, next, t.action)
		return err
	})
	switch code, refused := rejectionCode(err); {
	case err == nil && t.action == traegercatalog.ActionWithdraw:
		run.result.Outcome = enrollment.TraegerOutcomeWithdrawn
	case err == nil:
		run.result.Outcome = enrollment.TraegerOutcomeApplied
		return run, c.writeBilling(ctx, actor, snap, next, &run)
	case errors.Is(err, errStale):
		run.result.Outcome = enrollment.TraegerOutcomeStale
	case refused:
		run.result.Outcome, run.result.ErrorCode = enrollment.TraegerOutcomeRejected, code
	default:
		return run, err
	}
	return run, nil
}

// writeBilling writes the school's tariffs in the Träger transaction and
// then its contract templates, which Care Contract writes in its own
// transactions. A refusal marks only that offering at that school.
func (c *Catalog) writeBilling(ctx context.Context, actor traegercatalog.Actor, snap snapshot, next release, run *schoolRun) error {
	for _, offering := range snap.offerings {
		copyID := run.copies[offering.ID]
		if copyID == 0 {
			continue
		}
		plan := billingFor(offering, next.offerings[offering.ID], run.target.schoolID, next.phase, c.deps.Today())
		err := c.writeOfferingBilling(ctx, actor, run.target.schoolID, copyID, next.offerings[offering.ID].draft, plan)
		if code, refused := rejectionCode(err); refused {
			run.offeringCodes[offering.ID] = code
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (c *Catalog) writeOfferingBilling(ctx context.Context, actor traegercatalog.Actor, schoolID, copyID int64, draft careplan.TraegerOfferingDraft, plan billing) error {
	if plan.pricesFrom != nil {
		err := c.deps.Runtime.WithinTraeger(ctx, actor.OrganizationID, func(ctx context.Context) error {
			return c.deps.Tariffs(ctx, actor.OrganizationID, actor.AccountID, ports.ModulePrices{
				SchoolID: schoolID, CareOfferingID: copyID, ModuleName: draft.Fields.Name, ValidFrom: *plan.pricesFrom, Prices: draft.Prices,
			})
		})
		if err != nil {
			return err
		}
	}
	if plan.template {
		return c.deps.ContractTemplates(ctx, actor.OrganizationID, actor.AccountID, schoolID, copyID, *draft.ContractTemplate)
	}
	return nil
}

// record writes the outcome of every school in one Träger transaction,
// after all school transactions.
func (c *Catalog) record(ctx context.Context, actor traegercatalog.Actor, snap snapshot, next release, runs []schoolRun) error {
	if len(runs) == 0 {
		return nil
	}
	phaseOutcomes := make([]enrollment.TraegerCopyOutcome, 0, len(runs))
	offeringOutcomes := map[int64][]careplan.TraegerOfferingOutcome{}
	for _, run := range runs {
		phaseOutcomes = append(phaseOutcomes, phaseOutcome(run, next.phaseVersion))
		for _, offering := range snap.offerings {
			if outcome, ok := offeringOutcome(run, offering, next); ok {
				offeringOutcomes[offering.ID] = append(offeringOutcomes[offering.ID], outcome)
			}
		}
	}
	return c.deps.Runtime.WithinTraeger(ctx, actor.OrganizationID, func(ctx context.Context) error {
		if err := c.deps.Phases.RecordTraegerPhaseOutcomes(ctx, actor.OrganizationID, snap.phase.ID, phaseOutcomes); err != nil {
			return err
		}
		for _, offering := range snap.offerings {
			if err := c.deps.Offerings.RecordTraegerOfferingOutcomes(ctx, actor.OrganizationID, offering.ID, offeringOutcomes[offering.ID]); err != nil {
				return err
			}
		}
		return nil
	})
}

func phaseOutcome(run schoolRun, version int) enrollment.TraegerCopyOutcome {
	outcome := enrollment.TraegerCopyOutcome{SchoolID: run.target.schoolID, Outcome: run.result.Outcome, ErrorCode: run.result.ErrorCode}
	if run.result.Outcome == enrollment.TraegerOutcomeApplied {
		outcome.AppliedVersion = &version
	}
	return outcome
}

func offeringOutcome(run schoolRun, offering careplan.TraegerOffering, next release) (careplan.TraegerOfferingOutcome, bool) {
	outcome := careplan.TraegerOfferingOutcome{SchoolID: run.target.schoolID, Outcome: run.result.Outcome, ErrorCode: run.result.ErrorCode}
	switch {
	case run.result.Outcome == enrollment.TraegerOutcomeApplied && offering.WithdrawnAt != nil:
		outcome.Outcome = enrollment.TraegerOutcomeWithdrawn
	case run.result.Outcome == enrollment.TraegerOutcomeApplied && run.offeringCodes[offering.ID] != "":
		outcome.Outcome, outcome.ErrorCode = enrollment.TraegerOutcomeRejected, run.offeringCodes[offering.ID]
	case run.result.Outcome == enrollment.TraegerOutcomeApplied:
		version := next.offerings[offering.ID].version
		outcome.AppliedVersion = &version
	}
	return outcome, recordedVersion(offering, run.target.schoolID) != nil || run.result.Outcome != enrollment.TraegerOutcomeWithdrawn
}
