package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/workflows/traegercatalog"
	"github.com/moto-nrw/project-phoenix/workflows/traegercatalog/ports"
)

// schoolRun is one school of a rollout.
type schoolRun struct {
	target      target
	result      traegercatalog.SchoolResult
	copies      map[int64]int64
	leftTraeger bool
	prices      []ports.ModulePrices
	templates   func(context.Context) error
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
	var result traegercatalog.Rollout
	err = c.deps.Runtime.WithinExclusive(ctx, fmt.Sprintf("traeger-catalog:%d:%d", actor.OrganizationID, traegerPhaseID), func(ctx context.Context) error {
		snap, next, err := c.release(ctx, actor, traegerPhaseID, print)
		if err != nil {
			return err
		}
		result = traegercatalog.Rollout{TraegerPhaseID: traegerPhaseID, Version: next.phaseVersion, Schools: []traegercatalog.SchoolResult{}}
		runs := make([]schoolRun, 0, len(snap.targets()))
		for _, target := range snap.targets() {
			run, err := c.rollOutSchool(ctx, actor, snap, next, target, print.Schools[target.schoolID])
			if err != nil {
				return err
			}
			runs = append(runs, run)
			result.Schools = append(result.Schools, run.result)
		}
		return c.record(ctx, actor, snap, next, runs)
	})
	return result, err
}

// release checks the draft against the preview under the lock of the Träger
// phase and writes the versions. The lock ends with this transaction,
// before any school transaction begins.
func (c *Catalog) release(ctx context.Context, actor traegercatalog.Actor, id int64, print fingerprint) (snapshot, release, error) {
	schools, err := c.deps.Schools.ListTraegerSchoolIDs(ctx, actor.OrganizationID)
	if err != nil {
		return snapshot{}, release{}, err
	}
	snap := snapshot{schools: schools, month: monthStart(c.deps.Today())}
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
		if err != nil {
			return err
		}
		// Retain every attempted school before any copy can commit. An interrupted
		// rollout remains withdrawable even when billing or final recording fails.
		pending := make([]enrollment.TraegerCopyOutcome, 0, len(snap.targets()))
		for _, t := range snap.targets() {
			pending = append(pending, enrollment.TraegerCopyOutcome{SchoolID: t.schoolID, Outcome: enrollment.TraegerOutcomeStale})
		}
		return c.deps.Phases.RecordTraegerPhaseOutcomes(ctx, actor.OrganizationID, snap.phase.ID, pending)
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
	// ReleaseTraegerOfferings rereads the drafts. Confirm that this exact
	// snapshot was previewed; a concurrent edit rolls the release back.
	validated := snap
	validated.offerings = released
	if c.draftHash(validated, snap.targets()) != c.draftHash(snap, snap.targets()) {
		return release{}, traegercatalog.ErrPreviewStale
	}
	next := release{month: snap.month, phaseVersion: version, phase: snap.phase.Draft, offerings: map[int64]offeringRelease{}}
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
		if err := c.lockSchool(ctx, actor.OrganizationID, t.schoolID); err != nil {
			run.leftTraeger = errors.Is(err, errStale)
			return err
		}
		state, err := c.schoolState(ctx, snap, true)
		if err != nil {
			return err
		}
		if state != previewed {
			return errStale
		}
		_, run.copies, err = c.applySchool(ctx, snap, next, t.action)
		if err != nil || t.action == traegercatalog.ActionWithdraw {
			return err
		}
		run.prices, run.templates, err = c.prepareBilling(ctx, actor, snap, next, run)
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

// prepareBilling validates all templates before the school commits. The
// returned command contains only owner-validated inputs for the financial unit.
func (c *Catalog) prepareBilling(ctx context.Context, actor traegercatalog.Actor, snap snapshot, next release, run schoolRun) ([]ports.ModulePrices, func(context.Context) error, error) {
	prices := []ports.ModulePrices{}
	templates := []ports.ModuleTemplate{}
	for _, offering := range snap.offerings {
		copyID := run.copies[offering.ID]
		if copyID == 0 {
			continue
		}
		released := next.offerings[offering.ID]
		plan := billingFor(offering, released, run.target.schoolID, next.phase, next.month)
		if plan.pricesFrom != nil {
			prices = append(prices, ports.ModulePrices{SchoolID: run.target.schoolID, CareOfferingID: copyID,
				ModuleName: released.draft.Fields.Name, ValidFrom: *plan.pricesFrom, Prices: released.draft.Prices})
		}
		if plan.template {
			templates = append(templates, ports.ModuleTemplate{CareOfferingID: copyID, Template: *released.draft.ContractTemplate})
		}
	}
	prepared, err := c.deps.ContractTemplates(ctx, actor.OrganizationID, actor.AccountID, run.target.schoolID, templates)
	return prices, prepared, err
}

// writeBilling commits every tariff and template of a school together. A
// refusal rolls all financial writes back and preserves its applied version.
func (c *Catalog) writeBilling(ctx context.Context, actor traegercatalog.Actor, snap snapshot, next release, run *schoolRun) error {
	err := c.deps.Runtime.WithinTraeger(ctx, actor.OrganizationID, func(ctx context.Context) error {
		if err := c.deps.Tariffs(ctx, actor.OrganizationID, actor.AccountID, run.prices); err != nil {
			return err
		}
		if err := run.templates(ctx); err != nil {
			return err
		}
		// Billing and its comparison baseline must commit together. A later
		// school or final-outcome failure must not hide committed terms.
		return c.recordOutcomes(ctx, actor, snap, next, []schoolRun{*run})
	})
	if code, refused := rejectionCode(err); refused {
		run.result.Outcome, run.result.ErrorCode = enrollment.TraegerOutcomeRejected, code
		for id := range run.copies {
			run.offeringCodes[id] = code
		}
		return nil
	}
	return err
}

// record writes the outcome of every school in one Träger transaction,
// after all school transactions.
func (c *Catalog) record(ctx context.Context, actor traegercatalog.Actor, snap snapshot, next release, runs []schoolRun) error {
	if len(runs) == 0 {
		return nil
	}
	return c.deps.Runtime.WithinTraeger(ctx, actor.OrganizationID, func(ctx context.Context) error {
		return c.recordOutcomes(ctx, actor, snap, next, runs)
	})
}

func (c *Catalog) recordOutcomes(ctx context.Context, actor traegercatalog.Actor, snap snapshot, next release, runs []schoolRun) error {
	phaseOutcomes := make([]enrollment.TraegerCopyOutcome, 0, len(runs))
	offeringOutcomes := map[int64][]careplan.TraegerOfferingOutcome{}
	for _, run := range runs {
		if run.leftTraeger {
			continue
		}
		phaseOutcomes = append(phaseOutcomes, phaseOutcome(run, next.phaseVersion))
		for _, offering := range snap.offerings {
			if outcome, ok := offeringOutcome(run, offering, next); ok {
				offeringOutcomes[offering.ID] = append(offeringOutcomes[offering.ID], outcome)
			}
		}
	}
	if err := c.deps.Phases.RecordTraegerPhaseOutcomes(ctx, actor.OrganizationID, snap.phase.ID, phaseOutcomes); err != nil {
		return err
	}
	return c.deps.Offerings.RecordTraegerOfferingOutcomes(ctx, actor.OrganizationID, offeringOutcomes)
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
	case run.offeringCodes[offering.ID] != "":
		outcome.Outcome, outcome.ErrorCode = enrollment.TraegerOutcomeRejected, run.offeringCodes[offering.ID]
	case run.result.Outcome == enrollment.TraegerOutcomeApplied:
		version := next.offerings[offering.ID].version
		outcome.AppliedVersion = &version
	}
	return outcome, recordedVersion(offering, run.target.schoolID) != nil || run.result.Outcome != enrollment.TraegerOutcomeWithdrawn
}
