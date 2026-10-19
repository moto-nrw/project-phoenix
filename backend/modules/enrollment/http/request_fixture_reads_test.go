package enrollmenthttp_test

import (
	"context"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// requestFixtureRoot mirrors the HTTP root's school unit around direct flow calls.

func (r requestFixtureRoot) EditModeForStatus(ctx context.Context, req *enrollmentAPI.Request, children []*enrollmentAPI.RequestChild) (value0 string, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.RequestService.EditModeForStatus(txCtx, req, children)
		return err
	})
	return value0, err
}
func (r requestFixtureRoot) GuardiansByStatusToken(ctx context.Context, token string) (value0 []*capability.RequestGuardian, err error) {
	err = fixturePublicTokenRoot(ctx, func(txCtx context.Context) error {
		value0, err = r.RequestService.GuardiansByStatusToken(txCtx, token)
		return err
	})
	return value0, err
}
func (r requestFixtureRoot) LegalTexts(ctx context.Context) (value0 enrollmentAPI.LegalTexts, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.RequestService.LegalTexts(txCtx)
		return err
	})
	return value0, err
}
func (r requestFixtureRoot) LegalTextsForPhaseWithLateInvite(ctx context.Context, phaseID int64, lateInviteToken string) (value0 enrollmentAPI.LegalTexts, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.RequestService.LegalTextsForPhaseWithLateInvite(txCtx, phaseID, lateInviteToken)
		return err
	})
	return value0, err
}
func (r requestFixtureRoot) LoadPublicFormBootstrap(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (value0 *enrollmentAPI.PublicFormBootstrapData, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.RequestService.LoadPublicFormBootstrap(txCtx, phaseID, now, lateInviteToken)
		return err
	})
	return value0, err
}
func (r requestFixtureRoot) LoadEnrolleeFormBootstrap(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string, access enrollmentAPI.EnrolleeAudienceAccess) (value0 *enrollmentAPI.PublicFormBootstrapData, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.RequestService.LoadEnrolleeFormBootstrap(txCtx, phaseID, now, lateInviteToken, access)
		return err
	})
	return value0, err
}
func (r requestFixtureRoot) LoadPublicCareOfferings(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (value0 *enrollmentAPI.PublicFormBootstrapData, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.RequestService.LoadPublicCareOfferings(txCtx, phaseID, now, lateInviteToken)
		return err
	})
	return value0, err
}
func (r requestFixtureRoot) LoadManualEnrollmentBootstrap(ctx context.Context, phaseID int64) (value0 *enrollmentAPI.PublicFormBootstrapData, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.RequestService.LoadManualEnrollmentBootstrap(txCtx, phaseID)
		return err
	})
	return value0, err
}
func (r requestFixtureRoot) CreateManualApprovedEnrollment(ctx context.Context, input enrollmentAPI.ManualApprovedEnrollmentInput) (value0 *enrollmentAPI.ManualApprovedEnrollmentResult, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.RequestService.CreateManualApprovedEnrollment(txCtx, input)
		return err
	})
	return value0, err
}
func (r requestFixtureRoot) PublicActiveSchema(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (value0 *capability.FormSchema, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.RequestService.PublicActiveSchema(txCtx, phaseID, now, lateInviteToken)
		return err
	})
	return value0, err
}
