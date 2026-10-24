// Package compose builds the onboarding wizard for new schools (#2832, ADR
// 0043): the wizard's own store, the school-setup progress projection behind the
// progress port, and the settings the steps depend on.
package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/moto-nrw/project-phoenix/modules/schoolsetup"
	"github.com/moto-nrw/project-phoenix/modules/schoolsetup/internal/adapters/schoolsetupview"
	"github.com/moto-nrw/project-phoenix/modules/schoolsetup/internal/application"
	"github.com/uptrace/bun"
)

// Dependencies are the Settings Platform seams the wizard binds.
type Dependencies struct {
	// Settings resolves the school's settings that decide which steps apply;
	// the root binds it to the Settings Platform.
	Settings schoolsetup.Settings
	Store    schoolsetup.Store
}

// New returns the wizard service.
func New(deps Dependencies) (schoolsetup.Service, error) {
	if deps.Settings == nil {
		return nil, errors.New("school setup compose: settings are required")
	}
	store := deps.Store
	projection := schoolsetupview.New(ambientTx)
	return application.New(store, application.NewProgress(projection), deps.Settings, time.Now)
}

// NewStaffOnboarding returns the first steps of care workers (#3748): the
// person's own progress and the same progress projection, which says whether
// the school has a group or a child yet.
func NewStaffOnboarding(store schoolsetup.StaffStore) (schoolsetup.StaffOnboardingService, error) {
	projection := schoolsetupview.New(ambientTx)
	return application.NewStaffOnboarding(store, application.NewProgress(projection), time.Now)
}

// ambientTx resolves the request's tenant transaction. The wizard never runs
// outside one: the store relies on RLS and the projection's tenant_safe
// invariant requires it (ADR 0043).
func ambientTx(ctx context.Context) (bun.IDB, error) {
	return schoolsetupview.AmbientTx(tenant.TransactionFromContext(ctx))
}

// AmbientRuntime hands the store the request's tenant transaction. Without
// one it returns a missing-transaction error.
type AmbientRuntime struct{}

func (AmbientRuntime) DB(ctx context.Context) (bun.IDB, error) { return ambientTx(ctx) }
