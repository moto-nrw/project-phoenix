// Package compose builds the onboarding wizard for new schools (#2832, ADR
// 0035): Settings Platform's state store, the school-setup-view projection
// behind the service's port, the presence-mode writer and the HTTP routes.
package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/schoolsetupview"
	setupHTTP "github.com/moto-nrw/project-phoenix/modules/settings/inbound/setup"
	"github.com/moto-nrw/project-phoenix/realtime"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Middleware is the HTTP middleware shape of the composition root.
type Middleware = func(http.Handler) http.Handler

// Dependencies are the retained services and delivery mechanics the wizard
// binds. Every field is required.
type Dependencies struct {
	Settings configSvc.SettingsService
	// OpenAttendance feeds the open-attendance guard of the presence mode
	// switch.
	OpenAttendance func(context.Context, timezone.Date) (bool, error)
	Broadcaster    realtime.Broadcaster
	// SideEffect is the settings side-effect hook, so a presence mode set in
	// the wizard triggers exactly what an operator's write triggers.
	SideEffect configSvc.OperatorValueSetHook

	Protected    func(chi.Router, func(r chi.Router, withTx Middleware))
	RequireWrite Middleware
	Actor        func(context.Context) (tenantID, accountID int64)
	Respond      func(w http.ResponseWriter, r *http.Request, status int, data any, message string)
	Failure      func(w http.ResponseWriter, r *http.Request, status int, err error)
}

// New returns the router mounted at /api/school-setup.
func New(deps Dependencies) (chi.Router, error) {
	if deps.Settings == nil || deps.OpenAttendance == nil || deps.Broadcaster == nil || deps.SideEffect == nil ||
		deps.Protected == nil || deps.RequireWrite == nil || deps.Actor == nil || deps.Respond == nil || deps.Failure == nil {
		return nil, errors.New("school setup compose: all dependencies are required")
	}

	store := configRepo.NewSchoolSetupRepository(ambientRuntime{})
	projection := schoolsetupview.New(func(ctx context.Context) (bun.IDB, error) {
		return ambientTx(ctx)
	})
	operatorSettings := configSvc.NewOperatorSettingsService(
		deps.Settings,
		ambientSettingsRuntime{},
		settingsChangedNotifier(deps.Broadcaster),
		openAttendance(deps.OpenAttendance),
		slog.Default(),
	)
	presence := func(ctx context.Context, tenantID, accountID int64, mode string) error {
		return operatorSettings.SetValue(ctx, tenantID, configModel.KeyPresenceMode, mode, accountID, false, deps.SideEffect)
	}
	service, err := configSvc.NewSchoolSetupService(store, progress{projection: projection}, deps.Settings, presence, time.Now)
	if err != nil {
		return nil, err
	}
	resource := setupHTTP.NewResource(service, setupHTTP.Runtime{
		Protected:    deps.Protected,
		RequireWrite: deps.RequireWrite,
		Actor:        deps.Actor,
		Respond:      deps.Respond,
		Failure:      deps.Failure,
	})
	return resource.Router(), nil
}

// ConflictCode gives the client a stable code for each 409 of the wizard.
func ConflictCode(err error) string {
	switch {
	case errors.Is(err, configSvc.ErrSchoolSetupCompleted):
		return "school_setup_completed"
	case errors.Is(err, configSvc.ErrSchoolSetupIncomplete):
		return "school_setup_incomplete"
	case errors.Is(err, configSvc.ErrPresenceModeSwitchBlocked):
		return "presence_mode_switch_blocked"
	default:
		return "conflict"
	}
}

// ambientTx resolves the request's tenant transaction. The wizard never runs
// outside one: the store relies on RLS and the projection's tenant_safe
// invariant requires it (ADR 0035).
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

// ambientRuntime hands the settings repository the request's tenant
// transaction. Without one it panics: a repository call outside the tenant
// transaction is a composition bug, not a request error.
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

func settingsChangedNotifier(broadcaster realtime.Broadcaster) configSvc.SettingsChangedNotifier {
	return func(_ context.Context, tenantID int64, key string) {
		event := realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{Source: &key})
		_ = broadcaster.BroadcastToTenant(tenantID, event)
	}
}

type openAttendance func(context.Context, timezone.Date) (bool, error)

func (fn openAttendance) HasOpenAttendanceOn(ctx context.Context, day configModel.CalendarDate) (bool, error) {
	value := day.UTCMidnight()
	return fn(ctx, timezone.NewDate(value.Year(), value.Month(), value.Day()))
}

// progress adapts the projection to the service's port.
type progress struct{ projection *schoolsetupview.Projection }

func (p progress) SchoolSetupFacts(ctx context.Context, tenantID int64) (configSvc.SchoolSetupFacts, error) {
	facts, err := p.projection.Progress(ctx, tenantID)
	if err != nil {
		return configSvc.SchoolSetupFacts{}, err
	}
	return configSvc.SchoolSetupFacts{
		StaffInvited:    facts.StaffInvited,
		RoomCreated:     facts.RoomCreated,
		GroupCreated:    facts.GroupCreated,
		StudentEnrolled: facts.StudentEnrolled,
		GuardianInvited: facts.GuardianInvited,
	}, nil
}
