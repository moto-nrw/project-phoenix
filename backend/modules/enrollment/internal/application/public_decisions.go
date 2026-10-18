package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// PublicDecisions hands the decision flow to the HTTP adapters: refusals that
// originate in the selection contract or in Care Plan come back marked with
// Enrollment's public values as well.
func PublicDecisions(decisions *Decisions) enrollment.Decisions {
	return publicDecisions{inner: decisions}
}

type publicDecisions struct{ inner *Decisions }

func (d publicDecisions) DecisionRequests(ctx context.Context, filters enrollment.DecisionRequestFilters) ([]*enrollment.DecisionSummary, error) {
	out, err := d.inner.DecisionRequests(ctx, filters)
	return out, PublicError(err)
}

func (d publicDecisions) StudentDecisionRequests(ctx context.Context, studentID int64) ([]*enrollment.DecisionSummary, error) {
	out, err := d.inner.StudentDecisionRequests(ctx, studentID)
	return out, PublicError(err)
}

func (d publicDecisions) DecisionRequest(ctx context.Context, requestID int64) (*enrollment.DecisionSummary, error) {
	out, err := d.inner.DecisionRequest(ctx, requestID)
	return out, PublicError(err)
}

func (d publicDecisions) Decide(ctx context.Context, input enrollment.DecideInput) (*enrollment.DecideOutcome, error) {
	out, err := d.inner.Decide(ctx, input)
	return out, PublicError(err)
}

func (d publicDecisions) RestoreWithdrawn(ctx context.Context, requestID, restoredBy int64) (*enrollment.RestoreOutcome, error) {
	out, err := d.inner.RestoreWithdrawn(ctx, requestID, restoredBy)
	return out, PublicError(err)
}

func (d publicDecisions) UpdateChildOfferings(ctx context.Context, input enrollment.UpdateChildOfferingsInput) (*enrollment.RequestChild, error) {
	out, err := d.inner.UpdateChildOfferings(ctx, input)
	return out, PublicError(err)
}

func (d publicDecisions) ListOfferingAdjustments(ctx context.Context, requestID, requestChildID int64) ([]*enrollment.OfferingAdjustmentRecord, error) {
	out, err := d.inner.ListOfferingAdjustments(ctx, requestID, requestChildID)
	return out, PublicError(err)
}

func (d publicDecisions) ListChildOfferings(ctx context.Context, requestID int64) (map[int64]enrollment.ChildOfferingSet, error) {
	out, err := d.inner.ListChildOfferings(ctx, requestID)
	return out, PublicError(err)
}

func (d publicDecisions) ExportPhase(ctx context.Context, phaseID, actorAccountID int64, actorRole, format, childStatusFilter string) (*enrollment.PhaseExport, error) {
	out, err := d.inner.ExportPhase(ctx, phaseID, actorAccountID, actorRole, format, childStatusFilter)
	return out, PublicError(err)
}

func (d publicDecisions) ExportStudent(ctx context.Context, studentID, actorAccountID int64, actorRole, format string) (*enrollment.StudentEnrollmentExport, error) {
	out, err := d.inner.ExportStudent(ctx, studentID, actorAccountID, actorRole, format)
	return out, PublicError(err)
}

func (d publicDecisions) RecordPhaseExportAudit(ctx context.Context, actorAccountID int64, actorRole string, phase *enrollment.Phase, format, statusFilter string, requestCount, childCount int) error {
	return PublicError(d.inner.RecordPhaseExportAudit(ctx, actorAccountID, actorRole, phase, format, statusFilter, requestCount, childCount))
}

// errRequestReadsNotConfigured reports a composition without the read state.
var errRequestReadsNotConfigured = errors.New("enrollment: request read state not configured")

func (d publicDecisions) reads() (enrollment.RequestReads, error) {
	if reads := d.inner.RequestReads(); reads != nil {
		return reads, nil
	}
	return nil, errRequestReadsNotConfigured
}

func (d publicDecisions) CountUnreadRequests(ctx context.Context, accountID int64) (int, error) {
	reads, err := d.reads()
	if err != nil {
		return 0, err
	}
	return reads.CountUnreadRequests(ctx, accountID)
}

func (d publicDecisions) UnreadRequestIDs(ctx context.Context, accountID int64, requestIDs []int64) ([]int64, error) {
	reads, err := d.reads()
	if err != nil {
		return nil, err
	}
	return reads.UnreadRequestIDs(ctx, accountID, requestIDs)
}

func (d publicDecisions) MarkRequestsRead(ctx context.Context, accountID int64, requestIDs []int64) error {
	reads, err := d.reads()
	if err != nil {
		return err
	}
	return reads.MarkRequestsRead(ctx, accountID, requestIDs)
}

func (d publicDecisions) MarkAllRequestsRead(ctx context.Context, accountID int64) error {
	reads, err := d.reads()
	if err != nil {
		return err
	}
	return reads.MarkAllRequestsRead(ctx, accountID)
}

func (d publicDecisions) MarkRequestUnread(ctx context.Context, accountID, requestID int64) error {
	reads, err := d.reads()
	if err != nil {
		return err
	}
	return reads.MarkRequestUnread(ctx, accountID, requestID)
}
