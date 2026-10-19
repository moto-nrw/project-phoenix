package contracttest_test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// offeringChangesFixtureRoot supplies the HTTP root unit around direct application calls.
type offeringChangesFixtureRoot struct {
	careplan.OfferingChangeCapability
}

func (r offeringChangesFixtureRoot) Catalog(ctx context.Context, studentID int64) (*careplan.OfferingChangeCatalog, error) {
	return fixtureEnrollmentQuery(ctx, func(txCtx context.Context) (*careplan.OfferingChangeCatalog, error) {
		return r.OfferingChangeCapability.Catalog(txCtx, studentID)
	})
}
func (r offeringChangesFixtureRoot) CatalogAt(ctx context.Context, studentID int64, effectiveFrom calendar.Date) (*careplan.OfferingChangeCatalog, error) {
	return fixtureEnrollmentQuery(ctx, func(txCtx context.Context) (*careplan.OfferingChangeCatalog, error) {
		return r.OfferingChangeCapability.CatalogAt(txCtx, studentID, effectiveFrom)
	})
}
func (r offeringChangesFixtureRoot) GetForStudent(ctx context.Context, studentID int64) (*careplan.OfferingChangeView, error) {
	return fixtureEnrollmentQuery(ctx, func(txCtx context.Context) (*careplan.OfferingChangeView, error) {
		return r.OfferingChangeCapability.GetForStudent(txCtx, studentID)
	})
}
func (r offeringChangesFixtureRoot) SubmitOfferingChange(ctx context.Context, input careplan.CreateOfferingChangeInput) (*careplan.OfferingChangeRequest, error) {
	return fixtureEnrollmentQuery(ctx, func(txCtx context.Context) (*careplan.OfferingChangeRequest, error) {
		return r.OfferingChangeCapability.SubmitOfferingChange(txCtx, input)
	})
}
func (r offeringChangesFixtureRoot) Edit(ctx context.Context, requestID int64, input careplan.CreateOfferingChangeInput, expectedVersion string) (*careplan.OfferingChangeRequest, error) {
	return fixtureEnrollmentQuery(ctx, func(txCtx context.Context) (*careplan.OfferingChangeRequest, error) {
		return r.OfferingChangeCapability.Edit(txCtx, requestID, input, expectedVersion)
	})
}
func (r offeringChangesFixtureRoot) EarliestEffectiveFrom(ctx context.Context) (calendar.Date, error) {
	return fixtureEnrollmentQuery(ctx, func(txCtx context.Context) (calendar.Date, error) {
		return r.OfferingChangeCapability.EarliestEffectiveFrom(txCtx)
	})
}
func (r offeringChangesFixtureRoot) PreviewDecision(ctx context.Context, requestID int64, excludedIDs []int64, effectiveFrom *calendar.Date) (*careplan.OfferingChangePreview, error) {
	return fixtureEnrollmentQuery(ctx, func(txCtx context.Context) (*careplan.OfferingChangePreview, error) {
		return r.OfferingChangeCapability.PreviewDecision(txCtx, requestID, excludedIDs, effectiveFrom)
	})
}
func (r offeringChangesFixtureRoot) Decide(ctx context.Context, input careplan.OfferingChangeDecisionInput) error {
	return fixtureEnrollmentCommand(ctx, func(txCtx context.Context) error { return r.OfferingChangeCapability.Decide(txCtx, input) })
}
func (r offeringChangesFixtureRoot) MarkDone(ctx context.Context, requestID int64, expectedVersion, reason string, reviewedBy int64) error {
	return fixtureEnrollmentCommand(ctx, func(txCtx context.Context) error {
		return r.OfferingChangeCapability.MarkDone(txCtx, requestID, expectedVersion, reason, reviewedBy)
	})
}
func (r offeringChangesFixtureRoot) ConflictCandidate(ctx context.Context, requestID int64) (*careplan.OfferingConflictCandidate, error) {
	return fixtureEnrollmentQuery(ctx, func(txCtx context.Context) (*careplan.OfferingConflictCandidate, error) {
		return r.OfferingChangeCapability.ConflictCandidate(txCtx, requestID)
	})
}
func (r offeringChangesFixtureRoot) LockConflictRequest(ctx context.Context, requestID int64) error {
	return fixtureEnrollmentCommand(ctx, func(txCtx context.Context) error {
		return r.OfferingChangeCapability.LockConflictRequest(txCtx, requestID)
	})
}
func (r offeringChangesFixtureRoot) DecideConflictRequest(ctx context.Context, decision careplan.OfferingConflictDecision) error {
	return fixtureEnrollmentCommand(ctx, func(txCtx context.Context) error {
		return r.OfferingChangeCapability.DecideConflictRequest(txCtx, decision)
	})
}
func (r offeringChangesFixtureRoot) WriteStaffValue(ctx context.Context, write careplan.OfferingStaffValueWrite) error {
	return fixtureEnrollmentCommand(ctx, func(txCtx context.Context) error { return r.OfferingChangeCapability.WriteStaffValue(txCtx, write) })
}
func (r offeringChangesFixtureRoot) PrepareDirectOfferingAdjustment(ctx context.Context, input careplan.DirectOfferingAdjustmentInput) error {
	return fixtureEnrollmentCommand(ctx, func(txCtx context.Context) error {
		return r.OfferingChangeCapability.PrepareDirectOfferingAdjustment(txCtx, input)
	})
}
func (r offeringChangesFixtureRoot) PreviewDirectOfferingAdjustment(ctx context.Context, input careplan.DirectOfferingAdjustmentInput) (*careplan.DirectOfferingAdjustmentPreview, error) {
	return fixtureEnrollmentQuery(ctx, func(txCtx context.Context) (*careplan.DirectOfferingAdjustmentPreview, error) {
		return r.OfferingChangeCapability.PreviewDirectOfferingAdjustment(txCtx, input)
	})
}
func (r offeringChangesFixtureRoot) ApplyDirectOfferingAdjustment(ctx context.Context, input careplan.DirectOfferingAdjustmentInput) error {
	return fixtureEnrollmentCommand(ctx, func(txCtx context.Context) error {
		return r.OfferingChangeCapability.ApplyDirectOfferingAdjustment(txCtx, input)
	})
}
func (r offeringChangesFixtureRoot) CourseCatalog(ctx context.Context, studentID, accountID int64) (*careplan.CourseCatalog, error) {
	return fixtureEnrollmentQuery(ctx, func(txCtx context.Context) (*careplan.CourseCatalog, error) {
		return r.OfferingChangeCapability.CourseCatalog(txCtx, studentID, accountID)
	})
}
func (r offeringChangesFixtureRoot) CreateCourseRequest(ctx context.Context, input careplan.CreateCourseRequestInput) (*careplan.OfferingChangeRequest, error) {
	return fixtureEnrollmentQuery(ctx, func(txCtx context.Context) (*careplan.OfferingChangeRequest, error) {
		return r.OfferingChangeCapability.CreateCourseRequest(txCtx, input)
	})
}
func (r offeringChangesFixtureRoot) WithdrawCourseRequest(ctx context.Context, requestID, accountID, studentID int64) error {
	return fixtureEnrollmentCommand(ctx, func(txCtx context.Context) error {
		return r.OfferingChangeCapability.WithdrawCourseRequest(txCtx, requestID, accountID, studentID)
	})
}

// offeringReviewFixtureRoot scopes the full review projection, not only its rows.
type offeringReviewFixtureRoot struct{ careplan.OfferingReviewQuery }

func (r offeringReviewFixtureRoot) ListPending(ctx context.Context, filter careplan.RequestQueueFilter) (value []*careplan.OfferingReviewItem, cursor *careplan.RequestCursor, err error) {
	err = fixtureEnrollmentCommand(ctx, func(txCtx context.Context) error {
		value, cursor, err = r.OfferingReviewQuery.ListPending(txCtx, filter)
		return err
	})
	return
}
func (r offeringReviewFixtureRoot) ListHistory(ctx context.Context, filter careplan.RequestQueueFilter) (value []*careplan.OfferingHistoryItem, cursor *careplan.RequestCursor, err error) {
	err = fixtureEnrollmentCommand(ctx, func(txCtx context.Context) error {
		value, cursor, err = r.OfferingReviewQuery.ListHistory(txCtx, filter)
		return err
	})
	return
}
func (r offeringReviewFixtureRoot) PendingCount(ctx context.Context, date careplan.Date) (int, error) {
	return fixtureEnrollmentQuery(ctx, func(txCtx context.Context) (int, error) { return r.OfferingReviewQuery.PendingCount(txCtx, date) })
}
