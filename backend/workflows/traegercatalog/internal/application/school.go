package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/workflows/traegercatalog"
)

// schoolState hashes the state of a school's copies: the phase copy and the
// offering copies of the Träger phase. lock takes their row locks first,
// phase copy before offering copies in ID order (ADR 0195 section 5).
func (c *Catalog) schoolState(ctx context.Context, snap snapshot, lock bool) (string, error) {
	phase, err := c.deps.SchoolPhases.TraegerPhaseCopyState(ctx, snap.phase.ID, lock)
	if err != nil {
		return "", err
	}
	ids := make([]int64, 0, len(snap.offerings))
	for _, offering := range snap.offerings {
		ids = append(ids, offering.ID)
	}
	offerings, err := c.deps.SchoolOfferings.TraegerOfferingCopyStates(ctx, ids, lock)
	if err != nil {
		return "", err
	}
	stamps := []string{phase.Stamp}
	for _, offering := range offerings {
		stamps = append(stamps, offering.Stamp)
	}
	sort.Strings(stamps[1:])
	return c.hashOf(stamps), nil
}

// applySchool brings one school to the release, or withdraws its copies,
// through the owners' copy commands. The caller runs it in the school's
// tenant transaction; a returned error rolls the whole school back.
func (c *Catalog) applySchool(ctx context.Context, snap snapshot, next release, action string) (traegercatalog.SchoolPlan, map[int64]int64, error) {
	plan := traegercatalog.SchoolPlan{Action: action, Phase: []enrollment.TraegerFieldChange{}, Offerings: []traegercatalog.OfferingPlan{}}
	if action == traegercatalog.ActionWithdraw {
		return plan, nil, c.withdrawSchool(ctx, snap, &plan)
	}
	applied, err := c.deps.SchoolPhases.ApplyTraegerPhase(ctx, enrollment.TraegerPhaseCopy{
		SourceID: snap.phase.ID, Version: next.phaseVersion, Fields: next.phase,
	})
	if err != nil {
		return plan, nil, err
	}
	if applied.Created {
		plan.Action = traegercatalog.ActionCreate
	}
	plan.Phase = applied.Fields
	copies := []careplan.TraegerOfferingCopy{}
	withdrawn := []int64{}
	for _, offering := range snap.offerings {
		if offering.WithdrawnAt != nil {
			withdrawn = append(withdrawn, offering.ID)
			continue
		}
		released := next.offerings[offering.ID]
		copies = append(copies, careplan.TraegerOfferingCopy{SourceID: offering.ID, Version: released.version, Fields: released.draft.Fields, PhaseID: applied.CopyID})
	}
	batch, err := c.deps.SchoolOfferings.ApplyTraegerOfferings(ctx, copies, withdrawn)
	if err != nil {
		return plan, nil, err
	}
	bySource := map[int64]careplan.TraegerOfferingApplied{}
	copyIDs := map[int64]int64{}
	for _, row := range batch.Applied {
		bySource[row.SourceID] = row
		copyIDs[row.SourceID] = row.CopyID
	}
	for _, offering := range snap.offerings {
		row := bySource[offering.ID]
		offeringPlan := traegercatalog.OfferingPlan{TraegerOfferingID: offering.ID, Name: offering.Draft.Fields.Name, Fields: row.Fields, Action: traegercatalog.ActionUpdate}
		if offering.WithdrawnAt != nil {
			offeringPlan.Action = traegercatalog.ActionWithdraw
		} else if row.Created {
			offeringPlan.Action = traegercatalog.ActionCreate
		}
		plan.Offerings = append(plan.Offerings, offeringPlan)
	}
	return plan, copyIDs, nil
}

// withdrawSchool withdraws the offering copies first, then the phase copy,
// which only goes when no offering is left in it.
func (c *Catalog) withdrawSchool(ctx context.Context, snap snapshot, plan *traegercatalog.SchoolPlan) error {
	sources := make([]int64, 0, len(snap.offerings))
	for _, offering := range snap.offerings {
		sources = append(sources, offering.ID)
	}
	if _, err := c.deps.SchoolOfferings.ApplyTraegerOfferings(ctx, nil, sources); err != nil {
		return err
	}
	for _, offering := range snap.offerings {
		plan.Offerings = append(plan.Offerings, traegercatalog.OfferingPlan{
			TraegerOfferingID: offering.ID, Name: offering.Draft.Fields.Name, Action: traegercatalog.ActionWithdraw,
			Fields: []careplan.TraegerFieldChange{},
		})
	}
	_, err := c.deps.SchoolPhases.WithdrawTraegerPhase(ctx, snap.phase.ID)
	return err
}

// hashOf is the fingerprint of the JSON of value.
func (c *Catalog) hashOf(value any) string {
	encoded, _ := json.Marshal(value)
	return c.deps.Fingerprint(encoded)
}

func sameJSON(left, right any) bool {
	a, errA := json.Marshal(left)
	b, errB := json.Marshal(right)
	return errA == nil && errB == nil && bytes.Equal(a, b)
}

// lockSchool keeps ownership stable until this school transaction ends. A
// Träger change takes an incompatible lock before detaching its copies.
func (c *Catalog) lockSchool(ctx context.Context, organizationID, schoolID int64) error {
	school, err := c.deps.Schools.FindSchoolForShare(ctx, schoolID)
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return errStale
	}
	if err != nil {
		return err
	}
	if school.OrganizationID != organizationID || school.IsDeleted() {
		return errStale
	}
	return nil
}
