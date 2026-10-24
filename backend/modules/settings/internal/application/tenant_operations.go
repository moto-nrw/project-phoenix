package config

import (
	"context"
	"errors"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

var (
	ErrTenantOperatorOnly = errors.New("this setting is operator-only")
	// ErrTenantOperatorManaged rejects a school's write of a setting the moto
	// team records for it (ADR 0048); the school reads it only.
	ErrTenantOperatorManaged = errors.New("this setting is managed by the moto team")
	ErrDirectManagedSetting  = errors.New("setting is managed by a dedicated endpoint")
	ErrActiveLegalAGBPDF     = errors.New("AGB-PDF kann nicht entfernt werden, solange die AGB aktiv sind und als PDF angezeigt werden")
)

// TenantOperations is the application seam used by the tenant settings HTTP
// adapter. It owns policy checks, transactions, write side effects, and change
// notification; the adapter only translates HTTP.
type TenantOperations struct {
	configModel.LoginImages
	settings configModel.SettingsService
	payroll  configModel.PayrollStatusGetter
	runtime  configModel.TenantOperationsRuntime
	hook     configModel.TenantValueSetHook
	notify   configModel.SettingsChangedNotifier
}

func NewTenantOperations(settings configModel.SettingsService, payroll configModel.PayrollStatusGetter, runtime configModel.TenantOperationsRuntime, hook configModel.TenantValueSetHook, notify configModel.SettingsChangedNotifier) *TenantOperations {
	return &TenantOperations{LoginImages: settings, settings: settings, payroll: payroll, runtime: runtime, hook: hook, notify: notify}
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
		return nil, &configModel.SettingsError{Op: "reveal_value", Err: err}
	}
	return o.settings.Resolve(ctx, key)
}

func (o *TenantOperations) SetValue(ctx context.Context, tenantID, accountID int64, permissions []string, key string, value any) error {
	if o == nil || o.settings == nil || o.runtime == nil {
		return configModel.ErrRuntimeUnavailable
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
		return configModel.ErrRuntimeUnavailable
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
		return nil, configModel.ErrRuntimeUnavailable
	}
	return o.payroll.GetPayrollStatus(ctx)
}

func (o *TenantOperations) SetLegalDocument(ctx context.Context, tenantID, accountID int64, permissions []string, documentURL string, cleanup func(context.Context, int64, string, string) (func(), error)) error {
	return o.withLegalDocumentWrite(ctx, tenantID, accountID, permissions, documentURL, cleanup, false)
}

func (o *TenantOperations) DeleteLegalDocument(ctx context.Context, tenantID, accountID int64, permissions []string, cleanup func(context.Context, int64, string, string) (func(), error)) error {
	return o.withLegalDocumentWrite(ctx, tenantID, accountID, permissions, "", cleanup, true)
}

func (o *TenantOperations) withLegalDocumentWrite(ctx context.Context, tenantID, accountID int64, permissions []string, documentURL string, cleanup configModel.LegalDocumentCleanup, deleting bool) error {
	if o == nil || o.settings == nil || o.runtime == nil {
		return configModel.ErrRuntimeUnavailable
	}
	return o.runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
		if deleting {
			if err := o.checkLegalDocumentDeletion(txCtx); err != nil {
				return err
			}
		}
		oldURL, err := o.settings.ResolveString(txCtx, configModel.KeyEnrollmentLegalAGBDocumentURL)
		if err != nil {
			return err
		}
		err = o.writeLegalDocumentURL(txCtx, accountID, permissions, documentURL, deleting)
		if err != nil {
			return err
		}
		if err := o.prepareLegalDocumentCleanup(txCtx, tenantID, oldURL, documentURL, cleanup); err != nil {
			return err
		}
		o.scheduleNotification(txCtx, tenantID, configModel.KeyEnrollmentLegalAGBDocumentURL)
		return nil
	})
}

func (o *TenantOperations) writeLegalDocumentURL(ctx context.Context, accountID int64, permissions []string, documentURL string, deleting bool) error {
	if deleting {
		return o.settings.ResetValue(ctx, configModel.KeyEnrollmentLegalAGBDocumentURL, &accountID, permissions)
	}
	return o.settings.SetValue(ctx, configModel.KeyEnrollmentLegalAGBDocumentURL, documentURL, &accountID, permissions)
}

func (*TenantOperations) ClassifyError(err error) string {
	if errors.Is(err, ErrTenantOperatorOnly) || errors.Is(err, ErrTenantOperatorManaged) || errors.Is(err, ErrDirectManagedSetting) || errors.Is(err, configModel.ErrPermissionDenied) {
		return "forbidden"
	}
	if errors.Is(err, configModel.ErrDefinitionNotFound) {
		return "not_found"
	}
	if errors.Is(err, configModel.ErrInvalidValue) || errors.Is(err, ErrActiveLegalAGBPDF) {
		return "invalid"
	}
	return "internal"
}

func tenantDefinition(settings configModel.SettingsService, key string) (*configModel.Definition, error) {
	def := definitionFor(settings, key)
	if def == nil {
		return nil, &configModel.SettingsError{Op: "tenant_access", Err: &configModel.DefinitionNotFoundError{Key: key}}
	}
	if def.AccessPolicy == configModel.AccessOperatorOnly {
		return nil, ErrTenantOperatorOnly
	}
	return def, nil
}

func tenantWritableDefinition(settings configModel.SettingsService, key string) (*configModel.Definition, error) {
	def, err := tenantDefinition(settings, key)
	if err != nil {
		return nil, err
	}
	if key == configModel.KeyEnrollmentLegalAGBDocumentURL {
		return nil, ErrDirectManagedSetting
	}
	if def.AccessPolicy == configModel.AccessOperatorManaged {
		return nil, ErrTenantOperatorManaged
	}
	return def, nil
}

func (o *TenantOperations) checkLegalDocumentDeletion(ctx context.Context) error {
	termsEnabled, err := o.settings.ResolveBool(ctx, configModel.KeyEnrollmentLegalTermsEnabled)
	if err != nil {
		return err
	}
	displayMode, err := o.settings.ResolveString(ctx, configModel.KeyEnrollmentLegalAGBDisplayMode)
	if err != nil {
		return err
	}
	if termsEnabled && displayMode == configModel.EnrollmentLegalAGBDisplayModePDF {
		return ErrActiveLegalAGBPDF
	}
	return nil
}

func (o *TenantOperations) prepareLegalDocumentCleanup(ctx context.Context, tenantID int64, oldURL, documentURL string, cleanup configModel.LegalDocumentCleanup) error {
	if cleanup != nil {
		postCommit, cleanupErr := cleanup(ctx, tenantID, oldURL, documentURL)
		if cleanupErr != nil {
			return cleanupErr
		}
		o.runtime.AfterCommit(ctx, postCommit)
	}
	return nil
}
