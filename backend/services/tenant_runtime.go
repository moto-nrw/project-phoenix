package services

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func BindTenantRuntime(
	withinTenant func(context.Context, int64, func(context.Context, any) error) error,
	withinAdmin func(context.Context, func(context.Context, any) error) error,
	controlSavepoint func(context.Context, uint8) error,
) (tenant.Runtime, error) {
	return tenant.NewRuntime(withinTenant, withinAdmin, controlSavepoint)
}

type tenantRuntimeSetter interface {
	SetTenantRuntime(tenant.Runtime)
}

func (f *Factory) SetTenantRuntime(runtime tenant.Runtime) error {
	bindings := []struct {
		name   string
		target any
	}{
		{"auth", f.Auth},
		{"settings", f.Settings},
		{"invitation", f.Invitation},
		{"guardian invitation", f.GuardianInvitation},
		{"operator auth", f.OperatorAuth},
		{"operator invitation", f.OperatorInvitation},
		{"operator provisioning", f.OperatorProvisioning},
		{"operator MFA", f.OperatorMFA},
		{"notifications", f.Notifications},
		{"push subscriptions", f.PushSubscriptions},
		{"notification preferences", f.NotificationPreferences},
		{"absence notifier", f.AbsenceNotifier},
		{"PWA usage", f.PWAUsage},
		{"MFA", f.MFA},
	}
	for _, binding := range bindings {
		if binding.target == nil {
			continue
		}
		setter, ok := binding.target.(tenantRuntimeSetter)
		if !ok {
			return fmt.Errorf("configure tenant runtime for %s: setter is missing", binding.name)
		}
		setter.SetTenantRuntime(runtime)
	}
	if f.EmailOutboxWorker != nil {
		f.EmailOutboxWorker.SetTenantRuntime(runtime)
	}
	if f.ParentEventEmitter != nil {
		f.ParentEventEmitter.SetTenantRuntime(runtime)
	}
	return nil
}
