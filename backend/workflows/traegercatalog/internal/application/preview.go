package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/workflows/traegercatalog"
	"github.com/moto-ogs/project-phoenix/workflows/traegercatalog/ports"
)

// fingerprint ties a rollout to its preview: the draft with its targets as
// the Träger transaction read them, and the state of every target school as
// its tenant transaction read it (ADR 0195 section 5).
type fingerprint struct {
	Draft   string           `json:"d"`
	Schools map[int64]string `json:"s"`
}

func (f fingerprint) encode() string {
	encoded, _ := json.Marshal(f)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func decodeFingerprint(token string) (fingerprint, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return fingerprint{}, traegercatalog.ErrPreviewStale
	}
	var f fingerprint
	if err := json.Unmarshal(raw, &f); err != nil || f.Draft == "" {
		return fingerprint{}, traegercatalog.ErrPreviewStale
	}
	return f, nil
}

// draftHash covers everything a rollout releases and where: the phase and
// offering drafts, their withdrawals, target schools and billing baselines.
func (c *Catalog) draftHash(snap snapshot, targets []target) string {
	offerings := make([]any, 0, len(snap.offerings))
	for _, offering := range snap.offerings {
		offerings = append(offerings, []any{offering.ID, offering.Draft, offering.WithdrawnAt != nil})
	}
	plan := make([]any, 0, len(targets))
	for _, t := range targets {
		baselines := make([]any, 0, len(snap.offerings))
		for _, offering := range snap.offerings {
			baselines = append(baselines, []any{offering.ID, recordedVersion(offering, t.schoolID), recordedCopyID(offering, t.schoolID)})
		}
		plan = append(plan, []any{t.schoolID, t.action, baselines})
	}
	return c.hashOf([]any{snap.phase.ID, snap.phase.Draft, snap.phase.WithdrawnAt != nil, offerings, plan, snap.month})
}

// Preview runs the rollout in every target school and rolls each school
// back: the plan shows per school what the rollout would do or why the
// school refuses it, with the prices and contract templates it would write.
func (c *Catalog) Preview(ctx context.Context, actor traegercatalog.Actor, traegerPhaseID int64) (traegercatalog.Preview, error) {
	snap, err := c.read(ctx, actor, traegerPhaseID, false)
	if err != nil {
		return traegercatalog.Preview{}, err
	}
	targets := snap.targets()
	next := snap.prospective()
	print := fingerprint{Draft: c.draftHash(snap, targets), Schools: map[int64]string{}}
	preview := traegercatalog.Preview{TraegerPhaseID: traegerPhaseID, Version: next.phaseVersion, Schools: []traegercatalog.SchoolPlan{}}
	for _, t := range targets {
		plan, state, err := c.previewSchool(ctx, actor, snap, next, t)
		if err != nil {
			return traegercatalog.Preview{}, err
		}
		print.Schools[t.schoolID] = state
		preview.Schools = append(preview.Schools, plan)
	}
	preview.Fingerprint = print.encode()
	return preview, nil
}

func (c *Catalog) previewSchool(ctx context.Context, actor traegercatalog.Actor, snap snapshot, next release, t target) (traegercatalog.SchoolPlan, string, error) {
	var plan traegercatalog.SchoolPlan
	var copies map[int64]careplan.TraegerOfferingApplied
	var state string
	run := schoolRun{target: t}
	err := c.deps.Runtime.WithinSchool(ctx, t.schoolID, func(ctx context.Context) error {
		var err error
		if err = c.lockSchool(ctx, snap.phase.OrganizationID, t.schoolID); err != nil {
			return err
		}
		if state, err = c.schoolState(ctx, snap, false); err != nil {
			return err
		}
		if plan, copies, err = c.applySchool(ctx, snap, next, t.action); err != nil {
			return err
		}
		run.copies = copies
		run.prices, run.templates, err = c.preparePreviewBilling(ctx, actor, snap, next, run)
		return err
	})
	if errors.Is(err, errPreviewDiscard) && t.action != traegercatalog.ActionWithdraw {
		err = c.previewBilling(ctx, actor, run)
	}
	plan.SchoolID = t.schoolID
	switch code, refused := rejectionCode(err); {
	case errors.Is(err, errPreviewDiscard):
		plan.Outcome = traegercatalog.PreviewReady
	case errors.Is(err, errStale):
		return plan, "", traegercatalog.ErrPreviewStale
	case refused:
		plan.Action, plan.Outcome, plan.ErrorCode = t.action, traegercatalog.PreviewRejected, code
		plan.CareBeyondEnd = 0
	default:
		return plan, "", err
	}
	if t.action != traegercatalog.ActionWithdraw && plan.Outcome == traegercatalog.PreviewReady {
		c.planBilling(snap, next, t.schoolID, &plan, copies)
	}
	return plan, state, nil
}

// planBilling names per offering the month a new tariff starts and whether
// the contract template is written (ADR 0195 section 6).
func (c *Catalog) planBilling(snap snapshot, next release, schoolID int64, plan *traegercatalog.SchoolPlan, copies map[int64]careplan.TraegerOfferingApplied) {
	byID := map[int64]careplan.TraegerOffering{}
	for _, offering := range snap.offerings {
		byID[offering.ID] = offering
	}
	for index, offeringPlan := range plan.Offerings {
		offering, ok := byID[offeringPlan.TraegerOfferingID]
		if !ok || offering.WithdrawnAt != nil {
			continue
		}
		billing := billingFor(offering, next.offerings[offering.ID], schoolID, snap.phase.Draft, next.month, copies[offering.ID])
		plan.Offerings[index].PricesFrom = billing.pricesFrom
		plan.Offerings[index].ContractTemplate = billing.template
	}
}

// Exercise both financial owners in a separate Träger unit and discard it.
func (c *Catalog) previewBilling(ctx context.Context, actor traegercatalog.Actor, run schoolRun) error {
	return c.deps.Runtime.WithinTraeger(ctx, actor.OrganizationID, func(ctx context.Context) error {
		if err := c.deps.Tariffs(ctx, actor.OrganizationID, actor.AccountID, run.prices); err != nil {
			return err
		}
		if err := run.templates(ctx); err != nil {
			return err
		}
		return errPreviewDiscard
	})
}

func (c *Catalog) preparePreviewBilling(ctx context.Context, actor traegercatalog.Actor, snap snapshot, next release, run schoolRun) ([]ports.ModulePrices, func(context.Context) error, error) {
	if run.target.action == traegercatalog.ActionWithdraw {
		return nil, nil, errPreviewDiscard
	}
	prices, templates, err := c.prepareBilling(ctx, actor, snap, next, run)
	if err != nil {
		return nil, nil, err
	}
	return prices, templates, errPreviewDiscard
}
