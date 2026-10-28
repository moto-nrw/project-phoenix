package enrollmenthttp_test

import (
	"context"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// decisionFixtureRoot mirrors the HTTP root's school unit around direct flow calls.
type decisionFixtureRoot struct{ enrollmentAPI.DecisionService }

func (r decisionFixtureRoot) List(ctx context.Context, filters enrollmentAPI.RequestFilters) (value0 []*enrollmentAPI.RequestSummary, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.DecisionService.List(txCtx, filters)
		return err
	})
	return value0, err
}
func (r decisionFixtureRoot) ListByStudent(ctx context.Context, studentID int64) (value0 []*enrollmentAPI.RequestSummary, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.DecisionService.ListByStudent(txCtx, studentID)
		return err
	})
	return value0, err
}
func (r decisionFixtureRoot) Get(ctx context.Context, requestID int64) (value0 *enrollmentAPI.RequestSummary, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.DecisionService.Get(txCtx, requestID)
		return err
	})
	return value0, err
}
func (r decisionFixtureRoot) Decide(ctx context.Context, input enrollmentAPI.DecideInput) (value0 *enrollmentAPI.DecideOutcome, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.DecisionService.Decide(txCtx, input)
		return err
	})
	return value0, err
}
func (r decisionFixtureRoot) RestoreWithdrawn(ctx context.Context, requestID int64, restoredBy int64) (value0 *enrollmentAPI.RestoreOutcome, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.DecisionService.RestoreWithdrawn(txCtx, requestID, restoredBy)
		return err
	})
	return value0, err
}
func (r decisionFixtureRoot) UpdateChildOfferings(ctx context.Context, input enrollmentAPI.UpdateChildOfferingsInput) (value0 *enrollmentAPI.ChildOfferingsUpdate, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.DecisionService.UpdateChildOfferings(txCtx, input)
		return err
	})
	return value0, err
}
func (r decisionFixtureRoot) ListOfferingAdjustments(ctx context.Context, requestID int64, requestChildID int64) (value0 []*enrollmentAPI.OfferingAdjustment, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.DecisionService.ListOfferingAdjustments(txCtx, requestID, requestChildID)
		return err
	})
	return value0, err
}
func (r decisionFixtureRoot) ListChildOfferings(ctx context.Context, requestID int64) (value0 map[int64]enrollmentAPI.ChildOfferingSet, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.DecisionService.ListChildOfferings(txCtx, requestID)
		return err
	})
	return value0, err
}
func (r decisionFixtureRoot) ExportPhase(ctx context.Context, phaseID int64, actorAccountID int64, actorRole string, format string, childStatusFilter string) (value0 *enrollmentAPI.PhaseExport, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.DecisionService.ExportPhase(txCtx, phaseID, actorAccountID, actorRole, format, childStatusFilter)
		return err
	})
	return value0, err
}
func (r decisionFixtureRoot) ExportStudent(ctx context.Context, studentID int64, actorAccountID int64, actorRole string, format string) (value0 *enrollmentAPI.StudentEnrollmentExport, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.DecisionService.ExportStudent(txCtx, studentID, actorAccountID, actorRole, format)
		return err
	})
	return value0, err
}
func (r decisionFixtureRoot) RecordPhaseExportAudit(ctx context.Context, actorAccountID int64, actorRole string, phase *capability.Phase, format string, statusFilter string, requestCount int, childCount int) (err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		return r.DecisionService.RecordPhaseExportAudit(txCtx, actorAccountID, actorRole, phase, format, statusFilter, requestCount, childCount)
	})
	return err
}
