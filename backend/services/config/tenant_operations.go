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

// TenantActor is the authenticated tenant identity needed by setting writes.
// Transport-specific JWT claims stay outside the settings boundary.
type TenantActor struct {
	TenantID    int64
	AccountID   int64
	Permissions []string
}

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
}

func NewTenantOperations(settings SettingsService, payroll PayrollStatusGetter, runtime TenantOperationsRuntime, hook TenantValueSetHook, notify SettingsChangedNotifier) *TenantOperations {
	return &TenantOperations{settings: settings, payroll: payroll, runtime: runtime, hook: hook, notify: notify}
}

func (o *TenantOperations) SetValueSetHook(hook TenantValueSetHook) { o.hook = hook }

func (o *TenantOperations) Schema(ctx context.Context, permissions []string) (any, error) {
	return o.settings.GetSchema(ctx, permissions)
}

func (o *TenantOperations) Reveal(ctx context.Context, key string, permissions []string) (any, error) {
	def, err := tenantDefinition(key)
	if err != nil {
		return nil, err
	}
	if err := checkWritePermission(def, permissions); err != nil {
		return nil, &SettingsError{Op: "reveal_value", Err: err}
	}
	return o.settings.Resolve(ctx, key)
}

func (o *TenantOperations) SetValue(ctx context.Context, actor TenantActor, key string, value any) error {
	if o == nil || o.settings == nil || o.runtime == nil {
		return ErrRuntimeUnavailable
	}
	if _, err := tenantWritableDefinition(key); err != nil {
		return err
	}
	return o.runtime.WithinTenant(ctx, actor.TenantID, func(txCtx context.Context) error {
		changedBy := actor.AccountID
		if err := o.settings.SetValue(txCtx, key, value, &changedBy, actor.Permissions); err != nil {
			return err
		}
		if err := o.dispatchHook(txCtx, actor.TenantID, key, value); err != nil {
			return err
		}
		o.scheduleNotification(txCtx, actor.TenantID, key)
		return nil
	})
}

func (o *TenantOperations) ResetValue(ctx context.Context, actor TenantActor, key string) error {
	if o == nil || o.settings == nil || o.runtime == nil {
		return ErrRuntimeUnavailable
	}
	def, err := tenantWritableDefinition(key)
	if err != nil {
		return err
	}
	return o.runtime.WithinTenant(ctx, actor.TenantID, func(txCtx context.Context) error {
		changedBy := actor.AccountID
		if err := o.settings.ResetValue(txCtx, key, &changedBy, actor.Permissions); err != nil {
			return err
		}
		if resetReplaysHook(key) {
			if err := o.dispatchHook(txCtx, actor.TenantID, key, def.Default); err != nil {
				return err
			}
		}
		o.scheduleNotification(txCtx, actor.TenantID, key)
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

func (o *TenantOperations) SetLegalDocument(ctx context.Context, actor TenantActor, documentURL string, cleanup LegalDocumentCleanup) error {
	return o.withLegalDocumentWrite(ctx, actor, documentURL, cleanup, false)
}

func (o *TenantOperations) DeleteLegalDocument(ctx context.Context, actor TenantActor, cleanup LegalDocumentCleanup) error {
	return o.withLegalDocumentWrite(ctx, actor, "", cleanup, true)
}

func (o *TenantOperations) withLegalDocumentWrite(ctx context.Context, actor TenantActor, documentURL string, cleanup LegalDocumentCleanup, deleting bool) error {
	if o == nil || o.settings == nil || o.runtime == nil {
		return ErrRuntimeUnavailable
	}
	return o.runtime.WithinTenant(ctx, actor.TenantID, func(txCtx context.Context) error {
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
		changedBy := actor.AccountID
		if deleting {
			err = o.settings.ResetValue(txCtx, configModel.KeyEnrollmentLegalAGBDocumentURL, &changedBy, actor.Permissions)
		} else {
			err = o.settings.SetValue(txCtx, configModel.KeyEnrollmentLegalAGBDocumentURL, documentURL, &changedBy, actor.Permissions)
		}
		if err != nil {
			return err
		}
		if cleanup != nil {
			postCommit, cleanupErr := cleanup(txCtx, actor.TenantID, oldURL, documentURL)
			if cleanupErr != nil {
				return cleanupErr
			}
			o.runtime.AfterCommit(txCtx, postCommit)
		}
		o.scheduleNotification(txCtx, actor.TenantID, configModel.KeyEnrollmentLegalAGBDocumentURL)
		return nil
	})
}

func tenantDefinition(key string) (*configModel.Definition, error) {
	def := configModel.GetDefinition(key)
	if def == nil {
		return nil, &SettingsError{Op: "tenant_access", Err: &DefinitionNotFoundError{Key: key}}
	}
	if def.AccessPolicy == configModel.AccessOperatorOnly {
		return nil, ErrTenantOperatorOnly
	}
	return def, nil
}

func tenantWritableDefinition(key string) (*configModel.Definition, error) {
	def, err := tenantDefinition(key)
	if err != nil {
		return nil, err
	}
	if key == configModel.KeyEnrollmentLegalAGBDocumentURL {
		return nil, ErrDirectManagedSetting
	}
	return def, nil
}
