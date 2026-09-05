package config

import (
	"context"
	"errors"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

var (
	ErrTenantOperatorOnly   = errors.New("this setting is operator-only")
	ErrDirectManagedSetting = errors.New("setting is managed by a dedicated endpoint")
	ErrActiveLegalAGBPDF    = errors.New("AGB-PDF kann nicht entfernt werden, solange die AGB aktiv sind und als PDF angezeigt werden")
)

// TenantValueSetHook runs domain-owned side effects in the write transaction.
type TenantValueSetHook func(context.Context, int64, string, any) (func(), error)

// LegalDocumentCleanup prepares post-commit cleanup for a replaced document.
type LegalDocumentCleanup func(context.Context, int64, string, string) (func(), error)

type TenantOperationsRuntime interface {
	WithinTenant(context.Context, int64, func(context.Context) error) error
	AfterCommit(context.Context, func())
}

// TenantOperations is the application seam used by the tenant settings HTTP
// adapter. It owns policy checks, transactions, write side effects, and change
// notification; the adapter only translates HTTP.
type TenantOperations struct {
	settings SettingsService
	payroll  PayrollStatusGetter
	runtime  TenantOperationsRuntime
	hook     TenantValueSetHook
	notify   SettingsChangedNotifier
	// homeLayouts serves the personal start page composition (#2875). Optional:
	// nil makes those endpoints answer "not configured" instead of panicking.
	homeLayouts *HomeLayoutService
}

func NewTenantOperations(settings SettingsService, payroll PayrollStatusGetter, runtime TenantOperationsRuntime, homeLayouts *HomeLayoutService, hook TenantValueSetHook, notify SettingsChangedNotifier) *TenantOperations {
	return &TenantOperations{settings: settings, payroll: payroll, runtime: runtime, homeLayouts: homeLayouts, hook: hook, notify: notify}
}

func (o *TenantOperations) SetValueSetHook(hook func(context.Context, int64, string, any) (func(), error)) {
	o.hook = hook
}

func (o *TenantOperations) Schema(ctx context.Context, permissions []string) (any, error) {
	return o.settings.GetSchema(ctx, permissions)
}

func (o *TenantOperations) Reveal(ctx context.Context, key string, permissions []string) (any, error) {
	def, err := tenantDefinition(o.settings, key)
	if err != nil {
		return nil, err
	}
	if err := checkWritePermission(def, permissions); err != nil {
		return nil, &SettingsError{Op: "reveal_value", Err: err}
	}
	return o.settings.Resolve(ctx, key)
}

func (o *TenantOperations) SetValue(ctx context.Context, tenantID, accountID int64, permissions []string, key string, value any) error {
	if o == nil || o.settings == nil || o.runtime == nil {
		return ErrRuntimeUnavailable
	}
	if _, err := tenantWritableDefinition(o.settings, key); err != nil {
		return err
	}
	return o.runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
		changedBy := accountID
		if err := o.settings.SetValue(txCtx, key, value, &changedBy, permissions); err != nil {
			return err
		}
		if err := o.dispatchHook(txCtx, tenantID, key, value); err != nil {
			return err
		}
		o.scheduleNotification(txCtx, tenantID, key)
		return nil
	})
}

func (o *TenantOperations) ResetValue(ctx context.Context, tenantID, accountID int64, permissions []string, key string) error {
	if o == nil || o.settings == nil || o.runtime == nil {
		return ErrRuntimeUnavailable
	}
	def, err := tenantWritableDefinition(o.settings, key)
	if err != nil {
		return err
	}
	return o.runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
		changedBy := accountID
		if err := o.settings.ResetValue(txCtx, key, &changedBy, permissions); err != nil {
			return err
		}
		if resetReplaysHook(key) {
			if err := o.dispatchHook(txCtx, tenantID, key, def.Default); err != nil {
				return err
			}
		}
		o.scheduleNotification(txCtx, tenantID, key)
		return nil
	})
}

func (o *TenantOperations) dispatchHook(ctx context.Context, tenantID int64, key string, value any) error {
	if o.hook == nil {
		return nil
	}
	postCommit, err := o.hook(ctx, tenantID, key, value)
	if err != nil {
		return err
	}
	o.runtime.AfterCommit(ctx, postCommit)
	return nil
}

func (o *TenantOperations) scheduleNotification(ctx context.Context, tenantID int64, key string) {
	if o.notify == nil || tenantID <= 0 {
		return
	}
	o.runtime.AfterCommit(ctx, func() { o.notify(ctx, tenantID, key) })
}

func (o *TenantOperations) PayrollStatus(ctx context.Context) (any, error) {
	if o == nil || o.payroll == nil {
		return nil, ErrRuntimeUnavailable
	}
	return o.payroll.GetPayrollStatus(ctx)
}

func (o *TenantOperations) LoginImageURL(ctx context.Context, tenantID int64) (string, error) {
	return o.settings.GetLoginImageURL(ctx, tenantID)
}

func (o *TenantOperations) SetLoginImageURL(ctx context.Context, tenantID int64, imageURL string) (string, error) {
	return o.settings.SetLoginImageURL(ctx, tenantID, imageURL)
}

func (o *TenantOperations) ClearLoginImageURL(ctx context.Context, tenantID int64) (string, error) {
	return o.settings.ClearLoginImageURL(ctx, tenantID)
}

func (o *TenantOperations) SetLegalDocument(ctx context.Context, tenantID, accountID int64, permissions []string, documentURL string, cleanup func(context.Context, int64, string, string) (func(), error)) error {
	return o.withLegalDocumentWrite(ctx, tenantID, accountID, permissions, documentURL, cleanup, false)
}

func (o *TenantOperations) DeleteLegalDocument(ctx context.Context, tenantID, accountID int64, permissions []string, cleanup func(context.Context, int64, string, string) (func(), error)) error {
	return o.withLegalDocumentWrite(ctx, tenantID, accountID, permissions, "", cleanup, true)
}

func (o *TenantOperations) withLegalDocumentWrite(ctx context.Context, tenantID, accountID int64, permissions []string, documentURL string, cleanup LegalDocumentCleanup, deleting bool) error {
	if o == nil || o.settings == nil || o.runtime == nil {
		return ErrRuntimeUnavailable
	}
	return o.runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
		if deleting {
			termsEnabled, err := o.settings.ResolveBool(txCtx, configModel.KeyEnrollmentLegalTermsEnabled)
			if err != nil {
				return err
			}
			displayMode, err := o.settings.ResolveString(txCtx, configModel.KeyEnrollmentLegalAGBDisplayMode)
			if err != nil {
				return err
			}
			if termsEnabled && displayMode == configModel.EnrollmentLegalAGBDisplayModePDF {
				return ErrActiveLegalAGBPDF
			}
		}
		oldURL, err := o.settings.ResolveString(txCtx, configModel.KeyEnrollmentLegalAGBDocumentURL)
		if err != nil {
			return err
		}
		changedBy := accountID
		if deleting {
			err = o.settings.ResetValue(txCtx, configModel.KeyEnrollmentLegalAGBDocumentURL, &changedBy, permissions)
		} else {
			err = o.settings.SetValue(txCtx, configModel.KeyEnrollmentLegalAGBDocumentURL, documentURL, &changedBy, permissions)
		}
		if err != nil {
			return err
		}
		if cleanup != nil {
			postCommit, cleanupErr := cleanup(txCtx, tenantID, oldURL, documentURL)
			if cleanupErr != nil {
				return cleanupErr
			}
			o.runtime.AfterCommit(txCtx, postCommit)
		}
		o.scheduleNotification(txCtx, tenantID, configModel.KeyEnrollmentLegalAGBDocumentURL)
		return nil
	})
}

func (*TenantOperations) ClassifyError(err error) string {
	if errors.Is(err, ErrTenantOperatorOnly) || errors.Is(err, ErrDirectManagedSetting) || errors.Is(err, ErrPermissionDenied) {
		return "forbidden"
	}
	if errors.Is(err, ErrDefinitionNotFound) {
		return "not_found"
	}
	if errors.Is(err, ErrInvalidValue) || errors.Is(err, ErrActiveLegalAGBPDF) {
		return "invalid"
	}
	return "internal"
}

func tenantDefinition(settings SettingsService, key string) (*configModel.Definition, error) {
	def := definitionFor(settings, key)
	if def == nil {
		return nil, &SettingsError{Op: "tenant_access", Err: &DefinitionNotFoundError{Key: key}}
	}
	if def.AccessPolicy == configModel.AccessOperatorOnly {
		return nil, ErrTenantOperatorOnly
	}
	return def, nil
}

func tenantWritableDefinition(settings SettingsService, key string) (*configModel.Definition, error) {
	def, err := tenantDefinition(settings, key)
	if err != nil {
		return nil, err
	}
	if key == configModel.KeyEnrollmentLegalAGBDocumentURL {
		return nil, ErrDirectManagedSetting
	}
	return def, nil
}

// --- Start page composition (#2875) ---

func (o *TenantOperations) homeLayoutService() (*HomeLayoutService, error) {
	if o == nil || o.homeLayouts == nil {
		return nil, ErrHomeLayoutUnavailable
	}
	return o.homeLayouts, nil
}

// HomeLayout returns one person's start page composition plus the school's
// prescription. Readable by every signed-in account: the start page renders
// for everybody.
func (o *TenantOperations) HomeLayout(ctx context.Context, tenantID, accountID int64, permissions []string) (any, error) {
	service, err := o.homeLayoutService()
	if err != nil {
		return nil, err
	}
	return service.View(ctx, tenantID, accountID, permissions)
}

// SetHomeLayout replaces the caller's own composition. The account comes from
// the token, so there is no way to write somebody else's start page.
func (o *TenantOperations) SetHomeLayout(ctx context.Context, tenantID, accountID int64, overrides map[string]bool) error {
	service, err := o.homeLayoutService()
	if err != nil {
		return err
	}
	return service.SetOverrides(ctx, tenantID, accountID, overrides)
}

// ResetHomeLayout restores the composition recommended for the caller's role.
func (o *TenantOperations) ResetHomeLayout(ctx context.Context, tenantID, accountID int64) error {
	service, err := o.homeLayoutService()
	if err != nil {
		return err
	}
	return service.ResetOverrides(ctx, tenantID, accountID)
}

// SetHomeBlockPolicies replaces what the school prescribes. The permission is
// checked in the application layer, not by the route: the same path is read by
// everybody and written only by config:update.
func (o *TenantOperations) SetHomeBlockPolicies(ctx context.Context, tenantID, accountID int64, permissions []string, policies map[string]string) error {
	service, err := o.homeLayoutService()
	if err != nil {
		return err
	}

	parsed := make(map[string]configModel.BlockPolicy, len(policies))
	for key, policy := range policies {
		parsed[key] = configModel.BlockPolicy(policy)
	}
	return service.SetPolicies(ctx, tenantID, accountID, permissions, parsed)
}
