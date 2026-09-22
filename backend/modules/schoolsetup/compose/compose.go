// Package compose builds the onboarding wizard for new schools (#2832, ADR
// 0035): the wizard's own store, the school-setup-view projection behind the
// progress port, and the presence-mode write through Settings Platform's
// operator orchestration in the request's tenant transaction.
package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
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
	// Settings resolves the school's settings and writes the presence mode.
	Settings configSvc.SettingsService
	// OpenAttendance feeds the open-attendance guard of the presence mode
	// switch.
	OpenAttendance configSvc.OpenAttendanceChecker
	// Notify tells open tenant tabs that a setting changed. Nil disables the
	// broadcast.
	Notify configSvc.SettingsChangedNotifier
	// SideEffect is the settings side-effect hook, so a presence mode set in
	// the wizard triggers exactly what an operator's write triggers.
	SideEffect configSvc.OperatorValueSetHook
}

// New returns the wizard service.
func New(deps Dependencies) (schoolsetup.Service, error) {
	if deps.Settings == nil || deps.OpenAttendance == nil || deps.SideEffect == nil {
		return nil, errors.New("school setup compose: settings, open attendance and side effect are required")
	}
	store := configRepo.NewSchoolSetupRepository(ambientRuntime{})
	projection := schoolsetupview.New(ambientTx)
	operatorSettings := configSvc.NewOperatorSettingsService(
		deps.Settings,
		ambientSettingsRuntime{},
		deps.Notify,
		deps.OpenAttendance,
		slog.Default(),
	)
	presence := func(ctx context.Context, tenantID, accountID int64, mode string) error {
		err := operatorSettings.SetValue(ctx, tenantID, configModel.KeyPresenceMode, mode, accountID, false, deps.SideEffect)
		if errors.Is(err, configSvc.ErrPresenceModeSwitchBlocked) {
			return schoolsetup.ErrPresenceModeBlocked
		}
		return err
	}
	return application.New(store, progress{projection: projection}, settings{deps.Settings}, presence, time.Now)
}

// ambientTx resolves the request's tenant transaction. The wizard never runs
// outside one: the store relies on RLS and the projection's tenant_safe
// invariant requires it (ADR 0040).
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

// ambientSettingsRuntime runs the operator orchestration inside the request's
// tenant transaction instead of opening a second one, so the presence mode and
// the wizard answers commit together.
type ambientSettingsRuntime struct{}

func (ambientSettingsRuntime) WithinTenant(ctx context.Context, _ int64, fn func(context.Context) error) error {
	if _, err := ambientTx(ctx); err != nil {
		return err
	}
	return fn(ctx)
}

func (ambientSettingsRuntime) AfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}

func (ambientSettingsRuntime) Today() configModel.CalendarDate {
	today := timezone.TodayDate()
	return configModel.NewCalendarDate(today.Year(), today.Month(), today.Day())
}

// settings resolves the three settings the steps depend on in the request's
// tenant.
type settings struct{ service configSvc.SettingsService }

func (s settings) PresenceMode(ctx context.Context) (string, error) {
	return s.service.ResolveString(ctx, configModel.KeyPresenceMode)
}

func (s settings) GroupMode(ctx context.Context) (string, error) {
	return s.service.ResolveString(ctx, configModel.KeyGroupMode)
}

func (s settings) TimetableEnabled(ctx context.Context) (bool, error) {
	return s.service.ResolveBool(ctx, configModel.KeyTimetableEnabled)
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
