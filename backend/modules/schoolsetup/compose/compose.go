// Package compose builds the onboarding wizard for new schools (#2832, ADR
// 0043): the wizard's own store, the school-setup progress projection behind the
// progress port, and the settings the steps depend on.
package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/schoolsetup"
	"github.com/moto-nrw/project-phoenix/modules/schoolsetup/internal/application"

	"github.com/moto-nrw/project-phoenix/modules/schoolsetupview"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Dependencies are the retained Settings Platform seams the wizard binds.
type Dependencies struct {
	// Settings resolves the school's settings that decide which steps apply.
	Settings configSvc.SettingsService
}

// New returns the wizard service.
func New(deps Dependencies) (schoolsetup.Service, error) {
	if deps.Settings == nil {
		return nil, errors.New("school setup compose: settings are required")
	}
	store := configRepo.NewSchoolSetupRepository(ambientRuntime{})
	projection := schoolsetupview.New(ambientTx)
	return application.New(store, progress{projection: projection}, settings{deps.Settings}, time.Now)
}

// NewStaffOnboarding returns the first steps of care workers (#3748): the
// person's own progress and the same progress projection, which says whether
// the school has a group or a child yet.
func NewStaffOnboarding() (schoolsetup.StaffOnboardingService, error) {
	store := configRepo.NewStaffOnboardingRepository(ambientRuntime{})
	projection := schoolsetupview.New(ambientTx)
	return application.NewStaffOnboarding(store, progress{projection: projection}, time.Now)
}

// ambientTx resolves the request's tenant transaction. The wizard never runs
// outside one: the store relies on RLS and the projection's tenant_safe
// invariant requires it (ADR 0043).
func ambientTx(ctx context.Context) (bun.IDB, error) {
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("school setup: tenant transaction is required")
	}
	switch tx := transaction.(type) {
	case bun.Tx:
		return tx, nil
	case *bun.Tx:
		if tx != nil {
			return tx, nil
		}
	}
	return nil, fmt.Errorf("school setup: unsupported transaction %T", transaction)
}

// ambientRuntime hands the store the request's tenant transaction. Without
// one it panics: a store call outside the tenant transaction is a
// composition bug, not a request error.
type ambientRuntime struct{}

func (ambientRuntime) DB(ctx context.Context) bun.IDB {
	db, err := ambientTx(ctx)
	if err != nil {
		panic(err)
	}
	return db
}

// settings resolves the two settings the steps depend on in the request's
// tenant.
type settings struct{ service configSvc.SettingsService }

func (s settings) PresenceMode(ctx context.Context) (string, error) {
	return s.service.ResolveString(ctx, configModel.KeyPresenceMode)
}

func (s settings) GroupMode(ctx context.Context) (string, error) {
	return s.service.ResolveString(ctx, configModel.KeyGroupMode)
}

// progress adapts the projection to the service's port.
type progress struct{ projection *schoolsetupview.Projection }

func (p progress) Facts(ctx context.Context, tenantID int64) (schoolsetup.Facts, error) {
	facts, err := p.projection.Progress(ctx, tenantID)
	if err != nil {
		return schoolsetup.Facts{}, err
	}
	return schoolsetup.Facts{
		StaffInvited:    facts.StaffInvited,
		RoomCreated:     facts.RoomCreated,
		GroupCreated:    facts.GroupCreated,
		StudentEnrolled: facts.StudentEnrolled,
		GuardianInvited: facts.GuardianInvited,
	}, nil
}
