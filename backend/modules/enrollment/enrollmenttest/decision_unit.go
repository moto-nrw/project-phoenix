package enrollmenttest

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// enrollmentDecisionFixtureRoot supplies the unit owned by the production HTTP root.
type enrollmentDecisionFixtureRoot struct {
	enrollment.Decisions
	within func(context.Context, func(context.Context) error) error
}

// WithDecisionUnit supplies the outer root unit around direct fixture calls.
func WithDecisionUnit(decisions enrollment.Decisions, within func(context.Context, func(context.Context) error) error) enrollment.Decisions {
	return enrollmentDecisionFixtureRoot{decisions, within}
}
func decisionFixtureQuery[T any](within func(context.Context, func(context.Context) error) error, ctx context.Context, call func(context.Context) (T, error)) (value T, err error) {
	err = within(ctx, func(txCtx context.Context) error { value, err = call(txCtx); return err })
	return
}

func (r enrollmentDecisionFixtureRoot) DecisionRequests(ctx context.Context, filters enrollment.DecisionRequestFilters) ([]*enrollment.DecisionSummary, error) {
	return decisionFixtureQuery(r.within, ctx, func(txCtx context.Context) ([]*enrollment.DecisionSummary, error) {
		return r.Decisions.DecisionRequests(txCtx, filters)
	})
}
func (r enrollmentDecisionFixtureRoot) StudentDecisionRequests(ctx context.Context, studentID int64) ([]*enrollment.DecisionSummary, error) {
	return decisionFixtureQuery(r.within, ctx, func(txCtx context.Context) ([]*enrollment.DecisionSummary, error) {
		return r.Decisions.StudentDecisionRequests(txCtx, studentID)
	})
}
func (r enrollmentDecisionFixtureRoot) DecisionRequest(ctx context.Context, requestID int64) (*enrollment.DecisionSummary, error) {
	return decisionFixtureQuery(r.within, ctx, func(txCtx context.Context) (*enrollment.DecisionSummary, error) {
		return r.Decisions.DecisionRequest(txCtx, requestID)
	})
}
func (r enrollmentDecisionFixtureRoot) Decide(ctx context.Context, input enrollment.DecideInput) (*enrollment.DecideOutcome, error) {
	return decisionFixtureQuery(r.within, ctx, func(txCtx context.Context) (*enrollment.DecideOutcome, error) {
		return r.Decisions.Decide(txCtx, input)
	})
}
func (r enrollmentDecisionFixtureRoot) RestoreWithdrawn(ctx context.Context, requestID, restoredBy int64) (*enrollment.RestoreOutcome, error) {
	return decisionFixtureQuery(r.within, ctx, func(txCtx context.Context) (*enrollment.RestoreOutcome, error) {
		return r.Decisions.RestoreWithdrawn(txCtx, requestID, restoredBy)
	})
}
func (r enrollmentDecisionFixtureRoot) UpdateChildOfferings(ctx context.Context, input enrollment.UpdateChildOfferingsInput) (*enrollment.RequestChild, error) {
	return decisionFixtureQuery(r.within, ctx, func(txCtx context.Context) (*enrollment.RequestChild, error) {
		return r.Decisions.UpdateChildOfferings(txCtx, input)
	})
}
func (r enrollmentDecisionFixtureRoot) ListOfferingAdjustments(ctx context.Context, requestID, requestChildID int64) ([]*enrollment.OfferingAdjustmentRecord, error) {
	return decisionFixtureQuery(r.within, ctx, func(txCtx context.Context) ([]*enrollment.OfferingAdjustmentRecord, error) {
		return r.Decisions.ListOfferingAdjustments(txCtx, requestID, requestChildID)
	})
}
func (r enrollmentDecisionFixtureRoot) ListChildOfferings(ctx context.Context, requestID int64) (map[int64]enrollment.ChildOfferingSet, error) {
	return decisionFixtureQuery(r.within, ctx, func(txCtx context.Context) (map[int64]enrollment.ChildOfferingSet, error) {
		return r.Decisions.ListChildOfferings(txCtx, requestID)
	})
}
func (r enrollmentDecisionFixtureRoot) ExportPhase(ctx context.Context, phaseID, actorAccountID int64, actorRole, format, childStatusFilter string) (*enrollment.PhaseExport, error) {
	return decisionFixtureQuery(r.within, ctx, func(txCtx context.Context) (*enrollment.PhaseExport, error) {
		return r.Decisions.ExportPhase(txCtx, phaseID, actorAccountID, actorRole, format, childStatusFilter)
	})
}
func (r enrollmentDecisionFixtureRoot) ExportStudent(ctx context.Context, studentID, actorAccountID int64, actorRole, format string) (*enrollment.StudentEnrollmentExport, error) {
	return decisionFixtureQuery(r.within, ctx, func(txCtx context.Context) (*enrollment.StudentEnrollmentExport, error) {
		return r.Decisions.ExportStudent(txCtx, studentID, actorAccountID, actorRole, format)
	})
}
func (r enrollmentDecisionFixtureRoot) RecordPhaseExportAudit(ctx context.Context, actorAccountID int64, actorRole string, phase *enrollment.Phase, format, statusFilter string, requestCount, childCount int) error {
	return r.within(ctx, func(txCtx context.Context) error {
		return r.Decisions.RecordPhaseExportAudit(txCtx, actorAccountID, actorRole, phase, format, statusFilter, requestCount, childCount)
	})
}
