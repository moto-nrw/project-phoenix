package contracttest_test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
)

// scopedCareOfferingRows is the fixture root around catalog administration,
// including its cross-owner validation reads before its first mutation.
type scopedCareOfferingRows struct {
	*enrollmentTest.CareOfferingRows
}

func newScopedOfferingRows(catalog careplan.CareOfferingCatalog) *scopedCareOfferingRows {
	return &scopedCareOfferingRows{enrollmentTest.NewCareOfferingRows(catalog)}
}
func (r *scopedCareOfferingRows) Create(ctx context.Context, row *offeringContract.CareOfferingRecord) (*offeringContract.CareOfferingRecord, error) {
	return fixtureEnrollmentQuery(ctx, func(txCtx context.Context) (*offeringContract.CareOfferingRecord, error) {
		return r.CareOfferingRows.Create(txCtx, row)
	})
}
func (r *scopedCareOfferingRows) Update(ctx context.Context, row *offeringContract.CareOfferingRecord) error {
	return fixtureEnrollmentCommand(ctx, func(txCtx context.Context) error { return r.CareOfferingRows.Update(txCtx, row) })
}
func (r *scopedCareOfferingRows) Delete(ctx context.Context, id int64) error {
	return fixtureEnrollmentCommand(ctx, func(txCtx context.Context) error { return r.CareOfferingRows.Delete(txCtx, id) })
}
func (r *scopedCareOfferingRows) List(ctx context.Context) ([]*offeringContract.CareOfferingRecord, error) {
	return fixtureEnrollmentQuery(ctx, func(txCtx context.Context) ([]*offeringContract.CareOfferingRecord, error) {
		return r.CareOfferingRows.List(txCtx)
	})
}
func (r *scopedCareOfferingRows) ListByPhase(ctx context.Context, id int64) ([]*offeringContract.CareOfferingRecord, error) {
	return fixtureEnrollmentQuery(ctx, func(txCtx context.Context) ([]*offeringContract.CareOfferingRecord, error) {
		return r.CareOfferingRows.ListByPhase(txCtx, id)
	})
}
func (r *scopedCareOfferingRows) GetByID(ctx context.Context, id int64) (*offeringContract.CareOfferingRecord, error) {
	return fixtureEnrollmentQuery(ctx, func(txCtx context.Context) (*offeringContract.CareOfferingRecord, error) {
		return r.CareOfferingRows.GetByID(txCtx, id)
	})
}
func (r *scopedCareOfferingRows) Clone(ctx context.Context, source, target int64) (*offeringContract.CareOfferingRecord, error) {
	return fixtureEnrollmentQuery(ctx, func(txCtx context.Context) (*offeringContract.CareOfferingRecord, error) {
		return r.CareOfferingRows.Clone(txCtx, source, target)
	})
}
